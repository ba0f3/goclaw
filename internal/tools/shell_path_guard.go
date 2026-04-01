package tools

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
)

var windowsAbsPathPattern = regexp.MustCompile(`^[a-zA-Z]:[\\/]`)

func (t *ExecTool) validateExecCommandPaths(ctx context.Context, command, cwd string) error {
	allowed := t.execAllowedRoots(ctx)
	for _, token := range extractPathCandidates(command) {
		resolved := resolveExecPathCandidate(cwd, token)
		if !isPathAllowedByRoots(resolved, allowed) {
			slog.Warn("security.exec_path_denied", "path", token, "resolved", resolved, "cwd", cwd)
			return fmt.Errorf("access denied: command references path outside allowed workspace (%s)", token)
		}
	}
	return nil
}

func (t *ExecTool) validateCredentialedArgsPaths(ctx context.Context, args []string, cwd string) error {
	allowed := t.execAllowedRoots(ctx)
	for _, arg := range args {
		if !looksLikePathToken(arg) {
			continue
		}
		resolved := resolveExecPathCandidate(cwd, arg)
		if !isPathAllowedByRoots(resolved, allowed) {
			slog.Warn("security.exec_path_denied", "path", arg, "resolved", resolved, "cwd", cwd, "credentialed", true)
			return fmt.Errorf("access denied: command references path outside allowed workspace (%s)", arg)
		}
	}
	return nil
}

func (t *ExecTool) execAllowedRoots(ctx context.Context) []string {
	ws := ToolWorkspaceFromCtx(ctx)
	if ws == "" {
		ws = t.workspace
	}
	roots := make([]string, 0, len(t.allowedPrefixes)+2)
	roots = append(roots, ws)
	roots = append(roots, allowedWithTeamWorkspace(ctx, t.allowedPrefixes)...)

	out := make([]string, 0, len(roots))
	for _, root := range roots {
		if root == "" {
			continue
		}
		absRoot, _ := filepath.Abs(root)
		rootReal, err := filepath.EvalSymlinks(absRoot)
		if err != nil {
			rootReal = absRoot
		}
		out = append(out, rootReal)
	}
	return out
}

func extractPathCandidates(command string) []string {
	parts := strings.Fields(command)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.Trim(part, `"'()[]{};,`)
		if looksLikePathToken(trimmed) {
			out = append(out, trimmed)
		}
	}
	return out
}

func looksLikePathToken(token string) bool {
	if token == "" {
		return false
	}
	if strings.Contains(token, "://") {
		return false
	}
	if strings.HasPrefix(token, "/") || strings.HasPrefix(token, "./") || strings.HasPrefix(token, "../") {
		return true
	}
	if strings.Contains(token, "/") {
		return true
	}
	return windowsAbsPathPattern.MatchString(token)
}

func resolveExecPathCandidate(cwd, token string) string {
	var candidate string
	if filepath.IsAbs(token) {
		candidate = filepath.Clean(token)
	} else {
		candidate = filepath.Clean(filepath.Join(cwd, token))
	}
	absCandidate, _ := filepath.Abs(candidate)
	real, err := filepath.EvalSymlinks(absCandidate)
	if err != nil {
		return absCandidate
	}
	return real
}

func isPathAllowedByRoots(resolved string, allowedRoots []string) bool {
	for _, root := range allowedRoots {
		if isPathInside(resolved, root) {
			return true
		}
	}
	return false
}
