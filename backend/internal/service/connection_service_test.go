package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/engine/cryptohelper"
)

// A connection test answer is shown in the UI, and drivers wrap the URI they
// failed on into that error text.
func TestRedactCredentials(t *testing.T) {
	cases := []struct {
		name     string
		msg      string
		cfg      *entity.ConnectionConfig
		leaked   []string
		wantPart string
	}{
		{
			name:     "connection string userinfo",
			msg:      "failed to connect to mongodb: mongodb://mongo_user:mongo_password@sample_mongo:27017 failed",
			cfg:      &entity.ConnectionConfig{URI: "mongodb://mongo_user:mongo_password@sample_mongo:27017/?authSource=admin"},
			leaked:   []string{"mongo_password", "mongodb://mongo_user:mongo_password"},
			wantPart: "<credentials>",
		},
		{
			name:     "host fields build the same uri",
			msg:      "auth failed for mongo_user:mongoPass123@sample_mongo:27017",
			cfg:      &entity.ConnectionConfig{Host: "sample_mongo", Port: 27017, User: "mongo_user", Password: "mongoPass123"},
			leaked:   []string{"mongoPass123"},
			wantPart: "<redacted>",
		},
		{
			name:     "bare password in a sql error",
			msg:      "pq: password authentication failed for user \"sample_user\" (s3cr3t!)",
			cfg:      &entity.ConnectionConfig{Host: "sample_postgres", Port: 5432, User: "sample_user", Password: "s3cr3t!"},
			leaked:   []string{"s3cr3t!"},
			wantPart: "authentication failed",
		},
		{
			name:     "no credentials to hide",
			msg:      "dial tcp: lookup sample_mongo: no such host",
			cfg:      &entity.ConnectionConfig{Host: "sample_mongo", Port: 27017},
			leaked:   nil,
			wantPart: "no such host",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactCredentials(tc.msg, tc.cfg)
			for _, secret := range tc.leaked {
				if strings.Contains(got, secret) {
					t.Errorf("redacted message still contains %q: %s", secret, got)
				}
			}
			if !strings.Contains(got, tc.wantPart) {
				t.Errorf("redacted message = %q, want it to keep %q", got, tc.wantPart)
			}
		})
	}
}

// fakeConnRepo is the storage the connection service reads without a database.
type fakeConnRepo struct {
	conn     entity.DataConnection
	envs     []entity.ConnectionEnvironment
	savedEnv *entity.ConnectionEnvironment
}

func (f *fakeConnRepo) CreateConnection(context.Context, *entity.DataConnection) error { return nil }
func (f *fakeConnRepo) GetConnectionByID(context.Context, string, string) (*entity.DataConnection, error) {
	return &f.conn, nil
}
func (f *fakeConnRepo) GetConnectionByAlias(context.Context, string, string) (*entity.DataConnection, error) {
	return nil, nil
}
func (f *fakeConnRepo) ListConnections(context.Context, string) ([]entity.DataConnection, error) {
	return []entity.DataConnection{f.conn}, nil
}
func (f *fakeConnRepo) DeleteConnection(context.Context, string, string) error { return nil }
func (f *fakeConnRepo) UpsertEnvironment(_ context.Context, env *entity.ConnectionEnvironment) error {
	f.savedEnv = env
	return nil
}
func (f *fakeConnRepo) GetEnvironment(context.Context, string, string) (*entity.ConnectionEnvironment, error) {
	return nil, nil
}
func (f *fakeConnRepo) ListEnvironments(context.Context, string) ([]entity.ConnectionEnvironment, error) {
	return f.envs, nil
}
func (f *fakeConnRepo) UpdateCachedSchema(context.Context, string, string, []byte) error { return nil }

const testMasterKey = "01234567890123456789012345678901" // 32 bytes for AES-256

func newTestConnectionService(t *testing.T, repo *fakeConnRepo) *ConnectionService {
	t.Helper()
	crypto, err := cryptohelper.NewCryptoHelper(testMasterKey)
	if err != nil {
		t.Fatalf("crypto helper: %v", err)
	}
	return NewConnectionService(repo, crypto, nil, nil)
}

// The form prefills its SSL fields from this response, so a setting the API does
// not return is a setting the next save silently rewrites: ssl_mode falls back to
// the form's initial "prefer" and a stored CA path is wiped.
func TestListConnectionsReturnsTLSSettings(t *testing.T) {
	repo := &fakeConnRepo{
		conn: entity.DataConnection{ID: "conn-1", WorkspaceID: "ws-1", AliasName: "warehouse", DriverType: "postgres"},
	}
	crypto, err := cryptohelper.NewCryptoHelper(testMasterKey)
	if err != nil {
		t.Fatalf("crypto helper: %v", err)
	}
	encrypted, err := crypto.EncryptJSON(entity.ConnectionConfig{
		Host:     "db.internal",
		Port:     5432,
		User:     "analyst",
		Password: "s3cr3t",
		SSLMode:  "verify-ca",
		RootCert: "/etc/ssl/certs/internal-ca.pem",
		URI:      "postgres://analyst:s3cr3t@db.internal:5432/analytics",
	})
	if err != nil {
		t.Fatalf("encrypt config: %v", err)
	}
	repo.envs = []entity.ConnectionEnvironment{{
		ID:              "env-1",
		ConnectionID:    "conn-1",
		EnvTag:          "production",
		DisplayName:     "Production",
		EncryptedConfig: encrypted,
	}}

	conns, err := newTestConnectionService(t, repo).ListConnections(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(conns) != 1 || len(conns[0].Environments) != 1 {
		t.Fatalf("want 1 connection with 1 environment, got %+v", conns)
	}
	env := conns[0].Environments[0]
	if env.SSLMode != "verify-ca" {
		t.Errorf("ssl_mode = %q, want it to survive the read-back", env.SSLMode)
	}
	if env.RootCert != "/etc/ssl/certs/internal-ca.pem" {
		t.Errorf("root_cert = %q, want it to survive the read-back", env.RootCert)
	}

	// The response is public to any workspace member, so the credential half of
	// the stored config must not travel with the TLS settings.
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal env: %v", err)
	}
	for _, secret := range []string{"s3cr3t", "password", "uri"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("environment response leaks %q: %s", secret, raw)
		}
	}
}

// Saving normalises both settings, and the response is what the form is rebuilt
// from, so it has to carry the stored value rather than the one just typed in.
func TestUpsertEnvironmentReturnsNormalizedTLSSettings(t *testing.T) {
	repo := &fakeConnRepo{
		conn: entity.DataConnection{ID: "conn-1", WorkspaceID: "ws-1", AliasName: "warehouse", DriverType: "postgres"},
	}

	env, err := newTestConnectionService(t, repo).UpsertEnvironment(context.Background(), "ws-1", "conn-1",
		request.UpsertEnvironmentRequest{
			EnvTag:   "production",
			Host:     "db.internal",
			Port:     5432,
			User:     "analyst",
			Password: "s3cr3t",
			SSLMode:  " verify-ca ",
			RootCert: " /etc/ssl/certs/internal-ca.pem ",
		})
	if err != nil {
		t.Fatalf("UpsertEnvironment: %v", err)
	}
	if env.SSLMode != "verify-ca" || env.RootCert != "/etc/ssl/certs/internal-ca.pem" {
		t.Errorf("response = ssl_mode %q, root_cert %q, want both trimmed", env.SSLMode, env.RootCert)
	}

	// A driver that cannot read a CA must not be able to store one, and the
	// response then reports the empty value the form shows.
	sqlite := &fakeConnRepo{
		conn: entity.DataConnection{ID: "conn-2", WorkspaceID: "ws-1", AliasName: "ledger", DriverType: "sqlite"},
	}
	_, err = newTestConnectionService(t, sqlite).UpsertEnvironment(context.Background(), "ws-1", "conn-2",
		request.UpsertEnvironmentRequest{
			EnvTag:   "production",
			Host:     "/var/lib/ledger.db",
			SSLMode:  "verify-ca",
			RootCert: "/etc/ssl/certs/internal-ca.pem",
		})
	if err == nil {
		t.Fatal("want sqlite to reject a root_cert it cannot read")
	}
	if !strings.Contains(err.Error(), "root_cert is not read") {
		t.Errorf("err = %v, want the connector reason", err)
	}
}
