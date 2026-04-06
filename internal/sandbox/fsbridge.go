// Package sandbox — fsbridge.go provides sandboxed file operations via Sandbox.Exec.
// Matching TS src/agents/sandbox/fs-bridge.ts.
//
// When sandbox is enabled, file tools (read_file, write_file, list_files)
// route through FsBridge instead of direct host filesystem access.
package sandbox

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// FsBridge provides sandboxed file operations inside a Sandbox (Docker or bwrap).
type FsBridge struct {
	sb      Sandbox
	workdir string // container-side working directory (e.g. "/workspace")
}

// NewFsBridge creates a bridge that runs file ops through sb.Exec.
func NewFsBridge(sb Sandbox, workdir string) *FsBridge {
	if workdir == "" {
		workdir = "/workspace"
	}
	return &FsBridge{
		sb:      sb,
		workdir: workdir,
	}
}

// ReadFile reads file contents from inside the sandbox.
func (b *FsBridge) ReadFile(ctx context.Context, path string) (string, error) {
	resolved := b.resolvePath(path)

	res, err := b.sb.Exec(ctx, []string{"cat", "--", resolved}, b.workdir)
	if err != nil {
		return "", fmt.Errorf("fsbridge read: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("read failed: %s", strings.TrimSpace(res.Stderr))
	}

	return res.Stdout, nil
}

// WriteFile writes content to a file inside the sandbox, creating directories as needed.
// When append is true, content is appended (shell >>); otherwise the file is overwritten (shell >).
func (b *FsBridge) WriteFile(ctx context.Context, path, content string, appendMode bool) error {
	resolved := b.resolvePath(path)

	dir := resolved[:strings.LastIndex(resolved, "/")]
	if dir != "" && dir != "/" {
		_, _ = b.sb.Exec(ctx, []string{"mkdir", "-p", dir}, b.workdir)
	}

	redir := ">"
	if appendMode {
		redir = ">>"
	}
	res, err := b.sb.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("cat %s %q", redir, resolved)}, b.workdir, WithStdin([]byte(content)))
	if err != nil {
		return fmt.Errorf("fsbridge write: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("write failed: %s", strings.TrimSpace(res.Stderr))
	}

	return nil
}

// ListDir lists files and directories inside the sandbox.
func (b *FsBridge) ListDir(ctx context.Context, path string) (string, error) {
	resolved := b.resolvePath(path)

	res, err := b.sb.Exec(ctx, []string{"ls", "-la", "--", resolved}, b.workdir)
	if err != nil {
		return "", fmt.Errorf("fsbridge list: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("list failed: %s", strings.TrimSpace(res.Stderr))
	}

	return res.Stdout, nil
}

// Stat checks if a path exists and returns basic info.
func (b *FsBridge) Stat(ctx context.Context, path string) (string, error) {
	resolved := b.resolvePath(path)

	res, err := b.sb.Exec(ctx, []string{"stat", "--", resolved}, b.workdir)
	if err != nil {
		return "", fmt.Errorf("fsbridge stat: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("stat failed: %s", strings.TrimSpace(res.Stderr))
	}

	return res.Stdout, nil
}

// resolvePath resolves a path relative to the container workdir.
// Validates that absolute paths stay within the workdir (defense in depth).
func (b *FsBridge) resolvePath(path string) string {
	if path == "" || path == "." {
		return b.workdir
	}
	if strings.HasPrefix(path, "/") {
		cleaned := filepath.Clean(path)
		if cleaned == b.workdir || strings.HasPrefix(cleaned, b.workdir+"/") {
			return cleaned
		}
		return b.workdir
	}
	return filepath.Clean(filepath.Join(b.workdir, path))
}
