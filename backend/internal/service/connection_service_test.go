package service

import (
	"strings"
	"testing"

	"muara.bi/core/internal/domain/entity"
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
