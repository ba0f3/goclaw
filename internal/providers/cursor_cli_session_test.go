package providers

import (
	"strings"
	"testing"
)

func TestCursorBuildArgsDoesNotUseSessionIDFlag(t *testing.T) {
	p := NewCursorCLIProvider("agent")
	workDir := t.TempDir()

	args := p.buildArgs("gpt-5.4", workDir, "session-key", false)
	joined := strings.Join(args, " ")

	if strings.Contains(joined, "--session-id") {
		t.Fatalf("buildArgs() includes unsupported --session-id flag: %q", joined)
	}
}
