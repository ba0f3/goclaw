package providers

// cursorStreamEvent is a single line from `agent --output-format stream-json`.
type cursorStreamEvent struct {
	Type      string                 `json:"type"`              // "system", "user", "assistant", "tool_call", "result"
	Subtype   string                 `json:"subtype,omitempty"` // e.g. "init", "started", "completed", "failed"
	SessionID string                 `json:"session_id,omitempty"`
	Message   *cursorStreamMsg       `json:"message,omitempty"`   // for type="assistant"
	CallID    string                 `json:"call_id,omitempty"`   // for type="tool_call"
	ToolCall  map[string]interface{} `json:"tool_call,omitempty"` // for type="tool_call"
	Result    string                 `json:"result,omitempty"`    // for type="result"
	IsError   bool                   `json:"is_error,omitempty"`
	Usage     *cursorUsage           `json:"usage,omitempty"`
}

// cursorStreamMsg wraps content blocks inside an assistant message event.
type cursorStreamMsg struct {
	Content []cursorContentBlock `json:"content"`
}

// cursorContentBlock is a single content block (text).
type cursorContentBlock struct {
	Type string `json:"type"` // "text"
	Text string `json:"text,omitempty"`
}

// cursorUsage maps Cursor CLI usage counters.
type cursorUsage struct {
	InputTokens     int `json:"inputTokens,omitempty"`
	OutputTokens    int `json:"outputTokens,omitempty"`
	CacheReadTokens int `json:"cacheReadTokens,omitempty"`
}
