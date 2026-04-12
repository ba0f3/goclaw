package acp

import (
	"encoding/json"
	"testing"
)

func TestSessionConfigOption_UnmarshalJSON_snakeAndLabels(t *testing.T) {
	const raw = `{
		"id": "model",
		"label": "Model",
		"category": "model",
		"type": "select",
		"current_value": "gpt-4o",
		"options": [
			{"id": "gpt-4o", "label": "GPT-4o"},
			{"value": "opus", "name": "Opus"}
		]
	}`
	var o SessionConfigOption
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
	if o.Name != "Model" {
		t.Fatalf("name: got %q", o.Name)
	}
	if o.CurrentValue != "gpt-4o" {
		t.Fatalf("currentValue: got %q", o.CurrentValue)
	}
	if len(o.Options) != 2 {
		t.Fatalf("options len: %d", len(o.Options))
	}
	if o.Options[0].Value != "gpt-4o" || o.Options[0].Name != "GPT-4o" {
		t.Fatalf("opt0: %+v", o.Options[0])
	}
	models := ModelChoicesFromConfigOptions([]SessionConfigOption{o})
	if len(models) != 2 {
		t.Fatalf("models: %+v", models)
	}
}

func TestModelChoicesFromConfigOptions_excludesModeSelect(t *testing.T) {
	opts := []SessionConfigOption{
		{
			ID:       "mode",
			Category: "mode",
			Type:     "select",
			Options: []SessionConfigOptionValue{
				{Value: "agent", Name: "Agent"},
				{Value: "plan", Name: "Plan"},
			},
		},
		{
			ID:       "model",
			Category: "model",
			Type:     "select",
			Options: []SessionConfigOptionValue{
				{Value: "claude-sonnet", Name: "Sonnet"},
			},
		},
	}
	models := ModelChoicesFromConfigOptions(opts)
	if len(models) != 1 || models[0].ID != "claude-sonnet" {
		t.Fatalf("got %+v", models)
	}
}

func TestModelChoicesFromSessionModelsJSON_geminiShape(t *testing.T) {
	const raw = `{
		"sessionId": "s1",
		"models": {
			"availableModels": [
				{"modelId": "gemini-2.5-flash", "name": "Gemini 2.5 Flash"},
				{"modelId": "gemini-2.5-pro", "name": "Gemini 2.5 Pro"}
			],
			"currentModelId": "gemini-2.5-flash"
		}
	}`
	got := ModelChoicesFromSessionModelsJSON([]byte(raw))
	if len(got) != 2 || got[0].ID != "gemini-2.5-flash" || got[1].ID != "gemini-2.5-pro" {
		t.Fatalf("got %+v", got)
	}
}

func TestMergeDiscoveredModels_dedup(t *testing.T) {
	a := []DiscoveredModel{{ID: "x", Name: "X"}, {ID: "y", Name: "Y"}}
	b := []DiscoveredModel{{ID: "x", Name: "Dup"}, {ID: "z", Name: "Z"}}
	got := MergeDiscoveredModels(a, b)
	if len(got) != 3 || got[2].ID != "z" {
		t.Fatalf("got %+v", got)
	}
}

func TestModelChoicesFromConfigOptions_stringOptions(t *testing.T) {
	raw := `{
		"id": "model",
		"name": "Model",
		"category": "model",
		"type": "select",
		"options": ["a", "b"]
	}`
	var o SessionConfigOption
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
	models := ModelChoicesFromConfigOptions([]SessionConfigOption{o})
	if len(models) != 2 {
		t.Fatalf("got %+v", models)
	}
}
