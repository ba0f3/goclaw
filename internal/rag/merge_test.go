package rag

import (
	"encoding/json"
	"testing"
)

func TestEnrichOtherConfigWithRAG_DisabledStripsSupportedTypes(t *testing.T) {
	raw := []byte(`{"rag_indexing":{"enabled":false,"supported_types":[".md",".pdf"]}}`)
	out, deps, err := EnrichOtherConfigWithRAG(raw)
	if err != nil {
		t.Fatal(err)
	}
	if deps != nil {
		t.Fatal("expected no deps report when disabled")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	var ri map[string]any
	if err := json.Unmarshal(root["rag_indexing"], &ri); err != nil {
		t.Fatal(err)
	}
	if ri["enabled"] != false {
		t.Fatalf("enabled = %v, want false", ri["enabled"])
	}
	if _, ok := ri["supported_types"]; ok {
		t.Fatal("supported_types should be stripped when disabled")
	}
}

func TestEnrichOtherConfigWithRAG_EnabledAddsSupportedTypes(t *testing.T) {
	raw := []byte(`{"rag_indexing":{"enabled":true}}`)
	out, deps, err := EnrichOtherConfigWithRAG(raw)
	if err != nil {
		t.Fatal(err)
	}
	if deps == nil {
		t.Fatal("expected deps report when enabled")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	var ri map[string]any
	if err := json.Unmarshal(root["rag_indexing"], &ri); err != nil {
		t.Fatal(err)
	}
	if ri["enabled"] != true {
		t.Fatalf("enabled = %v, want true", ri["enabled"])
	}
	st, ok := ri["supported_types"].([]any)
	if !ok || len(st) == 0 {
		t.Fatalf("expected non-empty supported_types, got %#v", ri["supported_types"])
	}
}
