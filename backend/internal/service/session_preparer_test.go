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
	dsn := postgresDSN(cfg)
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
	parsed, err := pgx.ParseConfig(postgresDSN(&entity.ConnectionConfig{
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

	parsed, err := pgx.ParseConfig(postgresDSN(cfg))
	if err != nil {
		t.Fatalf("pgx rejects the generated dsn: %v (%q)", err, postgresDSN(cfg))
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

	parsed, err := pgx.ParseConfig(postgresDSN(cfg))
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
