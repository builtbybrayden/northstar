package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultModel      = "claude-sonnet-4-6"
	defaultMaxTokens  = 2048
	defaultBaseURL    = "https://api.anthropic.com"
	apiVersion        = "2023-06-01"
	streamReadTimeout = 5 * time.Minute
)

// Client wraps the Anthropic Messages API for streaming chat with tools.
type Client struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = defaultModel
	}
	return &Client{
		APIKey:  apiKey,
		BaseURL: defaultBaseURL,
		Model:   model,
		HTTP:    &http.Client{Timeout: streamReadTimeout},
	}
}

// StreamConversation runs a full tool-use loop against Anthropic. It calls
// `emit` for each token / tool_call / done event for the consumer (the
// SSE handler) to forward. Returns the final assistant content blocks plus
// the aggregated token usage across every turn in the loop so the caller can
// persist them.
func (c *Client) StreamConversation(
	ctx context.Context,
	system []systemBlock,
	tools []toolDef,
	convo []apiMessage,
	executeTool func(ctx context.Context, name string, input json.RawMessage) (string, error),
	emit func(OutEvent),
) ([]contentBlock, Usage, error) {
	var total Usage
	current := convo
	for turn := 0; turn < 8; turn++ {
		assistantBlocks, stop, usage, err := c.streamOneTurn(ctx, system, tools, current, emit)
		if err != nil {
			return nil, total, err
		}
		total.Add(usage)

		// Persist assistant's response into the conversation we'll feed next turn
		current = append(current, apiMessage{Role: "assistant", Content: assistantBlocks})

		if stop != "tool_use" {
			return assistantBlocks, total, nil
		}

		// Execute every tool_use block and feed results back as a user message
		var toolResults []contentBlock
		for _, b := range assistantBlocks {
			if b.Type != "tool_use" {
				continue
			}
			emit(OutEvent{Type: "tool_call", ToolName: b.Name})
			result, err := executeTool(ctx, b.Name, b.Input)
			tr := contentBlock{
				Type:       "tool_result",
				ToolUseID:  b.ID,
				ToolResult: result,
			}
			if err != nil {
				tr.ToolResult = fmt.Sprintf(`{"error": %q}`, err.Error())
				tr.IsError = true
				// Surface to iOS so the user sees the failure inline instead of
				// just watching the assistant silently keep talking.
				emit(OutEvent{Type: "tool_error", ToolName: b.Name, Error: err.Error()})
			}
			toolResults = append(toolResults, tr)
		}
		current = append(current, apiMessage{Role: "user", Content: toolResults})
	}
	return nil, total, errors.New("tool-use loop exceeded 8 turns")
}

// streamOneTurn sends one request and consumes the SSE response, accumulating
// content blocks (text + tool_use). Returns the blocks, stop_reason, and the
// token-usage totals for this turn.
func (c *Client) streamOneTurn(
	ctx context.Context,
	system []systemBlock,
	tools []toolDef,
	messages []apiMessage,
	emit func(OutEvent),
) ([]contentBlock, string, Usage, error) {
	body := messageRequest{
		Model:     c.Model,
		MaxTokens: defaultMaxTokens,
		Stream:    true,
		System:    system,
		Tools:     tools,
		Messages:  messages,
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, "", Usage{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return nil, "", Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", Usage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", Usage{}, fmt.Errorf("anthropic %d: %s", resp.StatusCode, string(body))
	}

	// Accumulators per index
	type acc struct {
		Type      string
		Text      strings.Builder
		ToolID    string
		ToolName  string
		ToolInput strings.Builder
	}
	blocks := map[int]*acc{}
	var stopReason string
	var usage Usage

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		raw := strings.TrimPrefix(line, "data: ")
		if raw == "[DONE]" {
			break
		}
		var ev streamEvent
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "message_start":
			if ev.Message.Usage != nil {
				// message_start carries input + cache_* totals (output starts at 1)
				usage.InputTokens = ev.Message.Usage.InputTokens
				usage.CacheCreationInputTokens = ev.Message.Usage.CacheCreationInputTokens
				usage.CacheReadInputTokens = ev.Message.Usage.CacheReadInputTokens
				usage.OutputTokens = ev.Message.Usage.OutputTokens
			}

		case "content_block_start":
			a := &acc{Type: ev.ContentBlock.Type}
			if ev.ContentBlock.Type == "tool_use" {
				a.ToolID = ev.ContentBlock.ID
				a.ToolName = ev.ContentBlock.Name
			}
			blocks[ev.Index] = a

		case "content_block_delta":
			a, ok := blocks[ev.Index]
			if !ok {
				continue
			}
			switch ev.Delta.Type {
			case "text_delta":
				a.Text.WriteString(ev.Delta.Text)
				emit(OutEvent{Type: "text", Text: ev.Delta.Text})
			case "input_json_delta":
				a.ToolInput.WriteString(ev.Delta.PartialJSON)
			}

		case "message_delta":
			if ev.Delta.StopReason != "" {
				stopReason = ev.Delta.StopReason
			}
			if ev.Usage != nil && ev.Usage.OutputTokens > 0 {
				// message_delta carries cumulative output_tokens — latest wins.
				usage.OutputTokens = ev.Usage.OutputTokens
			}

		case "message_stop":
			if ev.Message.StopReason != "" {
				stopReason = ev.Message.StopReason
			}

		case "error":
			// Anthropic error message — surface and bail
			return nil, "", Usage{}, fmt.Errorf("anthropic stream error: %s", raw)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, "", Usage{}, fmt.Errorf("stream read: %w", err)
	}

	// Order blocks by index
	maxIdx := -1
	for i := range blocks {
		if i > maxIdx {
			maxIdx = i
		}
	}
	out := make([]contentBlock, 0, maxIdx+1)
	for i := 0; i <= maxIdx; i++ {
		a, ok := blocks[i]
		if !ok {
			continue
		}
		switch a.Type {
		case "text":
			out = append(out, contentBlock{Type: "text", Text: a.Text.String()})
		case "tool_use":
			input := a.ToolInput.String()
			if input == "" {
				input = "{}"
			}
			out = append(out, contentBlock{
				Type:  "tool_use",
				ID:    a.ToolID,
				Name:  a.ToolName,
				Input: json.RawMessage(input),
			})
		}
	}
	return out, stopReason, usage, nil
}
