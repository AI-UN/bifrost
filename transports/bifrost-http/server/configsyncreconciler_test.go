package server

import "testing"

func TestCanonicalHash_IgnoresDynamicRuntimeFields(t *testing.T) {
	left := map[string]any{
		"providers": []any{
			map[string]any{
				"name":        "openai",
				"status":      "healthy",
				"description": "available",
				"keys": []any{
					map[string]any{"id": "key-b", "status": "active", "models": []any{"gpt-4o-mini", "gpt-4o"}},
					map[string]any{"id": "key-a", "status": "active"},
				},
			},
		},
		"budgets": []any{
			map[string]any{"id": "budget-1", "max_limit": 100, "current_usage": 10, "updated_at": "first"},
		},
	}
	right := map[string]any{
		"budgets": []any{
			map[string]any{"id": "budget-1", "max_limit": 100, "current_usage": 99, "updated_at": "second"},
		},
		"providers": []any{
			map[string]any{
				"name":        "openai",
				"status":      "error",
				"description": "temporary failure",
				"keys": []any{
					map[string]any{"id": "key-b", "status": "error", "models": []any{"gpt-4o-mini", "gpt-4o"}},
					map[string]any{"id": "key-a", "status": "error"},
				},
			},
		},
	}

	leftHash, err := canonicalHash("snapshot", left)
	if err != nil {
		t.Fatalf("hash left snapshot: %v", err)
	}
	rightHash, err := canonicalHash("snapshot", right)
	if err != nil {
		t.Fatalf("hash right snapshot: %v", err)
	}
	if leftHash != rightHash {
		t.Fatalf("dynamic fields changed canonical hash: %s != %s", leftHash, rightHash)
	}
}

func TestCanonicalHash_RetainsConfigurationDefinitions(t *testing.T) {
	left := map[string]any{"provider": "openai", "max_retries": 1}
	right := map[string]any{"provider": "openai", "max_retries": 2}

	leftHash, err := canonicalHash("snapshot", left)
	if err != nil {
		t.Fatalf("hash left definition: %v", err)
	}
	rightHash, err := canonicalHash("snapshot", right)
	if err != nil {
		t.Fatalf("hash right definition: %v", err)
	}
	if leftHash == rightHash {
		t.Fatal("definition change must alter canonical hash")
	}
}

func TestCanonicalHash_PluginOrderRemainsSignificant(t *testing.T) {
	left := []any{
		map[string]any{"name": "first"},
		map[string]any{"name": "second"},
	}
	right := []any{
		map[string]any{"name": "second"},
		map[string]any{"name": "first"},
	}

	leftHash, err := canonicalHash("plugin", left)
	if err != nil {
		t.Fatalf("hash left plugin order: %v", err)
	}
	rightHash, err := canonicalHash("plugin", right)
	if err != nil {
		t.Fatalf("hash right plugin order: %v", err)
	}
	if leftHash == rightHash {
		t.Fatal("plugin order must remain significant")
	}
}

func TestCanonicalHash_GovernanceFallbackOrderRemainsSignificant(t *testing.T) {
	left := map[string]any{"parsed_fallbacks": []any{"anthropic/claude", "openai/gpt-4o"}}
	right := map[string]any{"parsed_fallbacks": []any{"openai/gpt-4o", "anthropic/claude"}}

	leftHash, err := canonicalHash("governance", left)
	if err != nil {
		t.Fatalf("hash left fallback order: %v", err)
	}
	rightHash, err := canonicalHash("governance", right)
	if err != nil {
		t.Fatalf("hash right fallback order: %v", err)
	}
	if leftHash == rightHash {
		t.Fatal("fallback order must remain significant")
	}
}
