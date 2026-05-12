package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MockEngine simulates a one-tool-call response. Picks a tool based on
// keywords in the latest user message so the full pipeline (text stream,
// tool call indicator, second-pass response) runs without an API key.
type MockEngine struct {
	Dispatcher *ToolDispatcher
}

func NewMockEngine(disp *ToolDispatcher) *MockEngine { return &MockEngine{Dispatcher: disp} }

// Stream mirrors Client.StreamConversation's surface so callers can swap.
func (m *MockEngine) Stream(
	ctx context.Context,
	convo []apiMessage,
	emit func(OutEvent),
) ([]contentBlock, error) {
	if len(convo) == 0 {
		return nil, fmt.Errorf("empty conversation")
	}
	last := convo[len(convo)-1]
	userText := ""
	for _, b := range last.Content {
		if b.Type == "text" {
			userText += b.Text
		}
	}
	lower := strings.ToLower(userText)

	// Pick a tool based on keyword hits
	tool, args := mockPickTool(lower)
	emit(OutEvent{Type: "tool_call", ToolName: tool})

	rawArgs, _ := json.Marshal(args)
	result, err := m.Dispatcher.Dispatch(ctx, tool, rawArgs)
	if err != nil {
		emit(OutEvent{Type: "error", Error: err.Error()})
		return nil, err
	}

	// Stream a fake assistant reply that quotes a value from the result
	reply := mockComposeReply(tool, userText, result)
	for _, chunk := range chunkText(reply, 18) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		emit(OutEvent{Type: "text", Text: chunk})
		time.Sleep(30 * time.Millisecond)
	}

	return []contentBlock{{Type: "text", Text: reply}}, nil
}

func mockPickTool(lower string) (string, map[string]any) {
	switch {
	case strings.Contains(lower, "recovery") || strings.Contains(lower, "push") ||
		strings.Contains(lower, "feel") || strings.Contains(lower, "today"):
		return "health_today", map[string]any{}
	case strings.Contains(lower, "budget") || strings.Contains(lower, "spend") ||
		strings.Contains(lower, "month"):
		return "finance_summary", map[string]any{}
	case strings.Contains(lower, "oscp") || strings.Contains(lower, "goal") ||
		strings.Contains(lower, "milestone") || strings.Contains(lower, "track"):
		return "goals_milestones", map[string]any{}
	case strings.Contains(lower, "supplement") || strings.Contains(lower, "peptide") ||
		strings.Contains(lower, "stack"):
		return "health_supplements", map[string]any{"days": 7}
	case strings.Contains(lower, "subscription") || strings.Contains(lower, "recurring"):
		return "finance_subscriptions", map[string]any{}
	case strings.Contains(lower, "brief") || strings.Contains(lower, "today's") ||
		strings.Contains(lower, "tasks"):
		return "goals_brief", map[string]any{}
	default:
		return "health_today", map[string]any{}
	}
}

func mockComposeReply(tool, userText, jsonResult string) string {
	// Very simple — surface a short verdict that proves the tool result threaded through.
	preview := jsonResult
	if len(preview) > 280 {
		preview = preview[:280] + "…"
	}
	return fmt.Sprintf(
		"**[mock mode]** I'd answer this against real data with Claude. Tool I would have called: `%s`.\n\nHere's what the tool returned:\n```\n%s\n```\n\nSet `NORTHSTAR_AI_MODE=anthropic` + `NORTHSTAR_CLAUDE_API_KEY` to get real replies.",
		tool, preview)
}

func chunkText(s string, size int) []string {
	if size <= 0 {
		return []string{s}
	}
	out := []string{}
	runes := []rune(s)
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
	}
	return out
}
