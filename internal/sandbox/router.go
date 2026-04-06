package sandbox

import (
	"context"
	"fmt"
	"log/slog"
)

// SandboxRouter dispatches Get to DockerManager or BwrapManager based on effective config Backend.
type SandboxRouter struct {
	base   Config
	docker *DockerManager
	bwrap  *BwrapManager
}

// NewSandboxRouter returns a Manager that routes per-request backend. Either sub-manager may be nil
// if that backend was unavailable at startup; Get returns an error if the requested backend is nil.
func NewSandboxRouter(base Config, docker *DockerManager, bwrap *BwrapManager) Manager {
	return &SandboxRouter{base: base, docker: docker, bwrap: bwrap}
}

// Get implements Manager.
func (r *SandboxRouter) Get(ctx context.Context, key, workspace string, cfgOverride *Config) (Sandbox, error) {
	cfg := r.base
	if cfgOverride != nil {
		cfg = *cfgOverride
	}
	if cfg.Mode == ModeOff {
		return nil, ErrSandboxDisabled
	}
	switch cfg.Backend {
	case BackendBwrap:
		if r.bwrap == nil {
			return nil, fmt.Errorf("bwrap sandbox unavailable: need %s installed and executable", bwrapBin)
		}
		return r.bwrap.Get(ctx, key, workspace, cfgOverride)
	default:
		if r.docker == nil {
			return nil, fmt.Errorf("docker sandbox unavailable")
		}
		return r.docker.Get(ctx, key, workspace, cfgOverride)
	}
}

// Release destroys a sandbox key on both backends (no-op for missing side).
func (r *SandboxRouter) Release(ctx context.Context, key string) error {
	if r.docker != nil {
		if err := r.docker.Release(ctx, key); err != nil {
			slog.Debug("sandbox.router.docker_release", "key", key, "error", err)
		}
	}
	if r.bwrap != nil {
		if err := r.bwrap.Release(ctx, key); err != nil {
			slog.Debug("sandbox.router.bwrap_release", "key", key, "error", err)
		}
	}
	return nil
}

// ReleaseAll implements Manager.
func (r *SandboxRouter) ReleaseAll(ctx context.Context) error {
	if r.docker != nil {
		_ = r.docker.ReleaseAll(ctx)
	}
	if r.bwrap != nil {
		_ = r.bwrap.ReleaseAll(ctx)
	}
	return nil
}

// Stop implements Manager.
func (r *SandboxRouter) Stop() {
	if r.docker != nil {
		r.docker.Stop()
	}
	if r.bwrap != nil {
		r.bwrap.Stop()
	}
}

// Stats implements Manager.
func (r *SandboxRouter) Stats() map[string]any {
	out := map[string]any{
		"default_backend": string(r.base.Backend),
		"docker_ready":    r.docker != nil,
		"bwrap_ready":     r.bwrap != nil,
	}
	if r.docker != nil {
		out["docker"] = r.docker.Stats()
	}
	if r.bwrap != nil {
		out["bwrap"] = r.bwrap.Stats()
	}
	return out
}
