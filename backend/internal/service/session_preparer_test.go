package service

import (
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"muara.bi/core/internal/domain/entity"
)

func TestMongoURIPrefersStoredConnectionString(t *testing.T) {
	cfg := &entity.ConnectionConfig{
		URI:  "mongodb://user:p%40ss@mongo-a:27017,mongo-b:27017/?replicaSet=rs0",
		Host: "ignored",
		Port: 1234,
	}
	if got := mongoURI(cfg); got != cfg.URI {
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

	parsed, err := url.Parse(mongoURI(cfg))
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
	if got := mongoURI(&entity.ConnectionConfig{Database: "sample_analytics"}); got != "" {
		t.Errorf("mongoURI() = %q, want empty", got)
	}
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
