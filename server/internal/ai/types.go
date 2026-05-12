package ai

import "encoding/json"

// ─── Wire types for the Anthropic Messages API ────────────────────────────

type messageRequest struct {
	Model     string         `json:"model"`
	MaxTokens int            `json:"max_tokens"`
	Stream    bool           `json:"stream"`
	System    []systemBlock  `json:"system,omitempty"`
	Tools     []toolDef      `json:"tools,omitempty"`
	Messages  []apiMessage   `json:"messages"`
}

type systemBlock struct {
	Type         string        `json:"type"` // "text"
	Text         string        `json:"text"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type cacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type toolDef struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	CacheControl *cacheControl   `json:"cache_control,omitempty"`
}

type apiMessage struct {
	Role    string         `json:"role"` // user | assistant
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type       string          `json:"type"` // text | tool_use | tool_result
	Text       string          `json:"text,omitempty"`
	ID         string          `json:"id,omitempty"`           // for tool_use
	Name       string          `json:"name,omitempty"`         // for tool_use
	Input      json.RawMessage `json:"input,omitempty"`        // for tool_use
	ToolUseID  string          `json:"tool_use_id,omitempty"`  // for tool_result
	ToolResult string          `json:"content,omitempty"`      // for tool_result (stringified)
	IsError    bool            `json:"is_error,omitempty"`     // for tool_result
}

// ─── Streaming events ─────────────────────────────────────────────────────

type streamEvent struct {
	Type         string           `json:"type"`
	Index        int              `json:"index"`
	Delta        streamDelta      `json:"delta"`
	ContentBlock streamBlockStart `json:"content_block"`
	Message      streamMessage    `json:"message"`
	Usage        *Usage           `json:"usage,omitempty"` // message_delta carries cumulative usage
}

type streamMessage struct {
	StopReason string `json:"stop_reason,omitempty"`
	Usage      *Usage `json:"usage,omitempty"` // message_start carries input/cache_* totals
}

type streamDelta struct {
	Type        string `json:"type"` // text_delta | input_json_delta | message_delta
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// Usage mirrors Anthropic's streaming usage payload. Field names match the
// wire format so the same struct decodes from message_start and message_delta.
type Usage struct {
	InputTokens              int `json:"input_tokens,omitempty"`
	OutputTokens             int `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// Add accumulates one turn's usage into the running total. Cache reads /
// creation are write-once per turn (input side), output is the final per-turn
// total — so summing across turns yields a meaningful conversation total.
func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheCreationInputTokens += other.CacheCreationInputTokens
	u.CacheReadInputTokens += other.CacheReadInputTokens
}

type streamBlockStart struct {
	Type  string          `json:"type"`         // text | tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// ─── Northstar's stream-to-iOS event shape ────────────────────────────────
//
// The server re-encodes the Anthropic stream into our own event protocol so
// the iOS client doesn't have to track every Anthropic event type. iOS gets:
//   - "text"      → token of model text
//   - "tool_call" → server is running a tool (so the UI can show a "Reading
//                   recovery..." pill)
//   - "done"      → conversation turn complete
//   - "error"     → something failed; payload has message

type OutEvent struct {
	Type     string `json:"type"`             // text | tool_call | done | error
	Text     string `json:"text,omitempty"`
	ToolName string `json:"tool_name,omitempty"`
	Error    string `json:"error,omitempty"`
}
