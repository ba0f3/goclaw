package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type sessionNewWire struct {
	Cwd        string            `json:"cwd,omitempty"`
	McpServers []json.RawMessage `json:"mcpServers"`
}

func parseInitializeResponse(raw json.RawMessage) (AgentCaps, error) {
	var w struct {
		AgentCaps  json.RawMessage `json:"agentCapabilities"`
		LegacyCaps json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return AgentCaps{}, err
	}
	capsRaw := w.AgentCaps
	if len(capsRaw) == 0 {
		capsRaw = w.LegacyCaps
	}
	var caps AgentCaps
	if len(capsRaw) > 0 {
		_ = json.Unmarshal(capsRaw, &caps)
	}
	return caps, nil
}

// Initialize sends the ACP initialize request (spec-shaped clientCapabilities).
func (p *ACPProcess) Initialize(ctx context.Context) error {
	initReq := struct {
		ProtocolVersion    int `json:"protocolVersion"`
		ClientCapabilities struct {
			Fs       *FsCaps `json:"fs,omitempty"`
			Terminal bool    `json:"terminal,omitempty"`
		} `json:"clientCapabilities"`
		ClientInfo ClientInfo `json:"clientInfo"`
	}{
		ProtocolVersion: 1,
		ClientCapabilities: struct {
			Fs       *FsCaps `json:"fs,omitempty"`
			Terminal bool    `json:"terminal,omitempty"`
		}{
			Fs:       &FsCaps{ReadTextFile: true, WriteTextFile: true},
			Terminal: true,
		},
		ClientInfo: ClientInfo{Name: "goclaw", Version: "1.0"},
	}
	var raw json.RawMessage
	if err := p.conn.Call(ctx, "initialize", initReq, &raw); err != nil {
		return fmt.Errorf("acp initialize: %w", err)
	}
	caps, err := parseInitializeResponse(raw)
	if err != nil {
		return fmt.Errorf("acp initialize parse: %w", err)
	}
	p.agentCaps = caps
	return nil
}

func (p *ACPProcess) callSessionNewRaw(ctx context.Context, cwd string) (json.RawMessage, error) {
	cwd = strings.TrimSpace(cwd)
	req := sessionNewWire{Cwd: cwd, McpServers: []json.RawMessage{}}
	var raw json.RawMessage
	if err := p.conn.Call(ctx, "session/new", req, &raw); err != nil {
		return nil, fmt.Errorf("acp session/new: %w", err)
	}
	decoded := DecodeSessionNewResult(raw)
	if decoded.SessionID == "" {
		return raw, fmt.Errorf("acp session/new: empty sessionId")
	}
	p.mu.Lock()
	p.sessionID = decoded.SessionID
	p.sessionCwd = cwd
	p.toolRoot = cwd
	p.lastConfigOptions = decoded.ConfigOptions
	p.lastAppliedACPMode = ""
	p.mu.Unlock()
	return raw, nil
}

// NewSession creates a session using the process workdir as cwd (legacy helper).
func (p *ACPProcess) NewSession(ctx context.Context) error {
	cwd := p.fallbackWorkdir
	if cwd == "" {
		cwd = "."
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("acp session/new: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return fmt.Errorf("acp session/new mkdir: %w", err)
	}
	_, err = p.callSessionNewRaw(ctx, abs)
	return err
}

// EnsureSession replaces the session when workspace (cwd) changes.
func (p *ACPProcess) EnsureSession(ctx context.Context, workspace string) error {
	cwd := strings.TrimSpace(workspace)
	if cwd == "" {
		cwd = p.fallbackWorkdir
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("acp workspace: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return fmt.Errorf("acp workspace mkdir: %w", err)
	}

	p.mu.Lock()
	have := p.sessionID != "" && p.sessionCwd == abs
	p.mu.Unlock()
	if have {
		return nil
	}

	_, err = p.callSessionNewRaw(ctx, abs)
	return err
}

// SetConfigOptionRequest updates a session config option (e.g. LLM backend).
type SetConfigOptionRequest struct {
	SessionID string `json:"sessionId"`
	ConfigID  string `json:"configId"`
	Value     string `json:"value"`
}

// SetConfigOptionResponse returns updated config state.
type SetConfigOptionResponse struct {
	ConfigOptions []SessionConfigOption `json:"configOptions"`
}

// SetSessionModeRequest switches operating mode (agent/plan/ask).
type SetSessionModeRequest struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

// ModelConfigOption picks the config option row used for model selection.
func ModelConfigOption(opts []SessionConfigOption) *SessionConfigOption {
	for i := range opts {
		if configOptionLooksLikeModelPicker(opts[i]) {
			return &opts[i]
		}
	}
	return nil
}

// ApplySessionMode calls session/set_mode when a non-empty mode id is set.
func (p *ACPProcess) ApplySessionMode(ctx context.Context, modeID string) error {
	modeID = strings.TrimSpace(modeID)
	if modeID == "" {
		return nil
	}
	p.mu.Lock()
	sid := p.sessionID
	prev := p.lastAppliedACPMode
	p.mu.Unlock()
	if sid == "" || modeID == prev {
		return nil
	}
	var raw json.RawMessage
	err := p.conn.Call(ctx, "session/set_mode", SetSessionModeRequest{
		SessionID: sid,
		ModeID:    modeID,
	}, &raw)
	if err != nil {
		slog.Debug("acp: session/set_mode", "mode_id", modeID, "error", err)
		return nil
	}
	p.mu.Lock()
	p.lastAppliedACPMode = modeID
	p.mu.Unlock()
	return nil
}

// ApplySessionModel calls session/set_config_option when the agent advertised a model selector.
func (p *ACPProcess) ApplySessionModel(ctx context.Context, model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	p.mu.Lock()
	opts := p.lastConfigOptions
	sid := p.sessionID
	p.mu.Unlock()
	if sid == "" {
		return nil
	}
	cfg := ModelConfigOption(opts)
	if cfg == nil {
		return nil
	}
	var raw json.RawMessage
	err := p.conn.Call(ctx, "session/set_config_option", SetConfigOptionRequest{
		SessionID: sid,
		ConfigID:  cfg.ID,
		Value:     model,
	}, &raw)
	if err != nil {
		slog.Debug("acp: session/set_config_option", "config_id", cfg.ID, "error", err)
		return nil
	}
	var out SetConfigOptionResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		slog.Debug("acp: set_config parse", "error", err)
		return nil
	}
	p.mu.Lock()
	p.lastConfigOptions = out.ConfigOptions
	p.mu.Unlock()
	return nil
}

// Prompt sends user content and blocks until the agent responds.
// onUpdate is called for each session/update notification (streaming).
func (p *ACPProcess) Prompt(ctx context.Context, content []ContentBlock, onUpdate func(SessionUpdate)) (*PromptResponse, error) {
	p.inUse.Add(1)
	defer p.inUse.Add(-1)

	p.mu.Lock()
	p.lastActive = time.Now()
	p.mu.Unlock()

	p.setUpdateFn(onUpdate)
	defer p.setUpdateFn(nil)

	req := PromptRequest{
		SessionID: p.sessionID,
		Content:   content,
	}
	var resp PromptResponse
	if err := p.conn.Call(ctx, "session/prompt", req, &resp); err != nil {
		return nil, fmt.Errorf("acp session/prompt: %w", err)
	}

	p.mu.Lock()
	p.lastActive = time.Now()
	p.mu.Unlock()

	return &resp, nil
}

// Cancel sends a session/cancel notification for cooperative cancellation.
func (p *ACPProcess) Cancel() error {
	p.mu.Lock()
	sid := p.sessionID
	p.mu.Unlock()
	if sid == "" {
		return nil
	}
	return p.conn.Notify("session/cancel", CancelNotification{
		SessionID: sid,
	})
}
