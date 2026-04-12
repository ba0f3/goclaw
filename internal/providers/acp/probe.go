package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ProbeSessionModels runs initialize + session/new and returns LLM selector rows
// from configOptions. Session modes are separate — use ModelChoicesFromModesRaw on raw.
func ProbeSessionModels(ctx context.Context, binary string, extraArgs []string, cwd string) ([]DiscoveredModel, error) {
	models, _, err := ProbeSessionModelsWithRaw(ctx, binary, extraArgs, cwd)
	return models, err
}

// ProbeSessionModelsWithRaw is like ProbeSessionModels but returns raw session/new result JSON.
func ProbeSessionModelsWithRaw(ctx context.Context, binary string, extraArgs []string, cwd string) ([]DiscoveredModel, json.RawMessage, error) {
	if cwd == "" {
		return nil, nil, fmt.Errorf("acp probe: cwd required")
	}
	if !BinaryPathAllowed(binary) {
		return nil, nil, fmt.Errorf("acp probe: invalid binary %q", binary)
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, nil, fmt.Errorf("acp probe: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, nil, fmt.Errorf("acp probe mkdir: %w", err)
	}

	procCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	args := append([]string(nil), extraArgs...)
	cmd := exec.CommandContext(procCtx, binary, args...)
	cmd.Dir = abs
	cmd.Env = filterACPEnv(os.Environ())

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderrCap := &limitedWriter{max: 8192}
	cmd.Stderr = stderrCap

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("acp probe start: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	handler := func(_ context.Context, method string, _ json.RawMessage) (any, error) {
		return nil, fmt.Errorf("unexpected agent request during probe: %s", method)
	}
	notify := func(string, json.RawMessage) {}
	conn := NewConn(stdinPipe, stdoutPipe, handler, notify)
	conn.Start()

	proc := &ACPProcess{conn: conn}

	if err := proc.Initialize(ctx); err != nil {
		return nil, nil, err
	}
	raw, err := proc.callSessionNewRaw(ctx, abs)
	if err != nil {
		return nil, nil, fmt.Errorf("acp session/new: %w", err)
	}
	decoded := DecodeSessionNewResult(raw)
	fromConfig := ModelChoicesFromConfigOptions(decoded.ConfigOptions)
	fromModels := ModelChoicesFromSessionModelsJSON(raw)
	models := MergeDiscoveredModels(fromConfig, fromModels)
	if len(models) == 0 {
		args := []any{
			"binary", binary,
			"configOptions", len(decoded.ConfigOptions),
		}
		if tail := strings.TrimSpace(stderrCap.String()); tail != "" {
			args = append(args, "stderr_tail", tail)
		}
		slog.Warn("acp.probe: no LLM models parsed from session/new configOptions; UI will use static ACP fallback list",
			args...)
	}
	return models, raw, nil
}

// ProbeSessionModelsTimeout wraps ProbeSessionModels with a deadline.
func ProbeSessionModelsTimeout(parent context.Context, binary string, extraArgs []string, cwd string, timeout time.Duration) ([]DiscoveredModel, error) {
	if timeout <= 0 {
		timeout = 25 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return ProbeSessionModels(ctx, binary, extraArgs, cwd)
}
