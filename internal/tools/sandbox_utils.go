package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// SandboxHostMountRoot returns the host path to bind at the sandbox container workdir
// (e.g. /workspace). registryWorkspace is the tool registry root (ExecTool.workspace),
// used only when ToolWorkspaceFromCtx is empty.
//
// When ToolWorkspaceFromCtx is set (effective session workspace from injectContext:
// DM/group/team paths, or shared agent base when workspace sharing applies), that path
// is mounted so the sandbox sees only that session — not sibling agents or channels.
// Inside the container, containerBase (e.g. /workspace) is the root of that directory.
func SandboxHostMountRoot(ctx context.Context, registryWorkspace string) string {
	ws := ToolWorkspaceFromCtx(ctx)
	if ws == "" {
		return registryWorkspace
	}
	return filepath.Clean(ws)
}

// SandboxCwd maps the current effective workspace (from context) to its
// corresponding path inside the sandbox container. hostMountRoot must match the path
// passed to sandbox.Manager.Get(..., workspace, ...) for this request (use SandboxHostMountRoot).
//
// When hostMountRoot equals the context workspace (typical), this returns containerBase
// (e.g. /workspace). If hostMountRoot is a strict ancestor of the context workspace,
// the result is containerBase plus the relative suffix.
func SandboxCwd(ctx context.Context, hostMountRoot, containerBase string) (string, error) {
	ws := ToolWorkspaceFromCtx(ctx)
	if ws == "" {
		// No per-request workspace — fall back to container root.
		return containerBase, nil
	}

	rel, err := filepath.Rel(hostMountRoot, ws)
	if err != nil || strings.HasPrefix(filepath.Clean(rel), "..") {
		return "", fmt.Errorf("workspace %q is outside sandbox mount %q", ws, hostMountRoot)
	}

	if rel == "." {
		return containerBase, nil
	}
	return filepath.Join(containerBase, rel), nil
}

// SandboxHostPathToContainer maps a host working directory under hostMountRoot to the path
// inside the sandbox (same mount root as Manager.Get). Use for exec cmd.Dir / docker -w.
func SandboxHostPathToContainer(hostPath, hostMountRoot, containerBase string) (string, error) {
	if hostPath == "" {
		return containerBase, nil
	}
	hostPath = filepath.Clean(hostPath)
	root := filepath.Clean(hostMountRoot)
	rel, err := filepath.Rel(root, hostPath)
	if err != nil || strings.HasPrefix(filepath.Clean(rel), "..") {
		return "", fmt.Errorf("path %q is outside sandbox mount %q", hostPath, hostMountRoot)
	}
	if rel == "." {
		return containerBase, nil
	}
	return filepath.Join(containerBase, rel), nil
}

// ResolveSandboxPath resolves a tool-provided path (relative or absolute)
// against the sandbox container CWD. If the path is relative, it is joined
// with containerCwd. Absolute paths are returned as-is (the sandbox
// filesystem already restricts access to the mounted volume).
func ResolveSandboxPath(path, containerCwd string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(containerCwd, path)
}
