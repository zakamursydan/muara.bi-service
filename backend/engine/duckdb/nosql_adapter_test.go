package duckdb

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestFlattenMap(t *testing.T) {
	nested := map[string]any{
		"id": 1,
		"user": map[string]any{
			"name": "Zaka",
			"address": map[string]any{
				"city": "Jakarta",
			},
		},
		"tags": []any{"analytics", "bi"},
	}

	dest := make(map[string]any)
	flattenMap("", nested, dest)

	if dest["id"] != 1 {
		t.Errorf("expected id=1, got %v", dest["id"])
	}

	if dest["user_name"] != "Zaka" {
		t.Errorf("expected user_name='Zaka', got %v", dest["user_name"])
	}

	if dest["user_address_city"] != "Jakarta" {
		t.Errorf("expected user_address_city='Jakarta', got %v", dest["user_address_city"])
	}

	if tags, ok := dest["tags"].(string); !ok || tags != `["analytics","bi"]` {
		t.Errorf("expected serialized json tags, got %v", dest["tags"])
	}
}

func TestFlattenMapHandlesMongoDriverTypes(t *testing.T) {
	// A real cursor decodes into bson/primitive types, never plain []any or string,
	// so a type switch that only matches the plain kinds silently leaves arrays as
	// LIST columns and renders ObjectIDs as `ObjectID("...")`.
	oid := primitive.NewObjectID()
	doc := map[string]any{
		"_id":     oid,
		"user_id": oid,
		"tags":    primitive.A{"analytics", "bi"},
		"nested":  bson.M{"scores": primitive.A{int32(1), int32(2)}},
		"empty":   primitive.A{},
	}

	dest := make(map[string]any)
	flattenMap("", doc, dest)

	if dest["_id"] != oid.Hex() {
		t.Errorf("_id = %v, want hex %v", dest["_id"], oid.Hex())
	}
	if dest["user_id"] != oid.Hex() {
		t.Errorf("user_id = %v, want hex", dest["user_id"])
	}
	if got, ok := dest["tags"].(string); !ok || got != `["analytics","bi"]` {
		t.Errorf("tags = %#v, want serialized json array", dest["tags"])
	}
	if got, ok := dest["nested_scores"].(string); !ok || got != `[1,2]` {
		t.Errorf("nested_scores = %#v, want [1,2]", dest["nested_scores"])
	}
	if got, ok := dest["empty"].(string); !ok || got != "[]" {
		t.Errorf("empty = %#v, want []", dest["empty"])
	}
}

func TestMongoViewName(t *testing.T) {
	cases := []struct{ alias, collection, want string }{
		{"dwh_logs", "sessions", "dwh_logs_sessions"},
		// Aliases may contain dashes (validated as plain identifiers only for the
		// catalog name), and collection names are free-form.
		{"dwh-logs", "user.sessions", "dwh_logs_user_sessions"},
		{"2024_data", "events", "v_2024_data_events"},
		{"logs", "order$", "logs_order"},
	}

	for _, tc := range cases {
		if got := MongoViewName(tc.alias, tc.collection); got != tc.want {
			t.Errorf("MongoViewName(%q, %q) = %q, want %q", tc.alias, tc.collection, got, tc.want)
		}
	}
}

func TestCappedMongoViews(t *testing.T) {
	mounts := []MongoMount{
		{Collection: "orders", View: "logs_orders", Documents: 12},
		{Collection: "sessions", View: "logs_sessions", Documents: DefaultMongoDocLimit, Truncated: true},
		{Collection: "empty", View: "logs_empty", Documents: 0},
	}

	got := CappedMongoViews(mounts)
	if len(got) != 1 || got[0] != "logs_sessions" {
		t.Fatalf("capped views = %v, want [logs_sessions]", got)
	}

	// Nothing capped must read as "no cap", not as an empty list the caller has
	// to re-check element by element.
	if capped := CappedMongoViews([]MongoMount{{View: "logs_orders"}}); capped != nil {
		t.Errorf("uncapped mounts reported as %v, want nil", capped)
	}
}

func TestStreamMongoCollectionsRejectsIncompleteConfig(t *testing.T) {
	// The mount must fail before dialing when the alias or database is missing,
	// otherwise every query would pay a connection timeout for a doomed attach.
	// A nil session is safe here precisely because validation comes first.
	for _, cfg := range []MongoCollectionStream{
		{URI: "mongodb://127.0.0.1:27017", Database: "sample_analytics"},
		{URI: "mongodb://127.0.0.1:27017", Alias: "dwh_events"},
	} {
		if _, err := StreamMongoCollections(t.Context(), nil, cfg); err == nil {
			t.Errorf("expected config error for %+v", cfg)
		}
	}
}

// The product hands the driver a CA path as tlsCAFile, and "the driver reads that
// key" is a claim about the driver, not about our string building. It is measured
// against 127.0.0.1:1 (nothing listens) because the certificate file is opened
// before the server is ever reached — no TLS server needed. The legacy name
// sslCAFile is the reason this test exists: the driver ignores it silently, so a
// profile using it looks configured while nothing is verified.
func TestMongoDriverOpensTLSCAFileButNotSSLCaFile(t *testing.T) {
	ctx := context.Background()
	absent := filepath.Join(t.TempDir(), "absent-ca.pem")

	_, err := connectMongo(ctx, "mongodb://127.0.0.1:1/?tls=true&tlsCAFile="+absent+"&serverSelectionTimeoutMS=400", 3)
	if err == nil || !strings.Contains(err.Error(), "no such file") {
		t.Errorf("tlsCAFile was not opened by the driver: %v", err)
	}

	// An empty value is a failure too, so a profile must omit the key instead.
	if _, err := connectMongo(ctx, "mongodb://127.0.0.1:1/?tls=true&tlsCAFile=&serverSelectionTimeoutMS=400", 3); err == nil {
		t.Error("an empty tlsCAFile= was accepted, so it would be an error at connect time")
	}

	// The legacy name must stay unverified-but-observed: if a future driver ever
	// starts reading it, the log line says so instead of the product guessing.
	if client, err := connectMongo(ctx, "mongodb://127.0.0.1:1/?tls=true&sslCAFile="+absent+"&serverSelectionTimeoutMS=400", 3); err != nil {
		t.Logf("sslCAFile behaviour changed: %v", err)
	} else if client != nil {
		_ = client.Disconnect(context.Background())
	}
}
