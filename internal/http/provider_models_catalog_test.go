package http

import "testing"

func TestParseCursorCLIModelsOutputPlain(t *testing.T) {
	out := []byte(`Available models

auto - Auto (current)
composer-2-fast - Composer 2 Fast (default)
gpt-5.3-codex - Codex 5.3

Tip: use --model <id>`)

	models := parseCursorCLIModelsOutput(out)
	if len(models) != 3 {
		t.Fatalf("models len = %d, want 3", len(models))
	}
	if models[0].ID != "auto" || models[0].Name != "Auto (current)" {
		t.Fatalf("models[0] = %#v, want auto / Auto (current)", models[0])
	}
	if models[2].ID != "gpt-5.3-codex" || models[2].Name != "Codex 5.3" {
		t.Fatalf("models[2] = %#v, want gpt-5.3-codex / Codex 5.3", models[2])
	}
}

func TestParseCursorCLIModelsOutputANSI(t *testing.T) {
	out := []byte("\x1b[1mAvailable models\x1b[0m\n\n" +
		"\x1b[32mauto\x1b[0m - Auto (current)\n" +
		"\x1b[36mgpt-5.4-high\x1b[0m - \x1b[1mGPT-5.4 1M High\x1b[0m\n" +
		"\x1b]8;;https://example.com\x07ignored-link\x1b]8;;\x07\n" +
		"Tip: use --model <id>\n")

	models := parseCursorCLIModelsOutput(out)
	if len(models) != 2 {
		t.Fatalf("models len = %d, want 2", len(models))
	}
	if models[1].ID != "gpt-5.4-high" || models[1].Name != "GPT-5.4 1M High" {
		t.Fatalf("models[1] = %#v, want gpt-5.4-high / GPT-5.4 1M High", models[1])
	}
}

func TestParseCursorCLIModelsOutputSkipsInvalidAndDuplicateLines(t *testing.T) {
	out := []byte(`Available models
auto - Auto (current)
invalid line without separator
auto - Auto (current)
gpt-5.4-high - GPT-5.4 1M High
`)

	models := parseCursorCLIModelsOutput(out)
	if len(models) != 2 {
		t.Fatalf("models len = %d, want 2", len(models))
	}
	if models[0].ID != "auto" || models[1].ID != "gpt-5.4-high" {
		t.Fatalf("models = %#v, want auto then gpt-5.4-high", models)
	}
}
