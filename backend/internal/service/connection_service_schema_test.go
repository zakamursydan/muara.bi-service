package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/engine/cryptohelper"
	"muara.bi/engine/duckdb"
)

// sealEnv stores one credential profile the way the repository would, so a
// discovery test only has to name the driver it is checking.
func sealEnv(t *testing.T, connID, envTag string, cfg entity.ConnectionConfig) entity.ConnectionEnvironment {
	t.Helper()
	crypto, err := cryptohelper.NewCryptoHelper(testMasterKey)
	if err != nil {
		t.Fatalf("crypto helper: %v", err)
	}
	encrypted, err := crypto.EncryptJSON(cfg)
	if err != nil {
		t.Fatalf("encrypt config: %v", err)
	}
	return entity.ConnectionEnvironment{
		ID:              "env-" + envTag,
		ConnectionID:    connID,
		EnvTag:          envTag,
		EncryptedConfig: encrypted,
	}
}

// A source that answers "synced" while caching nothing is the defect this closes:
// every non-Postgres driver used to fall through the switch silently, so the
// schema tree stayed empty and the UI reported success.
func TestSyncSchemaNamesEveryDriverItCannotDiscover(t *testing.T) {
	cases := []struct {
		driver   string
		wantPart string
	}{
		// A schemaless source has no catalogue: its fields only surface by reading
		// documents, which the metadata-only rule forbids for a cache.
		{driver: "mongodb", wantPart: "no schema to sync"},
		{driver: "redshift", wantPart: `no schema discovery for driver "redshift"`},
	}

	for _, tc := range cases {
		t.Run(tc.driver, func(t *testing.T) {
			repo := &fakeConnRepo{
				conn: entity.DataConnection{ID: "conn-1", WorkspaceID: "ws-1", AliasName: "src", DriverType: tc.driver},
			}
			repo.envs = []entity.ConnectionEnvironment{sealEnv(t, "conn-1", "production", entity.ConnectionConfig{
				Host: "src.internal", Port: 27017, User: "analyst", Password: "s3cr3t", Database: "app",
			})}

			err := newTestConnectionService(t, repo, nil).SyncSchema(context.Background(), "ws-1", "conn-1", "production")
			if err == nil {
				t.Fatalf("%s: want an honest refusal, got success", tc.driver)
			}
			if !strings.Contains(err.Error(), tc.wantPart) {
				t.Errorf("%s: err = %v, want it to say %q", tc.driver, err, tc.wantPart)
			}
			if repo.savedSchema != nil {
				t.Errorf("%s: a refused sync still wrote %s", tc.driver, repo.savedSchema)
			}
		})
	}
}

// MySQL and SQLite have no Go driver in this build; DuckDB's scanner is the only
// reader. Without the engine the sync must say so instead of panicking on the nil.
func TestSyncSchemaScannerDriversNeedTheAnalyticalEngine(t *testing.T) {
	for _, driver := range []string{"mysql", "sqlite"} {
		t.Run(driver, func(t *testing.T) {
			repo := &fakeConnRepo{
				conn: entity.DataConnection{ID: "conn-1", WorkspaceID: "ws-1", AliasName: "src", DriverType: driver},
			}
			repo.envs = []entity.ConnectionEnvironment{sealEnv(t, "conn-1", "production", entity.ConnectionConfig{
				Host: "src.internal", Port: 3306, User: "analyst", Password: "s3cr3t", Database: "sales",
			})}

			err := newTestConnectionService(t, repo, nil).SyncSchema(context.Background(), "ws-1", "conn-1", "production")
			if err == nil {
				t.Fatalf("%s: want the missing engine reported, got success", driver)
			}
			if !strings.Contains(err.Error(), "analytical engine") {
				t.Errorf("%s: err = %v, want it to name the analytical engine", driver, err)
			}
		})
	}
}

// Attaching puts the password inside the statement, and the scanners quote that
// statement back in their error. SyncSchema hands its message to the UI, so the
// secret half has to be stripped on the way.
func TestSyncSchemaRedactsAFailedAttach(t *testing.T) {
	repo := &fakeConnRepo{
		conn: entity.DataConnection{ID: "conn-1", WorkspaceID: "ws-1", AliasName: "warehouse", DriverType: "mysql"},
	}
	repo.envs = []entity.ConnectionEnvironment{sealEnv(t, "conn-1", "production", entity.ConnectionConfig{
		Host: "db.internal", Port: 3306, User: "analyst", Password: "s3cr3t!", Database: "sales",
	})}

	err := newTestConnectionService(t, repo, echoEngine{
		session: echoSession{attachErr: errors.New(`failed to attach mysql db warehouse: Binder Error: Unrecognized configuration parameter "passwd=s3cr3t!"`)},
	}).SyncSchema(context.Background(), "ws-1", "conn-1", "production")
	if err == nil {
		t.Fatal("want the attach failure reported")
	}
	if strings.Contains(err.Error(), "s3cr3t!") {
		t.Errorf("sync error leaks the password: %v", err)
	}
	if !strings.Contains(err.Error(), "Unrecognized configuration parameter") {
		t.Errorf("err = %v, want the connector's own reason kept", err)
	}
	if repo.savedSchema != nil {
		t.Errorf("a failed sync still wrote %s", repo.savedSchema)
	}
}

// echoEngine is the smallest session that gets a sync as far as the catalogue read.
// Only the attach methods, DescribeSource and Close are implemented: any other
// method reaches the nil embedded interface and panics, which is the assertion that
// a schema sync reads no data.
type echoEngine struct {
	duckdb.Engine
	session echoSession
}

func (e echoEngine) NewSession(context.Context) (duckdb.Session, error) {
	return e.session, nil
}

type echoSession struct {
	duckdb.Session
	attachErr error
	tables    []duckdb.CatalogTable
}

func (s echoSession) AttachMySQL(context.Context, string, string) error  { return s.attachErr }
func (s echoSession) AttachSQLite(context.Context, string, string) error { return s.attachErr }

func (s echoSession) DescribeSource(context.Context, string) ([]duckdb.CatalogTable, error) {
	return s.tables, nil
}

func (s echoSession) Close() error { return nil }

// A discovery that sees nothing must not overwrite a cache that works: an empty map
// answers the AI prompt and the schema tree with "this source has no tables".
func TestSyncSchemaRefusesToCacheAnEmptyCatalog(t *testing.T) {
	repo := &fakeConnRepo{
		conn: entity.DataConnection{ID: "conn-1", WorkspaceID: "ws-1", AliasName: "warehouse", DriverType: "mysql"},
	}
	repo.envs = []entity.ConnectionEnvironment{sealEnv(t, "conn-1", "production", entity.ConnectionConfig{
		Host: "db.internal", Port: 3306, User: "analyst", Password: "s3cr3t", Database: "sales",
	})}

	err := newTestConnectionService(t, repo, echoEngine{session: echoSession{}}).
		SyncSchema(context.Background(), "ws-1", "conn-1", "production")
	if err == nil {
		t.Fatal("want an empty catalogue reported as a failure")
	}
	if !strings.Contains(err.Error(), "no queryable tables") {
		t.Errorf("err = %v, want it to say the source exposed no tables", err)
	}
	if repo.savedSchema != nil {
		t.Errorf("an empty discovery still wrote %s", repo.savedSchema)
	}
}

// The catalogue reaches the cache with the shape its readers already parse, and one
// entry per table the scanner reported.
func TestSyncSchemaStoresScannerColumns(t *testing.T) {
	repo := &fakeConnRepo{
		conn: entity.DataConnection{ID: "conn-1", WorkspaceID: "ws-1", AliasName: "warehouse", DriverType: "mysql"},
	}
	repo.envs = []entity.ConnectionEnvironment{sealEnv(t, "conn-1", "production", entity.ConnectionConfig{
		Host: "db.internal", Port: 3306, User: "analyst", Password: "s3cr3t", Database: "sales",
	})}

	tables := []duckdb.CatalogTable{
		{Schema: "sales", Name: "orders", Columns: []duckdb.CatalogColumn{
			{Name: "id", Type: "BIGINT"}, {Name: "total", Type: "DOUBLE"},
		}},
		{Schema: "sales", Name: "customers", Columns: []duckdb.CatalogColumn{
			{Name: "name", Type: "VARCHAR"},
		}},
	}

	if err := newTestConnectionService(t, repo, echoEngine{session: echoSession{tables: tables}}).
		SyncSchema(context.Background(), "ws-1", "conn-1", "production"); err != nil {
		t.Fatalf("SyncSchema: %v", err)
	}

	var cached map[string][]struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(repo.savedSchema, &cached); err != nil {
		t.Fatalf("cached_schema is not what the readers parse: %v (%s)", err, repo.savedSchema)
	}
	if len(cached) != 2 || len(cached["orders"]) != 2 || cached["customers"][0].Type != "VARCHAR" {
		t.Errorf("cached_schema = %s, want both tables with their columns", repo.savedSchema)
	}
	// The cache is stored as plain metadata, so a credential must never travel into
	// it with the column names.
	if strings.Contains(string(repo.savedSchema), "s3cr3t") {
		t.Errorf("cached_schema carries the password: %s", repo.savedSchema)
	}
}
