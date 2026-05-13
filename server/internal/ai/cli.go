package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// CLIEngine shells out to a locally installed `claude` CLI to generate
// replies, so Northstar doesn't need an Anthropic API key directly.
//
// Trade-offs vs the API-key Client:
//   - No live tool-use loop. We instead pre-fetch a snapshot of each pillar's
//     state via the same ToolDispatcher the Mock and Client engines use, and
//     inline it into the system prompt. The model sees a frozen view of
//     pillar data at request time rather than calling tools mid-response.
//     For a single-operator personal-life dashboard, this is fine; the data
//     is small (a few KB) and the user is asking ad-hoc questions, not
//     building long autonomous workflows.
//   - No real-time streaming. `claude --print` returns the full reply at
//     once; we fake-stream by chunking the output so the iOS SSE consumer
//     doesn't need a separate code path.
//   - No token-usage telemetry from the CLI surface, so `usage_json` stays
//     NULL for CLI-mode messages.
//
// The CLI invocation is `exec.Command`, NOT a shell — user text is passed
// via stdin, never interpolated into a shell string.
type CLIEngine struct {
	Dispatcher *ToolDispatcher
	Binary     string
	Model      string
	// ExtraArgs lets ops slot in additional CLI flags (e.g.,
	// `--add-dir`, `--mcp-config`) without code changes.
	ExtraArgs []string
	// Timeout caps how long we wait for the CLI to finish. Defaults to 90s.
	Timeout time.Duration

	// When BridgeURL is non-empty, the engine forwards the question to a
	// host-running `tools/claude-cli-bridge` over HTTP instead of trying to
	// exec the binary locally. This is the right mode when the server runs
	// inside a Docker container that can't reach the host's PATH.
	BridgeURL    string
	BridgeSecret string
	HTTP         *http.Client
}

func NewCLIEngine(disp *ToolDispatcher, binary, model string) *CLIEngine {
	if binary == "" {
		binary = "claude"
	}
	return &CLIEngine{
		Dispatcher: disp,
		Binary:     binary,
		Model:      model,
		Timeout:    90 * time.Second,
		HTTP:       &http.Client{Timeout: 120 * time.Second},
	}
}

// WithBridge wires the engine to call a host-side HTTP bridge instead of
// exec'ing `claude` directly. Returns the engine for fluent setup.
func (c *CLIEngine) WithBridge(url, secret string) *CLIEngine {
	c.BridgeURL = strings.TrimRight(url, "/")
	c.BridgeSecret = secret
	return c
}

// Stream mirrors Client.StreamConversation's surface so the SSE handler can
// swap engines without conditional logic.
func (c *CLIEngine) Stream(
	ctx context.Context,
	convo []apiMessage,
	emit func(OutEvent),
) ([]contentBlock, error) {
	if len(convo) == 0 {
		return nil, errors.New("empty conversation")
	}
	question := lastUserText(convo)
	if question == "" {
		return nil, errors.New("no user text to respond to")
	}

	snapshot := c.buildPillarSnapshot(ctx)
	system := c.systemPrompt(snapshot)

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Surface the chosen "tool" (snapshot scope) so the iOS chat shows a
	// chip — keeps parity with how the mock and Anthropic paths surface
	// their tool calls.
	emit(OutEvent{Type: "tool_call", ToolName: "pillar_snapshot"})

	var reply string
	var err error
	if c.BridgeURL != "" {
		reply, err = c.askBridge(runCtx, question, system)
	} else {
		reply, err = c.askExec(runCtx, question, system)
	}
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("claude CLI timed out after %s", timeout)
		}
		return nil, err
	}
	if reply == "" {
		return nil, errors.New("claude CLI returned empty output")
	}

	for _, chunk := range chunkText(reply, 24) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		emit(OutEvent{Type: "text", Text: chunk})
		time.Sleep(15 * time.Millisecond)
	}

	return []contentBlock{{Type: "text", Text: reply}}, nil
}

// askExec shells out to a locally installed `claude` binary. Right answer
// when the server runs on the host (no Docker between it and `claude`).
func (c *CLIEngine) askExec(ctx context.Context, question, system string) (string, error) {
	args := []string{"--print", "--output-format", "text", "--system-prompt", system}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	args = append(args, c.ExtraArgs...)

	cmd := exec.CommandContext(ctx, c.Binary, args...)
	cmd.Stdin = strings.NewReader(question)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("claude CLI failed: %s", msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// askBridge forwards the question to the host-side claude-cli-bridge over
// HTTP. Used when the server runs in a Docker container that can't see the
// host's `claude` binary.
func (c *CLIEngine) askBridge(ctx context.Context, question, system string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"question": question,
		"system":   system,
		"model":    c.Model,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BridgeURL+"/prompt", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.BridgeSecret != "" {
		req.Header.Set("X-Bridge-Secret", c.BridgeSecret)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude bridge: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		var apiErr struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &apiErr)
		msg := strings.TrimSpace(apiErr.Message)
		if msg == "" {
			msg = strings.TrimSpace(apiErr.Error)
		}
		if msg == "" {
			msg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("claude bridge: %s", msg)
	}
	var ok struct {
		Reply string `json:"reply"`
	}
	if err := json.Unmarshal(raw, &ok); err != nil {
		return "", fmt.Errorf("claude bridge: bad response: %w", err)
	}
	return strings.TrimSpace(ok.Reply), nil
}

func (c *CLIEngine) systemPrompt(snapshot string) string {
	const persona = `You are the cross-pillar assistant inside Northstar, a self-hosted personal life-OS that ties together one user's finance (Actual Budget), goals, and biometrics (WHOOP).

Tone: direct, specific, terse. Quote concrete numbers from the snapshot below — don't hedge with "approximately" when you have exact values.

Safety rail: For health questions, you may discuss recovery, sleep, strain, and supplement schedules already in the user's stack. Never diagnose, never recommend a new medication, peptide, or supplement that isn't already in their list, and never give medical advice. If asked about anything diagnostic or clinical, redirect to a clinician.`

	return persona + "\n\n## Current pillar snapshot (frozen at request time)\n\n" + snapshot
}

// lastUserText returns the concatenated text content of the most recent
// user-role message. Tool-result blocks are skipped.
func lastUserText(convo []apiMessage) string {
	for i := len(convo) - 1; i >= 0; i-- {
		if convo[i].Role != "user" {
			continue
		}
		var sb strings.Builder
		for _, b := range convo[i].Content {
			if b.Type == "text" {
				sb.WriteString(b.Text)
			}
		}
		return strings.TrimSpace(sb.String())
	}
	return ""
}

// buildPillarSnapshot calls a handful of read-only tools through the same
// dispatcher the real-API path uses, so the snapshot here is always in sync
// with the data the model would see via tool-use.
//
// Errors are tolerated — a missing pillar (e.g., AI runs before finance
// sync has populated) is shown as a placeholder so the model can still
// answer the rest. The whole purpose is "give the model enough context to
// answer", not "fail the whole reply if one query errors".
func (c *CLIEngine) buildPillarSnapshot(ctx context.Context) string {
	if c.Dispatcher == nil {
		return "(no dispatcher configured; snapshot unavailable)"
	}
	type section struct {
		title string
		tool  string
		args  string
	}
	sections := []section{
		{"Finance — current month summary", "finance_summary", `{}`},
		{"Finance — likely subscriptions", "finance_subscriptions", `{}`},
		{"Goals — today's brief", "goals_brief", `{}`},
		{"Goals — open milestones", "goals_milestones", `{}`},
		{"Health — today's readiness", "health_today", `{}`},
		{"Health — active supplements", "health_supplements", `{}`},
	}
	var sb strings.Builder
	for _, s := range sections {
		sb.WriteString("### ")
		sb.WriteString(s.title)
		sb.WriteString("\n```json\n")
		out, err := c.Dispatcher.Dispatch(ctx, s.tool, json.RawMessage(s.args))
		if err != nil {
			sb.WriteString(fmt.Sprintf("{\"error\":%q}", err.Error()))
		} else {
			sb.WriteString(out)
		}
		sb.WriteString("\n```\n\n")
	}
	return sb.String()
}
