// Package sandbox bubblewrap backend uses Linux namespaces (pid, ipc, uts, net)
// for lightweight isolation. Unlike Docker, bwrap shares the host kernel and
// UID space — there is no UID mapping. Processes inside the sandbox run with
// the same UID as the host caller. The environment is sanitized (not inherited
// from host) to prevent credential leakage. System directories are read-only
// binds; /tmp and /var are tmpfs for isolation. Resource limits (memory, CPU,
// pids) require systemd-run --scope and will not be enforced if unavailable.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	bwrapBin      = "/usr/bin/bwrap"
	systemdRunBin = "/usr/bin/systemd-run"
)

// minimalBwrapEnv returns a small, safe environment for sandboxed processes.
// Does NOT inherit the host environment to prevent credential leakage.
func minimalBwrapEnv(containerWorkdir string) []string {
	return []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=" + containerWorkdir,
		"USER=sandbox",
		"TMPDIR=/tmp",
		"LANG=C.UTF-8",
		"TERM=xterm-256color",
	}
}

// CheckBwrapAvailable verifies only that the bubblewrap binary exists and is executable.
// Resource limits (memory/CPU/pids) require systemd-run --scope when available; see probeSystemdRunScope.
func CheckBwrapAvailable(context.Context) error {
	fi, err := os.Stat(bwrapBin)
	if err != nil {
		return fmt.Errorf("%s: %w", bwrapBin, err)
	}
	if fi.Mode()&0o111 == 0 {
		return fmt.Errorf("%s is not executable", bwrapBin)
	}
	return nil
}

// probeSystemdRunScope returns whether systemd-run can create a transient scope (for cgroup limits).
// On failure (e.g. non-root, no user session delegate), logs at DEBUG only so log subscribers at INFO do not see it on the UI stream.
func probeSystemdRunScope(ctx context.Context) bool {
	fi, err := os.Stat(systemdRunBin)
	if err != nil || fi.Mode()&0o111 == 0 {
		slog.Debug("sandbox.bwrap.systemd_run_unavailable",
			"reason", "binary_missing_or_not_executable",
			"hint", "memory_mb/cpus/pids_limit will not be enforced; bubblewrap isolation still works")
		return false
	}
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(runCtx, systemdRunBin, "--scope", "/usr/bin/true").CombinedOutput()
	if err != nil {
		slog.Debug("sandbox.bwrap.systemd_scope_unavailable",
			"hint", "memory_mb/cpus/pids_limit will not be enforced; bubblewrap isolation still works",
			"error", err,
			"output", strings.TrimSpace(string(out)))
		return false
	}
	return true
}

// BwrapSandbox runs each Exec in a fresh bubblewrap namespace (stateless).
type BwrapSandbox struct {
	key               string
	config            Config
	resolvedWorkspace string // host path mounted at ContainerWorkdir(); empty if no workspace bind
	cgroupViaSystemd  bool   // false → run bwrap without systemd-run (no MemoryMax/CPUQuota/TasksMax)
	mu                sync.Mutex
	lastUsed          time.Time
}

// Exec runs command inside bwrap (+ optional systemd-run cgroup wrapper).
func (s *BwrapSandbox) Exec(ctx context.Context, command []string, workDir string, opts ...ExecOption) (*ExecResult, error) {
	s.mu.Lock()
	s.lastUsed = time.Now()
	s.mu.Unlock()

	timeout := time.Duration(s.config.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	o := ApplyExecOpts(opts)

	bwrapArgs := buildBwrapArgs(s.config, s.resolvedWorkspace)
	chdir := bwrapEffectiveChdir(s.config, s.resolvedWorkspace, workDir)
	bwrapArgs = append(bwrapArgs, "--chdir", chdir, "--")
	bwrapArgs = append(bwrapArgs, command...)

	wantLimits := s.config.MemoryMB > 0 || s.config.CPUs > 0 || s.config.PidsLimit > 0
	useSystemdWrap := wantLimits && s.cgroupViaSystemd
	var cmd *exec.Cmd
	if useSystemdWrap {
		sysArgs := []string{"--scope"}
		if s.config.MemoryMB > 0 {
			sysArgs = append(sysArgs, "-p", fmt.Sprintf("MemoryMax=%dM", s.config.MemoryMB))
		}
		if s.config.CPUs > 0 {
			pct := int(math.Round(s.config.CPUs * 100))
			if pct < 1 {
				pct = 1
			}
			sysArgs = append(sysArgs, "-p", fmt.Sprintf("CPUQuota=%d%%", pct))
		}
		if s.config.PidsLimit > 0 {
			sysArgs = append(sysArgs, "-p", fmt.Sprintf("TasksMax=%d", s.config.PidsLimit))
		}
		sysArgs = append(sysArgs, "--", bwrapBin)
		sysArgs = append(sysArgs, bwrapArgs...)
		cmd = exec.CommandContext(execCtx, systemdRunBin, sysArgs...)
	} else {
		cmd = exec.CommandContext(execCtx, bwrapBin, bwrapArgs...)
	}

	cmd.Env = mergeSandboxEnv(minimalBwrapEnv(s.config.ContainerWorkdir()), o.Env)
	if len(o.Stdin) > 0 {
		cmd.Stdin = bytes.NewReader(o.Stdin)
	}

	maxOut := s.config.MaxOutputBytes
	if maxOut <= 0 {
		maxOut = 1 << 20
	}
	stdout := &limitedBuffer{max: maxOut}
	stderr := &limitedBuffer{max: maxOut}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	slog.Debug("sandbox.bwrap.exec",
		"sandbox_key", s.key,
		"path", cmd.Path,
		"args", cmd.Args,
		"work_dir", workDir,
		"effective_chdir", chdir,
		"resolved_workspace", s.resolvedWorkspace,
		"stdin_bytes", len(o.Stdin),
		"cgroup_limits", useSystemdWrap,
	)

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("bwrap exec: %w", err)
		}
	}

	res := &ExecResult{ExitCode: exitCode, Stdout: stdout.String(), Stderr: stderr.String()}
	if stdout.truncated {
		res.Stdout += "\n...[output truncated]"
	}
	if stderr.truncated {
		res.Stderr += "\n...[output truncated]"
	}
	return res, nil
}

// Destroy is a no-op; bwrap has no persistent process per slot.
func (s *BwrapSandbox) Destroy(context.Context) error { return nil }

// ID returns the scope key (not a container id).
func (s *BwrapSandbox) ID() string { return s.key }

func buildBwrapArgs(cfg Config, resolvedHostWS string) []string {
	args := []string{
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/etc", "/etc",
		"--ro-bind", "/usr/local", "/usr/local",
		"--ro-bind", "/opt", "/opt",
	}
	if fi, err := os.Stat("/lib64"); err == nil && fi.IsDir() {
		args = append(args, "--ro-bind", "/lib64", "/lib64")
	}
	args = append(args,
		"--tmpfs", "/var",
		"--tmpfs", "/tmp",
		"--proc", "/proc",
		"--dev", "/dev",
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
	)
	if !cfg.NetworkEnabled {
		args = append(args, "--unshare-net")
	}
	args = append(args, "--die-with-parent")

	cw := cfg.ContainerWorkdir()
	switch cfg.WorkspaceAccess {
	case AccessRW:
		if resolvedHostWS != "" {
			args = append(args, "--bind", resolvedHostWS, cw)
		}
	case AccessRO:
		if resolvedHostWS != "" {
			args = append(args, "--ro-bind", resolvedHostWS, cw)
		}
	}
	for _, p := range normalizeReadOnlyHostPaths(cfg.ReadOnlyHostPaths, resolvedHostWS) {
		args = append(args, "--ro-bind", p, p)
	}
	return args
}

func bwrapEffectiveChdir(cfg Config, resolvedHostWS, workDir string) string {
	if workDir != "" {
		return workDir
	}
	if cfg.WorkspaceAccess != AccessNone && resolvedHostWS != "" {
		return cfg.ContainerWorkdir()
	}
	return "/tmp"
}

func mergeSandboxEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	out := slices.Clone(base)
	for k, v := range extra {
		pair := k + "=" + v
		prefix := k + "="
		replaced := false
		for i, e := range out {
			if strings.HasPrefix(e, prefix) {
				out[i] = pair
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, pair)
		}
	}
	return out
}

// BwrapManager tracks scope keys for API parity with DockerManager (no persistent bwrap processes).
type BwrapManager struct {
	config           Config
	sandboxes        map[string]*BwrapSandbox
	cgroupViaSystemd bool // true if systemd-run --scope works (enforces MemoryMB/CPUs/PidsLimit)
	mu               sync.RWMutex
}

// NewBwrapManager creates a bwrap-backed sandbox manager.
func NewBwrapManager(cfg Config) *BwrapManager {
	useCgroup := probeSystemdRunScope(context.Background())
	return &BwrapManager{
		config:           cfg,
		sandboxes:        make(map[string]*BwrapSandbox),
		cgroupViaSystemd: useCgroup,
	}
}

// CgroupLimitsActive reports whether memory/CPU/pids limits are applied via systemd-run.
func (m *BwrapManager) CgroupLimitsActive() bool {
	if m == nil {
		return false
	}
	return m.cgroupViaSystemd
}

// Get returns or creates a BwrapSandbox for the scope key.
func (m *BwrapManager) Get(ctx context.Context, key string, workspace string, cfgOverride *Config) (Sandbox, error) {
	cfg := m.config
	if cfgOverride != nil {
		cfg = *cfgOverride
	}
	if cfg.Mode == ModeOff {
		return nil, ErrSandboxDisabled
	}

	var resolved string
	if cfg.WorkspaceAccess != AccessNone && strings.TrimSpace(workspace) != "" {
		resolved = resolveHostWorkspacePath(ctx, workspace)
	}
	readOnlyKey := readOnlyHostPathsKey(cfg.ReadOnlyHostPaths, resolved)

	m.mu.Lock()
	defer m.mu.Unlock()
	if sb, ok := m.sandboxes[key]; ok {
		oldReadOnlyKey := readOnlyHostPathsKey(sb.config.ReadOnlyHostPaths, sb.resolvedWorkspace)
		if sb.resolvedWorkspace == resolved && oldReadOnlyKey == readOnlyKey {
			return sb, nil
		}
		delete(m.sandboxes, key)
	}

	sb := &BwrapSandbox{
		key:               key,
		config:            cfg,
		resolvedWorkspace: resolved,
		cgroupViaSystemd:  m.cgroupViaSystemd,
	}
	m.sandboxes[key] = sb
	slog.Debug("bwrap sandbox slot created", "key", key, "workspace_bind", resolved != "", "read_only_host_paths", len(normalizeReadOnlyHostPaths(cfg.ReadOnlyHostPaths, resolved)))
	return sb, nil
}

// Release removes a slot (no host resources to free).
func (m *BwrapManager) Release(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.sandboxes, key)
	m.mu.Unlock()
	return nil
}

// ReleaseAll clears all slots.
func (m *BwrapManager) ReleaseAll(context.Context) error {
	m.mu.Lock()
	m.sandboxes = make(map[string]*BwrapSandbox)
	m.mu.Unlock()
	return nil
}

// Stop is a no-op (no background pruning for bwrap).
func (m *BwrapManager) Stop() {}

// Stats returns active slot counts.
func (m *BwrapManager) Stats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.sandboxes))
	for k := range m.sandboxes {
		keys = append(keys, k)
	}
	return map[string]any{
		"backend":                   string(BackendBwrap),
		"mode":                      m.config.Mode,
		"active":                    len(m.sandboxes),
		"keys":                      keys,
		"cgroup_limits_via_systemd": m.cgroupViaSystemd,
	}
}
