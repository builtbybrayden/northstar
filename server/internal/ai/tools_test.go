package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Minimal schema covering only what the tool dispatchers touch.
func inProcDB(t *testing.T) *sql.DB {
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
		   notes TEXT, imported_at INTEGER NOT NULL)`,
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
		`CREATE TABLE goal_output_log (
		   id TEXT PRIMARY KEY, date TEXT, category TEXT, title TEXT,
		   body_md TEXT, url TEXT, created_at INTEGER NOT NULL)`,
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
	return d
}

func makeDispatcher(t *testing.T, now time.Time) *ToolDispatcher {
	d := inProcDB(t)
	return &ToolDispatcher{DB: d, Now: func() time.Time { return now }}
}

// ─── Tool dispatch tests ─────────────────────────────────────────────────

func TestDispatch_FinanceSummary(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	d := makeDispatcher(t, now)
	// Seed: 1 account, 1 budget, 1 spending transaction
	_, _ = d.DB.Exec(`INSERT INTO fin_accounts (actual_id, name, type, balance_cents, on_budget, closed, updated_at)
		VALUES ('a1', 'Chase', 'checking', 100000, 1, 0, 0)`)
	_, _ = d.DB.Exec(`INSERT INTO fin_budget_targets (category, monthly_cents, rationale, threshold_pcts, push_enabled, updated_at)
		VALUES ('Restaurants', 50000, 't', '[50,75,90,100]', 1, 0)`)
	_, _ = d.DB.Exec(`INSERT INTO fin_transactions (actual_id, account_id, date, payee, category, amount_cents, notes, imported_at)
		VALUES ('t1', 'a1', '2026-05-10', 'Diner', 'Restaurants', -7500, '', 0)`)

	out, err := d.Dispatch(context.Background(), "finance_summary", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out, `"month":"2026-05"`) ||
		!strings.Contains(out, `"net_worth_cents":100000`) ||
		!strings.Contains(out, `"spent_cents":7500`) ||
		!strings.Contains(out, `"category":"Restaurants"`) {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestDispatch_FinanceSearch_PayeeFilter(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	d := makeDispatcher(t, now)
	_, _ = d.DB.Exec(`INSERT INTO fin_accounts VALUES ('a1', 'Chase', '', 0, 1, 0, 0)`)
	_, _ = d.DB.Exec(`INSERT INTO fin_transactions VALUES ('t1', 'a1', '2026-05-10', 'Starbucks #4', 'Restaurants', -745, '', 0)`)
	_, _ = d.DB.Exec(`INSERT INTO fin_transactions VALUES ('t2', 'a1', '2026-05-10', 'Kroger',       'Groceries',  -8412, '', 0)`)

	out, err := d.Dispatch(context.Background(), "finance_search_transactions",
		json.RawMessage(`{"payee":"starbucks"}`))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out, `"count":1`) || !strings.Contains(out, "Starbucks") ||
		strings.Contains(out, "Kroger") {
		t.Errorf("filter failed: %s", out)
	}
}

func TestDispatch_GoalsMilestones_StatusFilter(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	d := makeDispatcher(t, now)
	insert := func(id, title, status string, flagship int) {
		_, _ = d.DB.Exec(`INSERT INTO goal_milestones VALUES (?, ?, '', '', ?, ?, 0, 0, 0)`,
			id, title, status, flagship)
	}
	insert("m1", "OSCP", "in_progress", 1)
	insert("m2", "CVE",  "pending",     0)
	insert("m3", "Past", "done",        0)
	insert("m4", "Old",  "archived",    0)

	// No filter → excludes archived
	out, err := d.Dispatch(context.Background(), "goals_milestones", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out, "OSCP") || strings.Contains(out, `"title":"Old"`) {
		t.Errorf("default filter wrong: %s", out)
	}

	// Filter to in_progress only
	out, err = d.Dispatch(context.Background(), "goals_milestones",
		json.RawMessage(`{"status_in":["in_progress"]}`))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out, "OSCP") || strings.Contains(out, "CVE") {
		t.Errorf("status filter wrong: %s", out)
	}
}

func TestDispatch_HealthToday_ComputesVerdict(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	d := makeDispatcher(t, now)
	_, _ = d.DB.Exec(`INSERT INTO health_recovery (date, score, hrv_ms, rhr, source)
		VALUES ('2026-05-12', 84, 78, 51, 'whoop')`)

	out, err := d.Dispatch(context.Background(), "health_today", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out, `"recovery_score":84`) ||
		!strings.Contains(out, `"verdict":"push"`) {
		t.Errorf("verdict wrong: %s", out)
	}
}

func TestDispatch_HealthToday_VerdictMaintainAndRecover(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	d := makeDispatcher(t, now)
	_, _ = d.DB.Exec(`INSERT INTO health_recovery (date, score, source) VALUES ('2026-05-12', 50, 'whoop')`)
	out, _ := d.Dispatch(context.Background(), "health_today", json.RawMessage(`{}`))
	if !strings.Contains(out, `"verdict":"maintain"`) {
		t.Errorf("expected maintain for 50, got %s", out)
	}

	_, _ = d.DB.Exec(`UPDATE health_recovery SET score = 28 WHERE date = '2026-05-12'`)
	out, _ = d.Dispatch(context.Background(), "health_today", json.RawMessage(`{}`))
	if !strings.Contains(out, `"verdict":"recover"`) {
		t.Errorf("expected recover for 28, got %s", out)
	}
}

func TestDispatch_UnknownTool(t *testing.T) {
	now := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)
	d := makeDispatcher(t, now)
	_, err := d.Dispatch(context.Background(), "not_a_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestDefs_LastIsCached(t *testing.T) {
	defs := Defs()
	if len(defs) == 0 {
		t.Fatal("expected tools")
	}
	if defs[len(defs)-1].CacheControl == nil {
		t.Errorf("last tool should have cache_control")
	}
}

func TestSystemBlocks_LastIsCached(t *testing.T) {
	now := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)
	blocks := SystemBlocks(now)
	if len(blocks) == 0 {
		t.Fatal("expected blocks")
	}
	if blocks[len(blocks)-1].CacheControl == nil {
		t.Errorf("last system block should have cache_control")
	}
}

func TestUsage_Add_Accumulates(t *testing.T) {
	a := Usage{InputTokens: 100, OutputTokens: 50, CacheReadInputTokens: 200, CacheCreationInputTokens: 80}
	a.Add(Usage{InputTokens: 30, OutputTokens: 70, CacheReadInputTokens: 0, CacheCreationInputTokens: 5})
	if a.InputTokens != 130 {
		t.Errorf("InputTokens = %d, want 130", a.InputTokens)
	}
	if a.OutputTokens != 120 {
		t.Errorf("OutputTokens = %d, want 120", a.OutputTokens)
	}
	if a.CacheReadInputTokens != 200 {
		t.Errorf("CacheReadInputTokens = %d, want 200", a.CacheReadInputTokens)
	}
	if a.CacheCreationInputTokens != 85 {
		t.Errorf("CacheCreationInputTokens = %d, want 85", a.CacheCreationInputTokens)
	}
}

func TestFilterDefsByScope_EmptyReturnsAll(t *testing.T) {
	defs := Defs()
	got := FilterDefsByScope(defs, nil)
	if len(got) != len(defs) {
		t.Errorf("empty scope should keep all tools, got %d vs %d", len(got), len(defs))
	}
}

func TestFilterDefsByScope_FinanceOnly(t *testing.T) {
	got := FilterDefsByScope(Defs(), []string{"finance"})
	if len(got) == 0 {
		t.Fatal("expected some finance tools")
	}
	for _, d := range got {
		if !strings.HasPrefix(d.Name, "finance_") {
			t.Errorf("unexpected tool %q in finance-only scope", d.Name)
		}
	}
	if got[len(got)-1].CacheControl == nil {
		t.Errorf("filtered list should have cache_control on the last tool")
	}
}

func TestFilterDefsByScope_MultiplePillars(t *testing.T) {
	got := FilterDefsByScope(Defs(), []string{"finance", "health"})
	hasF, hasH := false, false
	for _, d := range got {
		switch {
		case strings.HasPrefix(d.Name, "finance_"):
			hasF = true
		case strings.HasPrefix(d.Name, "health_"):
			hasH = true
		case strings.HasPrefix(d.Name, "goals_"):
			t.Errorf("scope didn't include goals but %q slipped through", d.Name)
		}
	}
	if !hasF || !hasH {
		t.Errorf("expected both finance and health tools, got hasF=%v hasH=%v", hasF, hasH)
	}
}

func TestFilterDefsByScope_AllUnknownReturnsAll(t *testing.T) {
	// Defensive: a scope of only unknown strings shouldn't blank out the tool
	// surface — that would silently break the chat. We treat it as "no filter".
	defs := Defs()
	got := FilterDefsByScope(defs, []string{"banana", "wallet"})
	if len(got) != len(defs) {
		t.Errorf("unknown-only scope should fall through to all tools, got %d", len(got))
	}
}

func TestNormalizePillarScope_DedupesAndSorts(t *testing.T) {
	in := []string{"Health", "finance", " HEALTH ", "wallet", "goals"}
	out := normalizePillarScope(in)
	want := []string{"finance", "goals", "health"}
	if len(out) != len(want) {
		t.Fatalf("normalize = %v, want %v", out, want)
	}
	for i := range out {
		if out[i] != want[i] {
			t.Errorf("normalize[%d] = %q, want %q", i, out[i], want[i])
		}
	}
}

func TestUsage_JSON_MatchesAnthropicShape(t *testing.T) {
	// message_start frame from Anthropic should round-trip into Usage.
	raw := []byte(`{"type":"message_start","message":{"usage":{"input_tokens":42,"output_tokens":1,"cache_creation_input_tokens":120,"cache_read_input_tokens":3500}}}`)
	var ev streamEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Message.Usage == nil {
		t.Fatal("expected Message.Usage to be populated")
	}
	if ev.Message.Usage.InputTokens != 42 {
		t.Errorf("InputTokens = %d, want 42", ev.Message.Usage.InputTokens)
	}
	if ev.Message.Usage.CacheReadInputTokens != 3500 {
		t.Errorf("CacheReadInputTokens = %d, want 3500", ev.Message.Usage.CacheReadInputTokens)
	}
}
