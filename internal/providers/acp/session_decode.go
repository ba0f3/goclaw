package acp

import (
	"encoding/json"
	"strings"
)

// SessionConfigOption is one row from session/new configOptions (model picker, etc.).
type SessionConfigOption struct {
	ID           string                     `json:"id"`
	Name         string                     `json:"name,omitempty"`
	Description  string                     `json:"description,omitempty"`
	Category     string                     `json:"category,omitempty"`
	Type         string                     `json:"type,omitempty"`
	CurrentValue string                     `json:"currentValue,omitempty"`
	Options      []SessionConfigOptionValue `json:"options,omitempty"`
}

// UnmarshalJSON accepts camelCase and snake_case fields and Cursor/Zed label/value variants.
func (o *SessionConfigOption) UnmarshalJSON(data []byte) error {
	var w struct {
		ID                string            `json:"id"`
		Name              string            `json:"name"`
		Label             string            `json:"label"`
		Description       string            `json:"description"`
		Category          string            `json:"category"`
		Type              string            `json:"type"`
		CurrentValue      string            `json:"currentValue"`
		CurrentValueSnake string            `json:"current_value"`
		Options           []json.RawMessage `json:"options"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	o.ID = w.ID
	o.Name = w.Name
	if o.Name == "" {
		o.Name = w.Label
	}
	o.Description = w.Description
	o.Category = w.Category
	o.Type = w.Type
	o.CurrentValue = w.CurrentValue
	if o.CurrentValue == "" {
		o.CurrentValue = w.CurrentValueSnake
	}
	o.Options = nil
	for _, raw := range w.Options {
		if len(raw) == 0 {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && strings.TrimSpace(s) != "" {
			s = strings.TrimSpace(s)
			o.Options = append(o.Options, SessionConfigOptionValue{Value: s, Name: s})
			continue
		}
		var num json.Number
		if err := json.Unmarshal(raw, &num); err == nil && num != "" {
			ns := num.String()
			o.Options = append(o.Options, SessionConfigOptionValue{Value: ns, Name: ns})
			continue
		}
		var v SessionConfigOptionValue
		if err := json.Unmarshal(raw, &v); err != nil {
			continue
		}
		if strings.TrimSpace(v.Value) != "" || strings.TrimSpace(v.Name) != "" {
			o.Options = append(o.Options, v)
		}
	}
	return nil
}

// SessionConfigOptionValue is one allowed value for a SessionConfigOption.
type SessionConfigOptionValue struct {
	Value       string `json:"value"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// UnmarshalJSON maps id/label into value/name when the wire format omits value/name.
func (v *SessionConfigOptionValue) UnmarshalJSON(data []byte) error {
	var w struct {
		Value       string `json:"value"`
		ID          string `json:"id"`
		Name        string `json:"name"`
		Label       string `json:"label"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	v.Value = strings.TrimSpace(w.Value)
	if v.Value == "" {
		v.Value = strings.TrimSpace(w.ID)
	}
	v.Name = strings.TrimSpace(w.Name)
	if v.Name == "" {
		v.Name = strings.TrimSpace(w.Label)
	}
	v.Description = w.Description
	return nil
}

// DecodedSessionNew is the parsed session/new result (beyond sessionId).
type DecodedSessionNew struct {
	SessionID     string
	ConfigOptions []SessionConfigOption
	ModesRaw      json.RawMessage
}

// DecodeSessionNewResult parses session/new JSON-RPC result with key fallbacks.
func DecodeSessionNewResult(data []byte) DecodedSessionNew {
	var out DecodedSessionNew
	if len(data) == 0 {
		return out
	}
	var top struct {
		SessionID       string                 `json:"sessionId"`
		SessionIDSnake  string                 `json:"session_id"`
		ConfigOptions   []SessionConfigOption  `json:"configOptions"`
		ConfigOptsSnake []SessionConfigOption  `json:"config_options"`
		Modes           json.RawMessage        `json:"modes"`
	}
	if err := json.Unmarshal(data, &top); err != nil {
		return out
	}
	out.SessionID = top.SessionID
	if out.SessionID == "" {
		out.SessionID = top.SessionIDSnake
	}
	if len(top.ConfigOptions) > 0 {
		out.ConfigOptions = top.ConfigOptions
	} else {
		out.ConfigOptions = top.ConfigOptsSnake
	}
	out.ModesRaw = top.Modes
	if len(out.ConfigOptions) > 0 {
		return out
	}
	var bag map[string]json.RawMessage
	if err := json.Unmarshal(data, &bag); err != nil {
		return out
	}
	for _, key := range []string{
		"config_options", "sessionConfigOptions", "configurationOptions",
		"configuration_options", "settings",
	} {
		raw, ok := bag[key]
		if !ok {
			continue
		}
		var opts []SessionConfigOption
		if err := json.Unmarshal(raw, &opts); err == nil && len(opts) > 0 {
			out.ConfigOptions = opts
			return out
		}
	}
	return out
}

// DiscoveredModel is a normalized id+label for UI lists.
type DiscoveredModel struct {
	ID   string
	Name string
}

// MergeDiscoveredModels returns a then b, skipping duplicate IDs (a wins).
func MergeDiscoveredModels(a, b []DiscoveredModel) []DiscoveredModel {
	seen := make(map[string]struct{})
	out := make([]DiscoveredModel, 0, len(a)+len(b))
	for _, m := range a {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = id
		}
		out = append(out, DiscoveredModel{ID: id, Name: name})
	}
	for _, m := range b {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = id
		}
		out = append(out, DiscoveredModel{ID: id, Name: name})
	}
	return out
}

// sessionModelWire matches Gemini CLI and similar agents (not ACP configOptions).
type sessionModelWire struct {
	ModelID      string `json:"modelId"`
	ModelIDSnake string `json:"model_id"`
	ID           string `json:"id"`
	Value        string `json:"value"`
	Name         string `json:"name"`
	Title        string `json:"title"`
}

// ModelChoicesFromSessionModelsJSON parses top-level "models" on session/new result.
// Gemini CLI returns { models: { availableModels: [{ modelId, name }], currentModelId } }.
func ModelChoicesFromSessionModelsJSON(data []byte) []DiscoveredModel {
	if len(data) == 0 {
		return nil
	}
	var top struct {
		Models json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(data, &top); err != nil {
		return nil
	}
	if len(top.Models) == 0 {
		var nest struct {
			Session struct {
				Models json.RawMessage `json:"models"`
			} `json:"session"`
		}
		if err := json.Unmarshal(data, &nest); err == nil && len(nest.Session.Models) > 0 {
			top.Models = nest.Session.Models
		}
	}
	return parseModelsAvailableList(top.Models)
}

func parseModelsAvailableList(modelsJSON json.RawMessage) []DiscoveredModel {
	if len(modelsJSON) == 0 {
		return nil
	}
	var w struct {
		Available      []sessionModelWire `json:"availableModels"`
		AvailableSnake []sessionModelWire `json:"available_models"`
	}
	if err := json.Unmarshal(modelsJSON, &w); err != nil {
		return nil
	}
	arr := w.Available
	if len(arr) == 0 {
		arr = w.AvailableSnake
	}
	seen := make(map[string]struct{})
	out := make([]DiscoveredModel, 0, len(arr))
	for _, m := range arr {
		id := strings.TrimSpace(m.ModelID)
		if id == "" {
			id = strings.TrimSpace(m.ModelIDSnake)
		}
		if id == "" {
			id = strings.TrimSpace(m.ID)
		}
		if id == "" {
			id = strings.TrimSpace(m.Value)
		}
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = strings.TrimSpace(m.Title)
		}
		if name == "" {
			name = id
		}
		out = append(out, DiscoveredModel{ID: id, Name: name})
	}
	return out
}

// ModelChoicesFromConfigOptions extracts LLM / backend selector values from configOptions.
func ModelChoicesFromConfigOptions(opts []SessionConfigOption) []DiscoveredModel {
	if len(opts) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	var out []DiscoveredModel
	for _, o := range opts {
		if !configOptionLooksLikeModelPicker(o) {
			continue
		}
		for _, v := range o.Options {
			id := strings.TrimSpace(v.Value)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			name := strings.TrimSpace(v.Name)
			if name == "" {
				name = id
			}
			out = append(out, DiscoveredModel{ID: id, Name: name})
		}
		if len(o.Options) == 0 && strings.TrimSpace(o.CurrentValue) != "" {
			id := strings.TrimSpace(o.CurrentValue)
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				name := strings.TrimSpace(o.Name)
				if name == "" {
					name = id
				}
				out = append(out, DiscoveredModel{ID: id, Name: name})
			}
		}
	}
	return out
}

// isModeConfigOption detects session mode selectors (agent/plan/ask) — not LLM model lists.
func isModeConfigOption(o SessionConfigOption) bool {
	cat := strings.ToLower(strings.TrimSpace(o.Category))
	id := strings.ToLower(strings.TrimSpace(o.ID))
	return cat == "mode" || id == "mode"
}

func configOptionLooksLikeModelPicker(o SessionConfigOption) bool {
	if isModeConfigOption(o) {
		return false
	}
	cat := strings.ToLower(o.Category)
	typ := strings.ToLower(o.Type)
	name := strings.ToLower(o.Name)
	desc := strings.ToLower(o.Description)
	id := strings.ToLower(strings.TrimSpace(o.ID))
	switch {
	case strings.Contains(cat, "model"):
		return true
	case id == "model" || strings.HasPrefix(id, "model_") || strings.HasSuffix(id, "_model"):
		return true
	case strings.Contains(name, "model"):
		return true
	case strings.Contains(desc, "model"):
		return true
	case typ == "enum" || typ == "select" || typ == "dropdown":
		return len(o.Options) > 0
	default:
		return len(o.Options) > 1
	}
}

// ModelChoicesFromModesRaw parses modes.availableModes for session/set_mode UI.
func ModelChoicesFromModesRaw(raw json.RawMessage) []DiscoveredModel {
	c, _ := parseModesWire(raw)
	return c
}

func parseModesWire(raw json.RawMessage) ([]DiscoveredModel, []string) {
	if len(raw) == 0 {
		return nil, nil
	}
	var w struct {
		Available      []modeWire `json:"availableModes"`
		AvailableSnake []modeWire `json:"available_modes"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, nil
	}
	arr := w.Available
	if len(arr) == 0 {
		arr = w.AvailableSnake
	}
	out := make([]DiscoveredModel, 0, len(arr))
	ids := make([]string, 0, len(arr))
	for _, m := range arr {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = id
		}
		out = append(out, DiscoveredModel{ID: id, Name: name})
		ids = append(ids, id)
	}
	return out, ids
}

type modeWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UnmarshalJSON accepts configOptions and config_options.
func (r *SetConfigOptionResponse) UnmarshalJSON(data []byte) error {
	var w struct {
		ConfigOptions   []SessionConfigOption `json:"configOptions"`
		ConfigOptsSnake []SessionConfigOption `json:"config_options"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	if len(w.ConfigOptions) > 0 {
		r.ConfigOptions = w.ConfigOptions
	} else {
		r.ConfigOptions = w.ConfigOptsSnake
	}
	return nil
}
