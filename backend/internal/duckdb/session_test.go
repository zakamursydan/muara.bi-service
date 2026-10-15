package duckdb

import (
	"context"
	"testing"
)

func TestEngine_Lifecycle(t *testing.T) {
	engine, err := NewEngine(DefaultGuardrailConfig())
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	session, err := engine.NewSession(ctx)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Test registering JSON data
	jsonData := []byte(`[{"id": 1, "product": "Widget A", "price": 100}, {"id": 2, "product": "Widget B", "price": 200}]`)
	if err := session.RegisterJSONData(ctx, "products", jsonData); err != nil {
		t.Fatalf("failed to register JSON data: %v", err)
	}

	// Test querying registered JSON data
	res, err := session.Query(ctx, "SELECT * FROM products")
	if err != nil {
		t.Fatalf("failed to query session: %v", err)
	}

	if res.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", res.RowCount)
	}

	if len(res.Columns) == 0 {
		t.Errorf("expected columns in result, got 0")
	}
}
