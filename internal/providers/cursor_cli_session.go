package providers

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// validCursorModes lists accepted working modes for the Cursor CLI.
var validCursorModes = map[string]bool{
	"agent": true,
	"plan":  true,
	"ask":   true,
}

func (p *CursorCLIProvider) buildArgs(model, workDir, sessionKey string, streaming bool) []string {
	args := []string{
		"-p",
		"--force",
		"--trust",
		"--workspace", workDir,
	}
	if streaming {
		args = append(args, "--output-format", "stream-json")
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	mode := p.defaultMode
	if !validCursorModes[mode] {
		mode = "agent"
	}
	if mode != "agent" {
		args = append(args, "--mode", mode)
	}
	if sessionKey != "" {
		// Use deterministic session ID for resume support
		sid := deriveCursorSessionUUID(sessionKey).String()
		if cursorSessionFileExists(workDir, sid) {
			args = append(args, "--resume", sid)
		} else {
			args = append(args, "--session-id", sid)
		}
	}
	return args
}

func (p *CursorCLIProvider) ensureWorkDir(sessionKey string) string {
	safe := sanitizePathSegment(sessionKey)
	dir := filepath.Join(p.baseWorkDir, safe)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("cursor-cli: failed to create workdir", "dir", dir, "error", err)
		return os.TempDir()
	}
	return dir
}

func deriveCursorSessionUUID(sessionKey string) uuid.UUID {
	if sessionKey == "" {
		return uuid.New()
	}
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte("cursor:"+sessionKey))
}

func cursorSessionFileExists(workDir, sessionID string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	// Cursor stores sessions at: ~/.cursor/projects/<encoded-path>/agent-transcripts/<session-id>/<session-id>.jsonl
	// Path encoding: replace path separators and special chars with "-"
	resolved, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		resolved = workDir
	}
	encoded := strings.NewReplacer(string(filepath.Separator), "-", "_", "-", ".", "-", ":", "-", " ", "-").Replace(resolved)
	encoded = strings.Trim(encoded, "-")
	sessionFile := filepath.Join(home, ".cursor", "projects", encoded, "agent-transcripts", sessionID, sessionID+".jsonl")
	_, err = os.Stat(sessionFile)
	return err == nil
}

// ResetCursorCLISession deletes the Cursor CLI session file for a given session key.
func ResetCursorCLISession(baseWorkDir, sessionKey string) {
	if baseWorkDir == "" {
		baseWorkDir = defaultCursorWorkDir()
	}
	safe := sanitizePathSegment(sessionKey)
	workDir := filepath.Join(baseWorkDir, safe)
	sessionID := deriveCursorSessionUUID(sessionKey).String()

	home, err := os.UserHomeDir()
	if err == nil {
		resolved, err := filepath.EvalSymlinks(workDir)
		if err != nil {
			resolved = workDir
		}
		encoded := strings.NewReplacer(string(filepath.Separator), "-", "_", "-", ".", "-", ":", "-", " ", "-").Replace(resolved)
		encoded = strings.Trim(encoded, "-")
		sessionFile := filepath.Join(home, ".cursor", "projects", encoded, "agent-transcripts", sessionID, sessionID+".jsonl")
		if err := os.Remove(sessionFile); err == nil {
			slog.Info("cursor-cli: deleted session file on /reset", "path", sessionFile)
		}
	}
}
