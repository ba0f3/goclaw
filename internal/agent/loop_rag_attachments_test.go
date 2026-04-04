package agent

import "testing"

func TestDocumentNameFromMediaTagByPath(t *testing.T) {
	refPath := "/workspace/.uploads/36a44897-39ef-4ae3-82b7-c7168c738210.pdf"
	content := `<media:document name="DigitalOcean Invoice 2026 Mar (110277-541870709).pdf" path="` + refPath + `">` +
		"\n\n[File: DigitalOcean Invoice 2026 Mar (110277-541870709).pdf — use read_document tool to analyze this file]"
	got := documentNameFromMediaTagByPath(content, refPath)
	want := "DigitalOcean Invoice 2026 Mar (110277-541870709).pdf"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
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
