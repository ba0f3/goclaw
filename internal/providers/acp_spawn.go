package providers

import (
	"path/filepath"
	"strings"
)

// EffectiveACPArgs returns argv after the binary for ACP subprocesses.
// Cursor's `agent` CLI expects the `acp` subcommand when no args are configured.
func EffectiveACPArgs(binary string, extra []string) []string {
	if len(extra) > 0 {
		out := make([]string, len(extra))
		copy(out, extra)
		return out
	}
	base := filepath.Base(strings.TrimSpace(binary))
	switch base {
	case "agent", "cursor-agent":
		return []string{"acp"}
	case "gemini":
		// Official Gemini CLI ACP transport (see geminicli.com/docs/cli/acp-mode).
		return []string{"--acp"}
	default:
		return nil
	}
}
