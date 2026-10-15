package duckdb

import (
	"testing"
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
