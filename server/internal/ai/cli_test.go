package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// fakeClaudeBinary writes a tiny script the CLIEngine will invoke instead
// of the real `claude` CLI. The reply is baked into the script body so
// each test gets a clean, isolated binary (no env-var leakage between
// tests in the same process). On Windows we use a .bat; everywhere else,
// a POSIX shell script.
func fakeClaudeBinary(t *testing.T, reply string) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "claude.bat")
		body := "@echo off\r\n"
		if reply == "" {
			// `rem` is a no-op with no stdout side-effect. echo with no
			// argument would print "ECHO is on." which we don't want.
			body += "rem empty\r\n"
		} else {
			body += "echo " + escapeBat(reply) + "\r\n"
		}
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatalf("write fake binary: %v", err)
		}
		return path
	}
	path := filepath.Join(dir, "claude")
	body := "#!/bin/sh\nprintf '%s' " + shellQuote(reply) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return path
}

func escapeBat(s string) string {
	// Minimal escaping for the reply strings we use in tests.
	s = strings.ReplaceAll(s, "%", "%%")
	s = strings.ReplaceAll(s, "\"", "\"\"")
	return s
}

func shellQuote(s string) string {
	// Single-quote and escape any embedded single quotes.
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func minimalDispatcherDB(t *testing.T) *ToolDispatcher {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	stmts := []string{
		`CREATE TABLE fin_accounts (
		   actual_id TEXT PRIMARY KEY, name TEXT, type TEXT,
		   balance_cents INTEGER NOT NULL DEFAULT 0, on_budget INTEGER NOT NULL DEFAULT 1,
		   closed INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE fin_transactions (
		   actual_id TEXT PRIMARY KEY, account_id TEXT, date TEXT,
		   payee TEXT, category TEXT, amount_cents INTEGER NOT NULL,
		   notes TEXT, imported_at INTEGER NOT NULL, category_user TEXT)`,
		`CREATE TABLE fin_budget_targets (
		   category TEXT PRIMARY KEY, monthly_cents INTEGER NOT NULL,
		   rationale TEXT, threshold_pcts TEXT NOT NULL DEFAULT '[50,75,90,100]',
		   push_enabled INTEGER NOT NULL DEFAULT 1, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE goal_milestones (
		   id TEXT PRIMARY KEY, title TEXT NOT NULL, description_md TEXT,
		   due_date TEXT, status TEXT NOT NULL DEFAULT 'pending',
		   flagship INTEGER NOT NULL DEFAULT 0, display_order INTEGER NOT NULL DEFAULT 0,
		   created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE goal_daily_log (
		   date TEXT PRIMARY KEY, items_json TEXT NOT NULL DEFAULT '[]',
		   reflection_md TEXT, streak_count INTEGER NOT NULL DEFAULT 0,
		   updated_at INTEGER NOT NULL)`,
		`CREATE TABLE goal_reminders (
		   id TEXT PRIMARY KEY, title TEXT NOT NULL, body TEXT, recurrence TEXT NOT NULL,
		   next_fires_at INTEGER, active INTEGER NOT NULL DEFAULT 1,
		   created_at INTEGER NOT NULL)`,
		`CREATE TABLE health_recovery (
		   date TEXT PRIMARY KEY, score INTEGER, hrv_ms REAL, rhr INTEGER, source TEXT)`,
		`CREATE TABLE health_sleep (
		   date TEXT PRIMARY KEY, duration_min INTEGER, score INTEGER, debt_min INTEGER, source TEXT)`,
		`CREATE TABLE health_strain (
		   date TEXT PRIMARY KEY, score REAL, avg_hr INTEGER, max_hr INTEGER, source TEXT)`,
		`CREATE TABLE health_supplement_defs (
		   id TEXT PRIMARY KEY, name TEXT NOT NULL, dose TEXT, category TEXT,
		   schedule_json TEXT, cycle_days_on INTEGER, cycle_days_off INTEGER,
		   reminder_enabled INTEGER NOT NULL DEFAULT 1, active INTEGER NOT NULL DEFAULT 1,
		   prescribing_doc TEXT, notes TEXT, created_at INTEGER NOT NULL)`,
		`CREATE TABLE health_supplement_log (
		   id TEXT PRIMARY KEY, def_id TEXT NOT NULL, taken_at INTEGER NOT NULL, notes TEXT)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return &ToolDispatcher{DB: d, Now: time.Now}
}

func TestCLIEngine_Stream_FakeBinary(t *testing.T) {
	bin := fakeClaudeBinary(t, "All systems normal.")
	disp := minimalDispatcherDB(t)
	e := NewCLIEngine(disp, bin, "")
	e.Timeout = 10 * time.Second

	convo := []apiMessage{{
		Role:    "user",
		Content: []contentBlock{{Type: "text", Text: "How are things?"}},
	}}

	var events []OutEvent
	emit := func(ev OutEvent) { events = append(events, ev) }

	blocks, err := e.Stream(context.Background(), convo, emit)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Type != "text" || blocks[0].Text != "All systems normal." {
		t.Errorf("unexpected blocks: %+v", blocks)
	}

	// Must have emitted a tool_call (the pillar snapshot marker) and at
	// least one text chunk that reconstructs the canned reply.
	var sawToolCall bool
	var streamed strings.Builder
	for _, ev := range events {
		if ev.Type == "tool_call" && ev.ToolName == "pillar_snapshot" {
			sawToolCall = true
		}
		if ev.Type == "text" {
			streamed.WriteString(ev.Text)
		}
	}
	if !sawToolCall {
		t.Error("expected pillar_snapshot tool_call event")
	}
	if streamed.String() != "All systems normal." {
		t.Errorf("streamed = %q, want %q", streamed.String(), "All systems normal.")
	}
}

func TestCLIEngine_Stream_EmptyOutputFails(t *testing.T) {
	bin := fakeClaudeBinary(t, "")
	disp := minimalDispatcherDB(t)
	e := NewCLIEngine(disp, bin, "")
	e.Timeout = 5 * time.Second

	convo := []apiMessage{{
		Role:    "user",
		Content: []contentBlock{{Type: "text", Text: "ping"}},
	}}

	_, err := e.Stream(context.Background(), convo, func(OutEvent) {})
	if err == nil {
		t.Fatal("expected error on empty CLI output")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCLIEngine_Stream_MissingBinaryFails(t *testing.T) {
	disp := minimalDispatcherDB(t)
	e := NewCLIEngine(disp, "/nonexistent/claude", "")
	e.Timeout = 2 * time.Second

	convo := []apiMessage{{
		Role:    "user",
		Content: []contentBlock{{Type: "text", Text: "ping"}},
	}}

	_, err := e.Stream(context.Background(), convo, func(OutEvent) {})
	if err == nil {
		t.Fatal("expected error when CLI binary is missing")
	}
}

func TestCLIEngine_Stream_NoUserText(t *testing.T) {
	disp := minimalDispatcherDB(t)
	e := NewCLIEngine(disp, "claude", "")

	// Convo with only an assistant message — no user text to respond to.
	convo := []apiMessage{{
		Role:    "assistant",
		Content: []contentBlock{{Type: "text", Text: "hello"}},
	}}

	_, err := e.Stream(context.Background(), convo, func(OutEvent) {})
	if err == nil {
		t.Fatal("expected error when no user text is present")
	}
}

func TestCLIEngine_Stream_BridgeMode_Success(t *testing.T) {
	// Stand up an httptest server that mimics the host-side bridge.
	var receivedBody map[string]string
	var sawSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prompt" {
			http.Error(w, "wrong path", 404)
			return
		}
		sawSecret = r.Header.Get("X-Bridge-Secret")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"reply": "From the bridge."})
	}))
	defer srv.Close()

	disp := minimalDispatcherDB(t)
	e := NewCLIEngine(disp, "claude", "claude-sonnet-4-6").
		WithBridge(srv.URL, "shh")
	e.Timeout = 10 * time.Second

	convo := []apiMessage{{
		Role:    "user",
		Content: []contentBlock{{Type: "text", Text: "Should I push hard today?"}},
	}}

	var events []OutEvent
	blocks, err := e.Stream(context.Background(), convo, func(ev OutEvent) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Text != "From the bridge." {
		t.Errorf("unexpected blocks: %+v", blocks)
	}
	if receivedBody["question"] != "Should I push hard today?" {
		t.Errorf("bridge got question=%q", receivedBody["question"])
	}
	if receivedBody["model"] != "claude-sonnet-4-6" {
		t.Errorf("bridge got model=%q", receivedBody["model"])
	}
	if !strings.Contains(receivedBody["system"], "Current pillar snapshot") {
		t.Error("system prompt missing snapshot section")
	}
	if sawSecret != "shh" {
		t.Errorf("bridge auth header = %q, want 'shh'", sawSecret)
	}
	var sawToolCall bool
	for _, ev := range events {
		if ev.Type == "tool_call" && ev.ToolName == "pillar_snapshot" {
			sawToolCall = true
		}
	}
	if !sawToolCall {
		t.Error("expected pillar_snapshot tool_call event")
	}
}

func TestCLIEngine_Stream_BridgeMode_PropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "handler_failed",
			"message": "claude CLI exit 1: API key required",
		})
	}))
	defer srv.Close()

	disp := minimalDispatcherDB(t)
	e := NewCLIEngine(disp, "claude", "").WithBridge(srv.URL, "")
	e.Timeout = 5 * time.Second

	_, err := e.Stream(context.Background(), []apiMessage{{
		Role:    "user",
		Content: []contentBlock{{Type: "text", Text: "ping"}},
	}}, func(OutEvent) {})
	if err == nil {
		t.Fatal("expected error from bridge 500")
	}
	if !strings.Contains(err.Error(), "API key required") {
		t.Errorf("error message should surface bridge message, got: %v", err)
	}
}

func TestCLIEngine_Stream_BridgeMode_UnreachableHost(t *testing.T) {
	disp := minimalDispatcherDB(t)
	// Port 1 isn't normally bound; the dial fails fast.
	e := NewCLIEngine(disp, "claude", "").WithBridge("http://127.0.0.1:1", "")
	e.Timeout = 3 * time.Second

	_, err := e.Stream(context.Background(), []apiMessage{{
		Role:    "user",
		Content: []contentBlock{{Type: "text", Text: "ping"}},
	}}, func(OutEvent) {})
	if err == nil {
		t.Fatal("expected error when bridge is unreachable")
	}
}

func TestCLIEngine_BuildPillarSnapshot_TolerantOfMissingData(t *testing.T) {
	disp := minimalDispatcherDB(t)
	e := NewCLIEngine(disp, "claude", "")
	snap := e.buildPillarSnapshot(context.Background())

	// All six sections should render even with an empty DB; missing data
	// becomes an empty-result JSON, not an error.
	for _, label := range []string{
		"Finance — current month summary",
		"Goals — today's brief",
		"Health — today's readiness",
	} {
		if !strings.Contains(snap, label) {
			t.Errorf("snapshot missing section %q:\n%s", label, snap)
		}
	}

	// Snapshot must be valid-looking — every section block ends with a
	// closing ``` fence, so the model can parse it.
	if strings.Count(snap, "```") < 12 {
		t.Errorf("expected ≥12 code fences (6 sections × 2 each), got %d:\n%s",
			strings.Count(snap, "```"), snap)
	}
}
