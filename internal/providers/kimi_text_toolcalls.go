package providers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// Kimi K2.x (Moonshot) may emit Hermes-style tool invocations inside assistant *text*
// when the API does not map them to OpenAI delta.tool_calls. See:
// https://github.com/MoonshotAI/Kimi-K2/blob/main/docs/tool_call_guidance.md
//
// Section: <|redacted_tool_calls_section_begin|> ... <|redacted_tool_calls_section_end|>
// Per call: <|redacted_tool_call_begin_kimi|> functions.NAME:idx <|redacted_tool_call_argument_begin|> {json} <|redacted_tool_call_end_kimi|>
//
// Some endpoints emit shorter tokens (e.g. <|toolcallssectionbegin|>, <|toolcallbegin|>).

var (
	kimiSectionBeginMarkers = []string{
		"<|redacted_tool_calls_section_begin|>",
		"<|toolcallssectionbegin|>",
	}
	kimiSectionEndMarkers = []string{
		"<|redacted_tool_calls_section_end|>",
		"<|toolcallssectionend|>",
	}
	kimiCallBeginMarkers = []string{
		"<|redacted_tool_call_begin_kimi|>",
		"<|toolcallbegin|>",
	}
	kimiArgBeginMarkers = []string{
		"<|redacted_tool_call_argument_begin|>",
		"<|toolcallargumentbegin|>",
	}
	kimiCallEndMarkers = []string{
		"<|redacted_tool_call_end_kimi|>",
		"<|toolcallend|>",
	}
)

// maybeExtractKimiTextToolCalls converts in-text Kimi tool blocks into structured ToolCalls
// and removes them from Content. No-op when API already returned tool_calls or when
// no Kimi-style markers are present.
func maybeExtractKimiTextToolCalls(result *ChatResponse) {
	if result == nil || len(result.ToolCalls) > 0 {
		return
	}
	if !kimiLikelyHasTextToolCalls(result.Content) {
		return
	}
	calls, stripped, ok := parseKimiStyleTextToolCalls(result.Content)
	if !ok || len(calls) == 0 {
		return
	}
	result.ToolCalls = calls
	result.Content = strings.TrimSpace(stripped)
	if result.FinishReason != "length" {
		result.FinishReason = "tool_calls"
	}
	slog.Debug("kimi_text_toolcalls: extracted tool calls from assistant text",
		"calls", len(calls), "content_len", len(result.Content))
}

func kimiLikelyHasTextToolCalls(s string) bool {
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	return strings.Contains(lower, "toolcallssection") ||
		strings.Contains(lower, "redacted_tool_calls_section") ||
		strings.Contains(lower, "redacted_tool_call_begin_kimi") ||
		strings.Contains(lower, "<|toolcallbegin|>") ||
		strings.Contains(lower, "<|toolcallargumentbegin|>")
}

func parseKimiStyleTextToolCalls(content string) (calls []ToolCall, stripped string, ok bool) {
	inner, prefix, suffix, sec := extractKimiToolSection(content)
	scan := inner
	if !sec {
		scan = content
	}

	var out strings.Builder
	remaining := scan
	for {
		cbIdx, cbMark := indexAnyMarker(remaining, kimiCallBeginMarkers)
		if cbIdx < 0 {
			if !sec {
				out.WriteString(remaining)
			}
			break
		}
		if !sec {
			out.WriteString(remaining[:cbIdx])
		}
		tail := remaining[cbIdx+len(cbMark):]
		funcID, afterID := readUntilNextAngleBracket(tail)
		name := kimiFunctionNameFromID(funcID)
		if name == "" {
			remaining = tail
			continue
		}
		name = normalizeKimiToolName(name)

		argIdx, argMark := indexAnyMarker(afterID, kimiArgBeginMarkers)
		if argIdx < 0 {
			if !sec {
				out.WriteString(remaining[cbIdx:])
			}
			remaining = ""
			break
		}
		jsonSrc := strings.TrimLeft(afterID[argIdx+len(argMark):], " \t\n\r")
		args, consumed, err := decodeFirstJSONObject(jsonSrc)
		if err != nil {
			slog.Warn("kimi_text_toolcalls: invalid JSON arguments",
				"tool", name, "error", err)
			remaining = tail
			continue
		}
		toolID := strings.TrimSpace(funcID)
		if toolID == "" {
			toolID = "kimi_text_" + name
		}
		calls = append(calls, ToolCall{
			ID:        toolID,
			Name:      name,
			Arguments: args,
		})
		afterJSON := jsonSrc[consumed:]
		endPos, endMark := indexAnyMarker(afterJSON, kimiCallEndMarkers)
		if endPos >= 0 && endMark != "" {
			remaining = afterJSON[endPos+len(endMark):]
		} else {
			remaining = afterJSON
		}
	}

	if len(calls) == 0 {
		return nil, content, false
	}

	if sec {
		stripped = strings.TrimSpace(prefix + suffix)
	} else {
		stripped = strings.TrimSpace(out.String())
	}
	return calls, stripped, true
}

func extractKimiToolSection(content string) (inner, prefix, suffix string, found bool) {
	bi, bmark := indexAnyMarker(content, kimiSectionBeginMarkers)
	if bi < 0 || bmark == "" {
		return "", "", "", false
	}
	afterOpen := content[bi+len(bmark):]
	ei, emark := indexAnyMarker(afterOpen, kimiSectionEndMarkers)
	if ei < 0 || emark == "" {
		return "", "", "", false
	}
	inner = afterOpen[:ei]
	prefix = content[:bi]
	suffix = afterOpen[ei+len(emark):]
	return inner, prefix, suffix, true
}

func indexAnyMarker(s string, markers []string) (int, string) {
	best := -1
	var bestM string
	for _, m := range markers {
		if m == "" {
			continue
		}
		if i := strings.Index(s, m); i >= 0 && (best < 0 || i < best) {
			best = i
			bestM = m
		}
	}
	return best, bestM
}

func readUntilNextAngleBracket(s string) (token string, rest string) {
	s = strings.TrimLeft(s, " \t\n\r")
	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			return strings.TrimSpace(s[:i]), s[i:]
		}
	}
	return strings.TrimSpace(s), ""
}

// kimiFunctionNameFromID parses "functions.get_weather:0" -> "get_weather".
func kimiFunctionNameFromID(id string) string {
	id = strings.TrimSpace(id)
	if !strings.HasPrefix(id, "functions.") {
		return ""
	}
	rest := strings.TrimPrefix(id, "functions.")
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimSpace(rest)
}

var kimiToolNameAliases = map[string]string{
	"writefile": "write_file",
	"readfile":  "read_file",
	"listfiles": "list_files",
	"editfile":  "edit_file",
	"websearch": "web_search",
	"webfetch":  "web_fetch",
}

func normalizeKimiToolName(name string) string {
	if n, ok := kimiToolNameAliases[strings.ToLower(strings.TrimSpace(name))]; ok {
		return n
	}
	return name
}

// decodeFirstJSONObject extracts the first top-level JSON object from s and unmarshals it.
// It does not use json.Decoder alone: in recent Go versions Decoder may advance past valid JSON
// without surfacing trailing garbage as an error, which would swallow Kimi delimiter text.
func decodeFirstJSONObject(s string) (map[string]any, int, error) {
	s = strings.TrimLeft(s, " \t\n\r")
	if len(s) == 0 || s[0] != '{' {
		return nil, 0, fmt.Errorf("expected JSON object")
	}
	end, err := jsonObjectEndIndex(s)
	if err != nil {
		return nil, 0, err
	}
	objStr := s[:end]
	var raw map[string]any
	if err := json.Unmarshal([]byte(objStr), &raw); err != nil {
		return nil, 0, err
	}
	return raw, end, nil
}

// jsonObjectEndIndex returns the index after the closing '}' of the first top-level JSON object.
func jsonObjectEndIndex(s string) (int, error) {
	depth := 0
	inStr := false
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("unterminated JSON object")
}
