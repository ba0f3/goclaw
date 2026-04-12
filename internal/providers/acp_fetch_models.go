package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/providers/acp"
)

// ACPModelChoice is one selectable row for the web UI (LLM backend or label).
type ACPModelChoice struct {
	ID   string
	Name string
}

// ACPProbeResult holds configOptions-derived models and modes.availableModes.
type ACPProbeResult struct {
	Models []ACPModelChoice
	Modes  []ACPModelChoice
}

// FetchACPProbeViaAgent runs initialize + session/new against the configured binary.
func FetchACPProbeViaAgent(ctx context.Context, binary string, settingsJSON []byte, probeCwd string, timeout time.Duration) (ACPProbeResult, error) {
	var empty ACPProbeResult
	if binary == "" {
		return empty, fmt.Errorf("acp: empty binary")
	}
	if !acp.BinaryPathAllowed(binary) {
		return empty, fmt.Errorf("acp: invalid binary")
	}
	if probeCwd == "" {
		return empty, fmt.Errorf("acp: empty probe cwd")
	}
	var settings struct {
		Args []string `json:"args"`
	}
	if len(settingsJSON) > 0 {
		_ = json.Unmarshal(settingsJSON, &settings)
	}
	spawnArgs := EffectiveACPArgs(binary, settings.Args)

	probeCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		probeCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	discovered, raw, err := acp.ProbeSessionModelsWithRaw(probeCtx, binary, spawnArgs, probeCwd)
	if err != nil {
		return empty, err
	}
	decoded := acp.DecodeSessionNewResult(raw)
	modeDisc := acp.ModelChoicesFromModesRaw(decoded.ModesRaw)

	out := ACPProbeResult{
		Models: make([]ACPModelChoice, 0, len(discovered)),
		Modes:  make([]ACPModelChoice, 0, len(modeDisc)),
	}
	for _, d := range discovered {
		out.Models = append(out.Models, ACPModelChoice{ID: d.ID, Name: d.Name})
	}
	for _, m := range modeDisc {
		out.Modes = append(out.Modes, ACPModelChoice{ID: m.ID, Name: m.Name})
	}
	return out, nil
}
