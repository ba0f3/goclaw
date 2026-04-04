package rag

import (
	"strings"
	"testing"
)

func TestNormalizeExt(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"pdf", ".pdf"},
		{".PDF", ".pdf"},
		{"  .Md  ", ".md"},
	}
	for _, tc := range cases {
		if got := NormalizeExt(tc.in); got != tc.want {
			t.Fatalf("NormalizeExt(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncateForLLM(t *testing.T) {
	// Under limit: trimmed, unchanged.
	if got := truncateForLLM("  hello \n"); got != "hello" {
		t.Fatalf("truncateForLLM under limit = %q, want %q", got, "hello")
	}

	// Over limit: truncated with marker.
	long := strings.Repeat("a", maxExtractedChars+10)
	got := truncateForLLM(long)
	if len(got) <= maxExtractedChars {
		t.Fatalf("truncateForLLM length = %d, want > %d", len(got), maxExtractedChars)
	}
	if !strings.HasSuffix(got, "\n... [truncated]") {
		t.Fatalf("truncateForLLM suffix missing, got %q", got[len(got)-20:])
	}
}

