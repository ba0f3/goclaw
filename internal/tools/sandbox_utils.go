package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/sandbox"
)

// SandboxHostMountRoot returns the host path to bind into the sandbox container.
// With host-path mounting, the container sees the workspace at the same absolute
// path as the host, so this returns the resolved host workspace path.
func SandboxHostMountRoot(ctx context.Context, registryWorkspace string) string {
	ws := ToolWorkspaceFromCtx(ctx)
	if ws == "" {
		return registryWorkspace
	}
	return filepath.Clean(ws)
}

// SandboxCwd returns the effective working directory path inside the sandbox.
// With host-path mounting, the container path equals the host path.
func SandboxCwd(ctx context.Context, hostMountRoot, containerBase string) (string, error) {
	ws := ToolWorkspaceFromCtx(ctx)
	if ws == "" {
		return hostMountRoot, nil
	}
	ws = filepath.Clean(ws)
	root := filepath.Clean(hostMountRoot)
	if ws == root || strings.HasPrefix(ws, root+string(filepath.Separator)) {
		return ws, nil
	}
	return "", fmt.Errorf("workspace %q is outside sandbox mount %q", ws, hostMountRoot)
}

// SandboxHostPathToContainer maps a host path to its container-side path.
// With host-path mounting, all mounted paths appear at the same absolute path
// inside the container, so this returns the host path directly.
func SandboxHostPathToContainer(hostPath, hostMountRoot, containerBase string) (string, error) {
	if hostPath == "" {
		return hostMountRoot, nil
	}
	return filepath.Clean(hostPath), nil
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

// SandboxRoutingKey returns the sandbox key for this tool call when sandboxing
// is enabled for the current request. It returns ("", false) when execution
// should stay on host (no manager, no key, mode off, or agent excluded).
func SandboxRoutingKey(ctx context.Context, mgr sandbox.Manager) (string, bool) {
	if mgr == nil {
		return "", false
	}
	sandboxKey := ToolSandboxKeyFromCtx(ctx)
	if sandboxKey == "" {
		return "", false
	}
	cfg := SandboxConfigFromCtx(ctx)
	if cfg == nil {
		// No per-request override: preserve existing behavior and let manager
		// decide based on its default config.
		return sandboxKey, true
	}
	if cfg.ModeIsOff() {
		return "", false
	}
	if aid := sandbox.ScopeKeyAgentID(sandboxKey); aid != "" && !cfg.ShouldSandbox(aid) {
		return "", false
	}
	return sandboxKey, true
}

// SandboxConfigWithTeamWorkspace returns a sandbox config override that
// includes the team workspace as an extra mount at its host absolute path.
// If baseCfg is nil, returns nil so the sandbox Manager uses its default
// (r.base / m.config). A zero Config has Mode "" which is not ModeOff and
// would incorrectly enable sandboxing.
func SandboxConfigWithTeamWorkspace(ctx context.Context, baseCfg *sandbox.Config) *sandbox.Config {
	teamWs := ToolTeamWorkspaceFromCtx(ctx)
	if teamWs == "" {
		return baseCfg
	}
	if baseCfg == nil {
		return nil
	}
	cfg := *baseCfg
	// Avoid duplicate mount if team workspace is already the primary workspace.
	mount := SandboxHostMountRoot(ctx, "")
	if mount != "" && filepath.Clean(mount) == filepath.Clean(teamWs) {
		return &cfg
	}
	cfg.ExtraMounts = append(cfg.ExtraMounts, sandbox.ExtraMount{
		HostPath:      teamWs,
		ContainerPath: teamWs,
		Access:        sandbox.AccessRW,
	})
	return &cfg
}
