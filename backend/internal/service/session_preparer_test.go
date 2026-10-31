package service

import (
	"net/url"
	"testing"

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
