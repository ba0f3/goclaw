package agent

import "testing"

func TestDocumentNameFromMediaTagByPath(t *testing.T) {
	refPath := "/workspace/.uploads/36a44897-39ef-4ae3-82b7-c7168c738210.pdf"
	display := "Example Vendor Statement Mar (REF-00000000).pdf"
	content := `<media:document name="` + display + `" path="` + refPath + `">` +
		"\n\n[File: " + display + " — use read_document tool to analyze this file]"
	got := documentNameFromMediaTagByPath(content, refPath)
	if got != display {
		t.Fatalf("got %q, want %q", got, display)
	}
}

func TestRagDocumentPlaceholderNames_prefersDisplayName(t *testing.T) {
	refPath := "/w/uuid.pdf"
	content := `<media:document name="Real Name.pdf" path="` + refPath + `">`
	names := ragDocumentPlaceholderNames(content, refPath)
	if len(names) < 2 || names[0] != "Real Name.pdf" || names[1] != "uuid.pdf" {
		t.Fatalf("unexpected order/names: %#v", names)
	}
}
