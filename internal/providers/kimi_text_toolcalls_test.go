package providers

import (
	"testing"
)

func TestParseKimiStyleTextToolCalls_OfficialSection(t *testing.T) {
	content := `Hi.<|redacted_tool_calls_section_begin|>
<|redacted_tool_call_begin_kimi|>functions.get_weather:0<|redacted_tool_call_argument_begin|>{"location":"SF","unit":"celsius"}<|redacted_tool_call_end_kimi|>
<|redacted_tool_calls_section_end|>Thanks.`

	calls, stripped, ok := parseKimiStyleTextToolCalls(content)
	if !ok || len(calls) != 1 {
		t.Fatalf("parse: ok=%v calls=%v", ok, calls)
	}
	if calls[0].Name != "get_weather" || calls[0].ID != "functions.get_weather:0" {
		t.Fatalf("call: %+v", calls[0])
	}
	loc, _ := calls[0].Arguments["location"].(string)
	if loc != "SF" {
		t.Fatalf("location=%q", loc)
	}
	if stripped != "Hi.Thanks." {
		t.Fatalf("stripped=%q", stripped)
	}
}

func TestParseKimiStyleTextToolCalls_ShortTokensWritefileAlias(t *testing.T) {
	content := `Done 📊<|toolcallssectionbegin|><|toolcallbegin|>functions.writefile:21<|toolcallargumentbegin|>{"path":"/tmp/x.md","content":"y"}<|toolcallend|><|toolcallssectionend|>`

	calls, stripped, ok := parseKimiStyleTextToolCalls(content)
	if !ok || len(calls) != 1 {
		t.Fatalf("parse: ok=%v calls=%v", ok, calls)
	}
	if calls[0].Name != "write_file" {
		t.Fatalf("want write_file alias, got %q", calls[0].Name)
	}
	if stripped != "Done 📊" {
		t.Fatalf("stripped=%q", stripped)
	}
}

func TestParseKimiStyleTextToolCalls_NoSectionInline(t *testing.T) {
	content := `Prefix <|toolcallbegin|>functions.read_file:1<|toolcallargumentbegin|>{"path":"a.txt"}<|toolcallend|> suffix`

	calls, stripped, ok := parseKimiStyleTextToolCalls(content)
	if !ok || len(calls) != 1 {
		t.Fatalf("parse: ok=%v calls=%v", ok, calls)
	}
	if calls[0].Name != "read_file" {
		t.Fatalf("name=%q", calls[0].Name)
	}
	// Text before/after the tool block: "Prefix " + " suffix" → two spaces between words.
	if want := "Prefix  suffix"; stripped != want {
		t.Fatalf("stripped=%q want %q", stripped, want)
	}
}

func TestMaybeExtractKimiTextToolCalls_IdempotentWhenToolCallsPresent(t *testing.T) {
	r := &ChatResponse{
		Content:   "x",
		ToolCalls: []ToolCall{{ID: "1", Name: "read_file", Arguments: map[string]any{}}},
	}
	maybeExtractKimiTextToolCalls(r)
	if len(r.ToolCalls) != 1 {
		t.Fatalf("unexpected merge")
	}
}

func TestMaybeExtractKimiTextToolCalls_ExtractsFromResponse(t *testing.T) {
	r := &ChatResponse{
		Content: `<|toolcallssectionbegin|><|toolcallbegin|>functions.web_search:0<|toolcallargumentbegin|>{"query":"x"}<|toolcallend|><|toolcallssectionend|>`,
	}
	maybeExtractKimiTextToolCalls(r)
	if len(r.ToolCalls) != 1 || r.ToolCalls[0].Name != "web_search" {
		t.Fatalf("got %+v", r.ToolCalls)
	}
	if r.Content != "" {
		t.Fatalf("content=%q", r.Content)
	}
	if r.FinishReason != "tool_calls" {
		t.Fatalf("finish=%q", r.FinishReason)
	}
}

func TestMaybeExtractKimiTextToolCalls_FromThinkingOnly(t *testing.T) {
	// Kimi K2.x often puts <|toolcall...|> in reasoning_content while content is user-facing prose.
	r := &ChatResponse{
		Content:  "Giờ em tạo PPTX theo chuẩn 📊",
		Thinking: `<|toolcallssectionbegin|><|toolcallbegin|>functions.write_file:1<|toolcallargumentbegin|>{"path":"x.md","content":"y"}<|toolcallend|><|toolcallssectionend|>`,
	}
	maybeExtractKimiTextToolCalls(r)
	if len(r.ToolCalls) != 1 || r.ToolCalls[0].Name != "write_file" {
		t.Fatalf("got %+v", r.ToolCalls)
	}
	if r.Content != "Giờ em tạo PPTX theo chuẩn 📊" {
		t.Fatalf("content changed: %q", r.Content)
	}
	if r.Thinking != "" {
		t.Fatalf("thinking=%q want empty", r.Thinking)
	}
	if r.FinishReason != "tool_calls" {
		t.Fatalf("finish=%q", r.FinishReason)
	}
}

func TestDecodeFirstJSONObject_TrailingGarbage(t *testing.T) {
	s := `{"path":"a.txt"}<|toolcallend|> suffix`
	m, n, err := decodeFirstJSONObject(s)
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := m["path"].(string); p != "a.txt" {
		t.Fatalf("path=%v", m)
	}
	wantN := len(`{"path":"a.txt"}`)
	if n != wantN || s[n:] != `<|toolcallend|> suffix` {
		t.Fatalf("n=%d want %d tail=%q", n, wantN, s[n:])
	}
}

func TestKimiFunctionNameFromID(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"functions.get_weather:0", "get_weather"},
		{"functions.writefile:21", "writefile"},
		{"bad", ""},
	}
	for _, tt := range tests {
		if got := kimiFunctionNameFromID(tt.id); got != tt.want {
			t.Errorf("%q: got %q want %q", tt.id, got, tt.want)
		}
	}
}

