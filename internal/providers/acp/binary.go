package acp

import (
	"path/filepath"
	"strings"
)

// BinaryPathAllowed checks whether binary is safe to pass to exec as argv0.
// Absolute paths are allowed. Otherwise only a single PATH token (no slashes,
// no traversal) made of [a-zA-Z0-9._-] — covers agent, cursor-agent, claude, etc.
func BinaryPathAllowed(binary string) bool {
	binary = strings.TrimSpace(binary)
	if binary == "" {
		return false
	}
	if filepath.IsAbs(binary) {
		return true
	}
	if strings.ContainsAny(binary, `/\`) {
		return false
	}
	for _, r := range binary {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}
