package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/engine/duckdb"
)

func TestMongoURIPrefersStoredConnectionString(t *testing.T) {
	cfg := &entity.ConnectionConfig{
		URI:  "mongodb://user:p%40ss@mongo-a:27017,mongo-b:27017/?replicaSet=rs0",
		Host: "ignored",
		Port: 1234,
	}
	got, err := mongoURI(cfg)
	if err != nil {
		t.Fatalf("mongoURI: %v", err)
	}
	// A profile that names no TLS wish must reach the driver exactly as saved.
	if got != cfg.URI {
		t.Errorf("mongoURI() = %q, want the stored URI", got)
	}
}

func TestMongoURIEscapesCredentials(t *testing.T) {
	cfg := &entity.ConnectionConfig{
		Host:     "muara_sample_mongo",
		Port:     27017,
		User:     "mongo user",
		Password: "p@ss:w/ord?",
	}

	uri, err := mongoURI(cfg)
	if err != nil {
		t.Fatalf("mongoURI: %v", err)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("generated URI does not parse: %v", err)
	}
	// Percent-encoding is what keeps a password containing "@" or "/" from
	// truncating the host section of the connection string.
	if pass, _ := parsed.User.Password(); pass != cfg.Password {
		t.Errorf("password round-trip = %q, want %q", pass, cfg.Password)
	}
	if got := parsed.User.Username(); got != cfg.User {
		t.Errorf("username round-trip = %q, want %q", got, cfg.User)
	}
	if parsed.Host != "muara_sample_mongo:27017" {
		t.Errorf("host = %q", parsed.Host)
	}
	if parsed.Query().Get("authSource") != "admin" {
		t.Errorf("authSource missing in %q", parsed.RawQuery)
	}
}

func TestMongoURIEmptyWithoutHost(t *testing.T) {
	// StreamMongoCollections reports a clear "needs a URI" error instead of
	// dialing an empty string and timing out.
	got, err := mongoURI(&entity.ConnectionConfig{Database: "sample_analytics"})
	if err != nil || got != "" {
		t.Errorf("mongoURI() = %q, err = %v, want empty", got, err)
	}
}

// The CA reaches the Go driver as tlsCAFile — and only together with TLS, since
// a measured CA file that is loaded while tls=false verifies nothing.
func TestMongoURIAddsTLSToTheCASavedOnTheProfile(t *testing.T) {
	ca := "/etc/ssl/certs/internal-ca.pem"
	cases := []struct {
		name string
		cfg  *entity.ConnectionConfig
	}{
		{"host fields", &entity.ConnectionConfig{
			Host: "mongo.internal", Port: 27017, User: "reader", Password: "pw",
			SSLMode: "verify-ca", RootCert: ca,
		}},
		{"stored uri", &entity.ConnectionConfig{
			URI:      "mongodb://reader:pw@mongo.internal:27017/admin?replicaSet=rs0",
			SSLMode:  "verify-full",
			RootCert: ca,
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uri, err := mongoURI(c.cfg)
			if err != nil {
				t.Fatalf("mongoURI: %v", err)
			}
			q := mustURL(t, uri).Query()
			if q.Get("tlsCAFile") != ca {
				t.Errorf("tlsCAFile = %q in %q, want %q", q.Get("tlsCAFile"), uri, ca)
			}
			if q.Get("tls") != "true" {
				t.Errorf("tls = %q, want true — a CA without TLS verifies nothing", q.Get("tls"))
			}
			// Whatever the profile already carried must survive the merge.
			if c.cfg.URI != "" && q.Get("replicaSet") != "rs0" {
				t.Errorf("replicaSet lost from the stored uri: %q", uri)
			}
			if c.cfg.Host != "" && q.Get("authSource") != "admin" {
				t.Errorf("authSource lost when TLS is added: %q", uri)
			}
		})
	}
}

// Two sources of truth for one CA is the failure mode this whole change is about:
// the driver reads tlsCAFile and silently ignores sslCAFile.
func TestMongoURIRefusesAConflictingCA(t *testing.T) {
	ca := "/etc/ssl/certs/internal-ca.pem"
	cases := []struct {
		name    string
		uri     string
		wantSub string
	}{
		{"other ca in uri", "mongodb://mongo.internal:27017/?tlsCAFile=/etc/other-ca.pem", "conflicts"},
		{"legacy name ignored", "mongodb://mongo.internal:27017/?sslCAFile=/etc/other-ca.pem", "ignores"},
		{"tls off", "mongodb://mongo.internal:27017/?tls=false", "tls=false"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := mongoURI(&entity.ConnectionConfig{
				URI: c.uri, SSLMode: "verify-ca", RootCert: ca,
			})
			if err == nil {
				t.Fatalf("accepted %q together with root_cert %q", c.uri, ca)
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("error = %q, want it to mention %q", err, c.wantSub)
			}
		})
	}
}

// The UI posts ssl_mode "prefer" for every driver, and the form already did before
// any of this existed. A fallback mode therefore has to keep the source plain
// instead of failing or silently upgrading to TLS — the one thing it may not do is
// sit next to a CA, because then the profile looks verified while nothing checks.
func TestMongoURILeavesFallbackModesPlainButRefusesACA(t *testing.T) {
	ca := "/etc/ssl/certs/internal-ca.pem"
	for _, mode := range []string{"", "disable", "allow", "prefer"} {
		uri, err := mongoURI(&entity.ConnectionConfig{
			Host: "mongo.internal", Port: 27017, SSLMode: mode,
		})
		if err != nil {
			t.Errorf("ssl_mode %q refused: %v", mode, err)
			continue
		}
		if q := mustURL(t, uri).Query(); q.Has("tls") || q.Has("tlsCAFile") {
			t.Errorf("ssl_mode %q produced TLS in %q, so a plain source would stop working", mode, uri)
		}

		if _, err := mongoURI(&entity.ConnectionConfig{
			Host: "mongo.internal", Port: 27017, SSLMode: mode, RootCert: ca,
		}); err == nil {
			t.Errorf("ssl_mode %q + root_cert accepted: the CA would be stored and used for nothing", mode)
		}
	}

	if _, err := mongoURI(&entity.ConnectionConfig{
		Host: "mongo.internal", Port: 27017, SSLMode: "bogus",
	}); err == nil {
		t.Error("an unrecognised ssl_mode was accepted for mongodb")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return parsed
}

func TestPostgresDSNDefaultsSSLModeOnce(t *testing.T) {
	// A connection saved without ssl_mode used to produce
	// "... sslmode= sslmode=disable", which pgx refuses to parse, so every
	// SyncSchema on such a connection failed.
	cfg := &entity.ConnectionConfig{
		Host: "dwh.internal", Port: 5432, User: "reader", Password: "secret", Database: "sales",
	}
	dsn := mustDSN(t, cfg)
	if n := strings.Count(dsn, "sslmode="); n != 1 {
		t.Fatalf("dsn = %q, want exactly one sslmode, got %d", dsn, n)
	}
	// The shape this used to produce, kept so the test still proves the bug it
	// guards against is really fatal to pgx.
	if _, err := pgx.ParseConfig("host=dwh.internal port=5432 user=reader password=secret dbname=sales sslmode= sslmode=disable"); err == nil {
		t.Error("the double-sslmode dsn parses, so this test no longer proves anything")
	}

	parsed, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgx rejects the generated dsn: %v", err)
	}
	// pgx folds sslmode into the TLS config instead of RuntimeParams, so the
	// effect is asserted there.
	if parsed.TLSConfig != nil || len(parsed.Fallbacks) > 0 {
		t.Errorf("sslmode=disable expected no TLS config or fallback, got TLS=%v fallbacks=%d",
			parsed.TLSConfig != nil, len(parsed.Fallbacks))
	}
	if parsed.Host != cfg.Host || parsed.Port != uint16(cfg.Port) || parsed.Database != cfg.Database || parsed.User != cfg.User {
		t.Errorf("dsn lost fields: %+v", parsed.Config)
	}
}

func TestPostgresDSNKeepsConfiguredSSLMode(t *testing.T) {
	parsed, err := pgx.ParseConfig(mustDSN(t, &entity.ConnectionConfig{
		Host: "dwh.internal", Port: 5432, User: "reader", Database: "sales", SSLMode: "verify-full",
	}))
	if err != nil {
		t.Fatalf("pgx rejects the generated dsn: %v", err)
	}
	if parsed.TLSConfig == nil || parsed.TLSConfig.InsecureSkipVerify || parsed.TLSConfig.ServerName != "dwh.internal" {
		t.Errorf("verify-full not in effect: %#v", parsed.TLSConfig)
	}
}

// Credentials are free text, so a space must not split the value into what pgx
// reads as another parameter.
func TestPostgresDSNQuotesValuesWithSpaces(t *testing.T) {
	cfg := &entity.ConnectionConfig{
		Host: "dwh.internal", Port: 5432,
		User:     "read er",
		Password: "p a'ss\\",
		Database: "sales db",
		SSLMode:  "require",
	}

	dsn := mustDSN(t, cfg)
	parsed, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgx rejects the generated dsn: %v (%q)", err, dsn)
	}
	if parsed.User != cfg.User {
		t.Errorf("user = %q, want %q", parsed.User, cfg.User)
	}
	if parsed.Config.Password != cfg.Password {
		t.Errorf("password = %q, want %q", parsed.Config.Password, cfg.Password)
	}
	if parsed.Database != cfg.Database {
		t.Errorf("database = %q, want %q", parsed.Database, cfg.Database)
	}
}

// A credential that looks like another keyword=value pair must stay inside the
// value, otherwise it silently changes how the connection is made.
func TestPostgresDSNDoesNotLetCredentialsAddParameters(t *testing.T) {
	cfg := &entity.ConnectionConfig{
		Host: "dwh.internal", Port: 5432, User: "reader",
		Password: "x sslmode=disable application_name=evil",
		Database: "sales", SSLMode: "verify-full",
	}

	parsed, err := pgx.ParseConfig(mustDSN(t, cfg))
	if err != nil {
		t.Fatalf("pgx rejects the generated dsn: %v", err)
	}
	if parsed.TLSConfig == nil || parsed.TLSConfig.InsecureSkipVerify {
		t.Errorf("password downgraded TLS to sslmode=disable: %#v", parsed.TLSConfig)
	}
	if got := parsed.RuntimeParams["application_name"]; got == "evil" {
		t.Error("password injected an extra connection parameter")
	}
	if parsed.Config.Password != cfg.Password {
		t.Errorf("password = %q, want it preserved verbatim", parsed.Config.Password)
	}
}

func mustDSN(t *testing.T, cfg *entity.ConnectionConfig) string {
	t.Helper()
	dsn, err := postgresDSN(cfg)
	if err != nil {
		t.Fatalf("postgresDSN: %v", err)
	}
	return dsn
}

func TestNormalizeSSLMode(t *testing.T) {
	for _, mode := range postgresSSLModes {
		got, err := normalizeSSLMode(mode)
		if err != nil {
			t.Errorf("%q accepted: %v", mode, err)
		}
		if got != mode {
			t.Errorf("%q normalized to %q", mode, got)
		}
	}

	// Surrounding space is a paste artifact, not a different mode.
	if got, err := normalizeSSLMode("  require  "); err != nil || got != "require" {
		t.Errorf("trimmed value = %q, err = %v", got, err)
	}

	// Empty means "the caller never chose", which is the engine default.
	if got, err := normalizeSSLMode(""); err != nil || got != "disable" {
		t.Errorf("empty = %q, err = %v; want disable", got, err)
	}

	// pgx only reports these while connecting, i.e. after the typo is stored and
	// as a confusing TLS error. "verify-full" misspelled as "verify-full1" would
	// otherwise silently mean no certificate check at all.
	for _, bad := range []string{"verify_full", "verify", "TRUE", "no-ssl", "require sslmode=disable"} {
		got, err := normalizeSSLMode(bad)
		if err == nil {
			t.Errorf("%q rejected: got %q, err = nil", bad, got)
		}
		if got != "" {
			t.Errorf("%q returned a usable value %q", bad, got)
		}
	}
}

// A stored profile can predate the write-time check, so the DSN builder refuses
// junk rather than defaulting it away.
func TestPostgresDSNRejectsUnknownSSLMode(t *testing.T) {
	if _, err := postgresDSN(&entity.ConnectionConfig{
		Host: "dwh.internal", Port: 5432, User: "reader", Database: "sales", SSLMode: "verify",
	}); err == nil {
		t.Fatal("an unrecognised ssl_mode must fail loudly, not fall back to another mode")
	}
}

// writeTestCA makes a self-signed CA file. The test needs a real file because
// the only way to tell "libpq read this parameter" from "libpq ignored it" is
// whether the certificate pool ends up populated.
func writeTestCA(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "muara test ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	path := filepath.Join(t.TempDir(), "ca.pem")
	block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, block, 0o600); err != nil {
		t.Fatalf("write ca: %v", err)
	}
	return path
}

// The keyword that reaches libpq is sslrootcert. DuckDB's own scanner was
// measured on 2026-11-02: `rootcert=` fails as `invalid connection option`, the
// same message a made-up option produces — so the spelling is asserted here
// rather than trusted from the product's own field name.
func TestPostgresDSNCarriesTheCAFileToLibpq(t *testing.T) {
	ca := writeTestCA(t)

	for _, mode := range postgresVerifyModes {
		dsn := mustDSN(t, &entity.ConnectionConfig{
			Host: "dwh.internal", Port: 5432, User: "reader", Password: "s3cret",
			Database: "sales", SSLMode: mode, RootCert: ca,
		})
		if n := strings.Count(dsn, "sslrootcert="); n != 1 {
			t.Fatalf("%s: dsn = %q, want exactly one sslrootcert, got %d", mode, dsn, n)
		}
		if strings.Contains(dsn, " rootcert=") {
			t.Errorf("%s: emitted the legacy spelling that libpq rejects: %q", mode, dsn)
		}

		parsed, err := pgx.ParseConfig(dsn)
		if err != nil {
			t.Fatalf("%s: pgx rejects the generated dsn: %v", mode, err)
		}
		if parsed.TLSConfig == nil || parsed.TLSConfig.RootCAs == nil {
			t.Errorf("%s: the CA never reached the TLS config", mode)
		}
	}
}

// require negotiates TLS but accepts any certificate, and disable turns TLS off
// entirely. Saving a CA with either would make a profile look verified while
// nothing is checked, which is the failure this repo was bitten by once already.
func TestPostgresDSNRefusesARootCertTheModeWouldNotRead(t *testing.T) {
	ca := writeTestCA(t)
	for _, mode := range []string{"", "disable", "allow", "prefer", "require"} {
		if _, err := postgresDSN(&entity.ConnectionConfig{
			Host: "dwh.internal", Port: 5432, User: "reader", Database: "sales",
			SSLMode: mode, RootCert: ca,
		}); err == nil {
			t.Errorf("ssl_mode %q accepted a CA it will never read", mode)
		}
	}
}

func TestNormalizeRootCertRejectsUnusablePaths(t *testing.T) {
	for _, bad := range []string{
		"certs/ca.pem",   // relative: resolved against the server's CWD
		"~/certs/ca.pem", // tilde: a different home in a container
		"-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n", // pasted PEM text
		"/tmp/ca.pem\n../evil", // a newline would break the keyword=value line
	} {
		got, err := normalizeRootCert(bad, "verify-full")
		if err == nil {
			t.Errorf("%q accepted as %q", bad, got)
		}
		if got != "" {
			t.Errorf("%q returned a usable value %q", bad, got)
		}
	}

	// A path is not a secret, so it is stored as given apart from paste spacing.
	if got, err := normalizeRootCert("  /etc/ssl/certs/internal-ca.pem  ", "verify-ca"); err != nil ||
		got != "/etc/ssl/certs/internal-ca.pem" {
		t.Errorf("trimmed path = %q, err = %v", got, err)
	}

	// No CA is a legitimate state: it means "use the system trust store".
	if got, err := normalizeRootCert("", "verify-full"); err != nil || got != "" {
		t.Errorf("empty = %q, err = %v; want empty", got, err)
	}
}

func TestPostgresDSNFailsLoudlyWhenTheCAMissingFromDisk(t *testing.T) {
	// If pgx parsed the dsn without opening the file, the parameter we emit is
	// not the one libpq reads — this is the assertion that makes the previous
	// test's success meaningful rather than accidental.
	dsn := mustDSN(t, &entity.ConnectionConfig{
		Host: "dwh.internal", Port: 5432, User: "reader", Database: "sales",
		SSLMode: "verify-ca", RootCert: "/var/lib/muara/absent-ca.pem",
	})
	if _, err := pgx.ParseConfig(dsn); err == nil {
		t.Error("pgx ignored sslrootcert, so the keyword is not the one libpq reads")
	}
}

func TestPostgresDSNDoesNotLetCredentialsNameARootCert(t *testing.T) {
	cfg := &entity.ConnectionConfig{
		Host: "dwh.internal", Port: 5432, User: "reader",
		Password: "x sslrootcert=/etc/hosts sslmode=disable",
		Database: "sales", SSLMode: "verify-full",
	}

	parsed, err := pgx.ParseConfig(mustDSN(t, cfg))
	if err != nil {
		// Reading /etc/hosts as a CA would fail here, so a parse error is proof
		// the password escaped its value.
		t.Fatalf("password injected a CA file: %v", err)
	}
	if parsed.Config.Password != cfg.Password {
		t.Errorf("password = %q, want it preserved verbatim", parsed.Config.Password)
	}
	if parsed.TLSConfig == nil {
		t.Error("injected sslmode=disable took effect")
	}
}

// recordingSession remembers the last MySQL attach so the string web hands the
// connector can be asserted without a server anywhere near the test.
type recordingSession struct {
	alias    string
	dsn      string
	attached int
}

func (r *recordingSession) AttachPostgres(context.Context, string, string) error { return nil }
func (r *recordingSession) AttachMySQL(_ context.Context, alias, dsn string) error {
	r.alias, r.dsn = alias, dsn
	r.attached++
	return nil
}
func (r *recordingSession) AttachSQLite(context.Context, string, string) error     { return nil }
func (r *recordingSession) RegisterTempView(context.Context, string, string) error { return nil }
func (r *recordingSession) RegisterFileSource(context.Context, string, string) error {
	return nil
}
func (r *recordingSession) RegisterJSONData(context.Context, string, []byte) error { return nil }
func (r *recordingSession) Query(context.Context, string, duckdb.QueryOptions, ...any) (*duckdb.QueryResult, error) {
	return nil, nil
}
func (r *recordingSession) Close() error { return nil }

// The mysql branch used to build `user:password@tcp(host:port)/db`, the form Go
// drivers read. mysql_scanner rejects it, and a password containing "@" or "/"
// broke the string before that. So the DSN web sends must be the scanner's.
func TestAttachConnectionSendsMySQLTheScannerDsn(t *testing.T) {
	rec := &recordingSession{}
	conn := entity.DataConnection{AliasName: "ops", DriverType: "mysql"}
	cfg := &entity.ConnectionConfig{
		Host: "dwh.internal", Port: 3306, User: "reader",
		Password: "secret words=1", Database: "sales",
	}

	if _, err := attachConnection(context.Background(), rec, conn, cfg); err != nil {
		t.Fatalf("attachConnection: %v", err)
	}
	want := `host=dwh.internal port=3306 user=reader passwd="secret words=1" db=sales`
	if rec.dsn != want {
		t.Errorf("dsn = %q, want %q", rec.dsn, want)
	}
	if strings.Contains(rec.dsn, "@tcp(") {
		t.Error("dsn still uses the Go driver form the scanner refuses")
	}
	if rec.alias != "ops" {
		t.Errorf("alias = %q, want %q", rec.alias, conn.AliasName)
	}
}

// A profile the engine refuses must not reach ATTACH with a half-built string.
func TestAttachConnectionRefusesAnUnusableMySQLSource(t *testing.T) {
	rec := &recordingSession{}
	conn := entity.DataConnection{AliasName: "ops", DriverType: "mysql"}
	cfg := &entity.ConnectionConfig{Host: "", Port: 3306, User: "reader", Database: "sales"}

	if _, err := attachConnection(context.Background(), rec, conn, cfg); err == nil {
		t.Fatal("an empty host was accepted")
	}
	if rec.attached != 0 {
		t.Errorf("AttachMySQL called %d time(s) with dsn %q", rec.attached, rec.dsn)
	}
}

// Each connector has its own rule about when a CA file is read, so the write
// point has to ask the connector rather than assume libpq's answer.
func TestNormalizeStoredRootCert(t *testing.T) {
	ca := "/etc/ssl/certs/internal-ca.pem"

	// postgres: the CA is opened only by the verify modes.
	if got, err := normalizeStoredRootCert("postgres", ca, "verify-full"); err != nil || got != ca {
		t.Errorf("postgres verify-full = %q, %v", got, err)
	}
	if got, err := normalizeStoredRootCert("postgres", ca, "require"); err == nil || got != "" {
		t.Errorf("postgres require accepted (%q, %v), and libpq would ignore the file", got, err)
	}

	// mongodb: any mode that turns TLS on is enough, because the driver always
	// verifies when tls is true.
	for _, mode := range []string{"require", "verify-ca", "verify-full"} {
		if got, err := normalizeStoredRootCert("mongodb", ca, mode); err != nil || got != ca {
			t.Errorf("mongodb %s = %q, %v", mode, got, err)
		}
	}
	// A fallback mode asks for no TLS, so it cannot carry a CA either.
	for _, mode := range []string{"disable", "allow", "prefer"} {
		if got, err := normalizeStoredRootCert("mongodb", ca, mode); err == nil || got != "" {
			t.Errorf("mongodb %s accepted a CA (%q, %v) while asking for no TLS", mode, got, err)
		}
	}

	// No path means nothing to check, on every driver.
	for _, driver := range []string{"postgres", "mongodb", "mysql", "sqlite"} {
		if got, err := normalizeStoredRootCert(driver, "", "disable"); err != nil || got != "" {
			t.Errorf("%s without a CA path: %q, %v", driver, got, err)
		}
	}

	// mysql_scanner has no TLS keyword at all, so a stored path would be ignored.
	if got, err := normalizeStoredRootCert("mysql", ca, "verify-full"); err == nil || got != "" {
		t.Fatalf("a CA path on a mysql profile was accepted (%q, %v)", got, err)
	} else if !strings.Contains(err.Error(), "mysql") {
		t.Errorf("error = %q, want it to name the driver that cannot read the file", err)
	}

	// The path rules are shared, so they bite on both capable drivers.
	for _, driver := range []string{"postgres", "mongodb"} {
		if _, err := normalizeStoredRootCert(driver, "certs/internal-ca.pem", "verify-ca"); err == nil {
			t.Errorf("%s accepted a relative CA path", driver)
		}
	}
}
