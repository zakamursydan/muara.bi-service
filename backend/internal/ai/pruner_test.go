package ai

import (
	"strings"
	"testing"
)

func TestPruneAndMaskSchema_PIIMasking(t *testing.T) {
	tables := []TableSchema{
		{
			AliasName: "db_sales",
			TableName: "users",
			Columns: []ColumnInfo{
				{Name: "id", Type: "INTEGER"},
				{Name: "name", Type: "VARCHAR"},
				{Name: "email", Type: "VARCHAR"},
				{Name: "password_hash", Type: "VARCHAR"},
				{Name: "phone_number", Type: "VARCHAR"},
				{Name: "user_salary", Type: "NUMERIC"},
				{Name: "nik_ktp", Type: "VARCHAR"},
				{Name: "created_at", Type: "TIMESTAMP"},
			},
		},
	}

	selected, ddl := PruneAndMaskSchema("show me users count by date", tables, []string{"ktp"}, 5)

	if len(selected) != 1 {
		t.Fatalf("expected 1 table, got %d", len(selected))
	}

	cols := selected[0].Columns
	colNames := make(map[string]bool)
	for _, c := range cols {
		colNames[c.Name] = true
	}

	// Safe columns should be present
	if !colNames["id"] || !colNames["name"] || !colNames["created_at"] {
		t.Errorf("expected safe columns id, name, created_at; got %+v", cols)
	}

	// PII columns must NOT be present
	piiForbidden := []string{"email", "password_hash", "phone_number", "user_salary", "nik_ktp"}
	for _, f := range piiForbidden {
		if colNames[f] {
			t.Errorf("PII column %s was not masked!", f)
		}
	}

	// DDL check
	if strings.Contains(ddl, "email") || strings.Contains(ddl, "password") {
		t.Errorf("DDL contains masked PII column: %s", ddl)
	}
	if !strings.Contains(ddl, "CREATE TABLE db_sales.users") {
		t.Errorf("DDL missing expected table header: %s", ddl)
	}
}

func TestPruneAndMaskSchema_RelevanceScoringAndLimit(t *testing.T) {
	tables := []TableSchema{
		{
			AliasName: "db",
			TableName: "audit_logs",
			Columns: []ColumnInfo{
				{Name: "id", Type: "INT"},
				{Name: "action", Type: "VARCHAR"},
			},
		},
		{
			AliasName: "db",
			TableName: "orders",
			Columns: []ColumnInfo{
				{Name: "id", Type: "INT"},
				{Name: "order_total", Type: "NUMERIC"},
				{Name: "status", Type: "VARCHAR"},
			},
		},
		{
			AliasName: "db",
			TableName: "order_items",
			Columns: []ColumnInfo{
				{Name: "order_id", Type: "INT"},
				{Name: "product_id", Type: "INT"},
				{Name: "quantity", Type: "INT"},
			},
		},
	}

	// Prompt focuses on orders
	selected, _ := PruneAndMaskSchema("show total revenue from orders and items", tables, nil, 2)

	if len(selected) != 2 {
		t.Fatalf("expected max 2 tables, got %d", len(selected))
	}

	// 'orders' and 'order_items' should score higher than 'audit_logs'
	foundOrders := false
	foundItems := false
	for _, tbl := range selected {
		if tbl.TableName == "orders" {
			foundOrders = true
		}
		if tbl.TableName == "order_items" {
			foundItems = true
		}
	}

	if !foundOrders || !foundItems {
		t.Errorf("expected orders and order_items to be prioritized over audit_logs, got: %+v", selected)
	}
}
