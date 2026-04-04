package store

import "testing"

func TestRAGIndexingConfig_SupportsExt_Disabled(t *testing.T) {
	c := RAGIndexingConfig{
		Enabled:        false,
		SupportedTypes: []string{".pdf"},
	}
	if c.SupportsExt(".pdf") {
		t.Fatal("SupportsExt should be false when disabled")
	}
}

func TestRAGIndexingConfig_SupportsExt_NormalizesLeadingDotAndCase(t *testing.T) {
	c := RAGIndexingConfig{
		Enabled:        true,
		SupportedTypes: []string{"PDF", ".Docx"},
	}
	if !c.SupportsExt(".pdf") {
		t.Fatal("expected .pdf to be supported")
	}
	if !c.SupportsExt("docx") {
		t.Fatal("expected docx to be supported")
	}
	if c.SupportsExt("") {
		t.Fatal("expected empty ext to be unsupported")
	}
}

