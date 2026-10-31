package duckdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
)

// Federated E2E against the sample sources defined in docker-compose.yml:
// PostgreSQL is mounted through postgres_scanner, MongoDB through the Go
// streaming adapter, and both are joined inside one DuckDB session.
//
//	MUARA_E2E_FEDERATED=1 go test ./engine/duckdb -run Federated -v
//
// Optional overrides: MUARA_E2E_PG_DSN, MUARA_E2E_MONGO_URI, MUARA_E2E_MONGO_DB.
func TestFederatedPostgresMongo(t *testing.T) {
	if os.Getenv("MUARA_E2E_FEDERATED") != "1" {
		t.Skip("set MUARA_E2E_FEDERATED=1 to run against the docker-compose sample sources")
	}

	pgDSN := envOr("MUARA_E2E_PG_DSN",
		"host=127.0.0.1 port=5435 user=sample_user password=sample_password dbname=sample_ecommerce sslmode=disable")
	mongoURI := envOr("MUARA_E2E_MONGO_URI",
		"mongodb://mongo_user:mongo_password@127.0.0.1:27017/?authSource=admin&directConnection=true")
	mongoDB := envOr("MUARA_E2E_MONGO_DB", "sample_analytics")

	ctx := context.Background()
	engine, err := NewEngine(DefaultGuardrailConfig())
	if errors.Is(err, ErrCGODisabled) {
		t.Skip(ErrCGODisabled.Error())
	}
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	session, err := engine.NewSession(ctx)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer session.Close()

	if err := session.AttachPostgres(ctx, "dwh", pgDSN); err != nil {
		t.Fatalf("attach postgres: %v", err)
	}

	mounts, err := StreamMongoCollections(ctx, session, MongoCollectionStream{
		URI:      mongoURI,
		Database: mongoDB,
		Alias:    "logs",
	})
	if err != nil {
		t.Fatalf("stream mongodb: %v", err)
	}
	mounted := make(map[string]int64, len(mounts))
	for _, m := range mounts {
		mounted[m.View] = m.Documents
	}
	for _, want := range []string{"logs_page_views", "logs_sessions"} {
		if _, ok := mounted[want]; !ok {
			t.Fatalf("mounted views = %v, want %s in it", mounted, want)
		}
	}
	// Listing the document counts is the evidence that a Mongo aggregate over
	// more than DefaultMongoDocLimit documents would have to be flagged.
	t.Logf("mongodb mount: %v", mounted)
	if capped := CappedMongoViews(mounts); len(capped) > 0 {
		t.Logf("source-capped collections: %v", capped)
	}

	for _, tc := range []struct {
		name  string
		sql   string
		args  map[string]any
		check func(*testing.T, *QueryResult)
	}{
		{
			name: "postgres side is readable",
			sql:  `SELECT COUNT(*) AS n FROM dwh.main.orders`,
			check: func(t *testing.T, res *QueryResult) {
				if got := fmt.Sprintf("%v", res.Rows[0]["n"]); got != "6" {
					t.Errorf("orders count = %v, want 6", got)
				}
			},
		},
		{
			name: "mongodb side keeps flattened columns",
			sql:  `SELECT session_id, customer_name, customer_address_city, tags FROM logs_sessions ORDER BY session_id`,
			check: func(t *testing.T, res *QueryResult) {
				if res.RowCount != 7 {
					t.Fatalf("sessions count = %d, want 7", res.RowCount)
				}
				first := res.Rows[0]
				if first["customer_name"] != "Aisyah" || first["customer_address_city"] != "Jakarta" {
					t.Errorf("flattened columns wrong: %v", first)
				}
				if tags, ok := first["tags"].(string); !ok || tags != `["repeat-buyer","loyalty"]` {
					t.Errorf("tags = %v, want serialized json array", first["tags"])
				}
			},
		},
		{
			name: "federated join with a bound parameter",
			sql: `SELECT o.branch_code, s.customer_address_city AS city, s.device,
				       COUNT(*) AS orders, SUM(o.revenue) AS revenue
		        FROM dwh.main.orders o
		        JOIN logs_sessions s ON s.order_id = o.order_id
		        WHERE s.device = {{device}}
		        GROUP BY 1, 2, 3
		        ORDER BY revenue DESC`,
			args: map[string]any{"device": "ios"},
			check: func(t *testing.T, res *QueryResult) {
				// Orders 1 and 6 are both ios sessions from the same branch, city and
				// device, so they collapse into one grouped row.
				if res.RowCount != 1 {
					t.Fatalf("ios sessions joined = %d rows, want 1: %v", res.RowCount, res.Rows)
				}
				if got := res.Rows[0]["orders"]; fmt.Sprintf("%v", got) != "2" {
					t.Errorf("ios orders = %v, want 2", got)
				}
				if got := res.Rows[0]["revenue"]; got != 180000.0 {
					t.Errorf("ios revenue = %v, want 180000", got)
				}
			},
		},
		{
			name: "federated revenue across the two sources",
			sql: `SELECT o.product, SUM(o.revenue) AS revenue
		        FROM dwh.main.orders o
		        JOIN logs_sessions s ON s.order_id = o.order_id
		        GROUP BY 1
		        ORDER BY 1`,
			check: func(t *testing.T, res *QueryResult) {
				// SES-1007 carries order_id 99, which no postgres row has, so the inner
				// join must drop it and leave the six real conversions.
				if res.RowCount != 4 {
					t.Fatalf("product groups = %d, want 4: %v", res.RowCount, res.Rows)
				}
				var total float64
				for _, row := range res.Rows {
					total += row["revenue"].(float64)
				}
				if total != 537000.0 {
					t.Errorf("federated revenue = %v, want 537000", total)
				}
			},
		},
		{
			name: "three source join across scanners and two streaming views",
			sql: `SELECT o.product, COUNT(pv.view_id) AS page_views
		        FROM dwh.main.orders o
		        JOIN logs_sessions s ON s.order_id = o.order_id
		        JOIN logs_page_views pv ON pv.session_id = s.session_id
		        GROUP BY 1
		        ORDER BY 1`,
			check: func(t *testing.T, res *QueryResult) {
				if res.RowCount != 4 {
					t.Fatalf("product groups = %d, want 4: %v", res.RowCount, res.Rows)
				}
				var views int64
				for _, row := range res.Rows {
					n, err := strconv.ParseInt(fmt.Sprintf("%v", row["page_views"]), 10, 64)
					if err != nil {
						t.Fatalf("page_views %v: %v", row["page_views"], err)
					}
					views += n
				}
				// PV-8 belongs to the orphan session and never reaches an order.
				if views != 7 {
					t.Errorf("joined page views = %d, want 7", views)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sqlText := tc.sql
			var bound []any
			if len(tc.args) > 0 {
				if err := ValidateQuery(sqlText); err != nil {
					t.Fatalf("validate: %v", err)
				}
				binding, err := TransformParameters(sqlText, tc.args)
				if err != nil {
					t.Fatalf("transform: %v", err)
				}
				sqlText = binding.ParameterizedSQL
				bound = binding.OrderedArgs
			}
			res, err := session.Query(ctx, sqlText, QueryOptions{MaxRows: 10000}, bound...)
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			t.Logf("%d rows in %dms (row_limit=%d truncated=%v by=%q)",
				res.RowCount, res.DurationMs, res.RowLimit, res.Truncated, res.TruncatedBy)
			tc.check(t, res)
		})
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
