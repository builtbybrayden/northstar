package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigrate_AppliesAll verifies every migration applies cleanly to a fresh
// DB, and that the schema matches the columns the handlers expect. This is the
// smoke test that catches regressions like "migration 00003 added a column the
// AI handler tries to write to but the migration didn't run."
func TestMigrate_AppliesAll(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "smoke.db")
	d, err := Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Spot-check tables and the columns the handlers actually read/write.
	cases := []struct {
		table  string
		column string
	}{
		// Phase 0 — core
		{"users", "settings_json"},
		{"devices", "token_hash"},
		{"pairing_codes", "token"},
		// Phase 0 — finance
		{"fin_accounts", "on_budget"},
		{"fin_transactions", "amount_cents"},
		{"fin_budget_targets", "threshold_pcts"},
		// Phase 0 — goals
		{"goal_milestones", "flagship"},
		{"goal_daily_log", "items_json"},
		{"goal_reminders", "next_fires_at"},
		// Phase 0 — health
		{"health_recovery", "score"},
		{"health_sleep", "debt_min"},
		{"health_strain", "score"},
		{"health_supplement_defs", "schedule_json"},
		{"health_supplement_log", "taken_at"},
		// Phase 0 — AI
		{"ai_conversations", "pillar_scope"},
		{"ai_messages", "content_json"},
		// Phase 2 — notifications
		{"notifications", "dedup_key"},
		{"notif_rules", "max_per_day"},
		{"notif_daily_counts", "count"},
		{"fin_merchant_stats", "median_abs_cents"},
		// Phase 6 polish — usage telemetry
		{"ai_messages", "usage_json"},
		// Phase 7 — finance polish: per-transaction category override
		{"fin_transactions", "category_user"},
		// Phase 7 — habit tracker
		{"goal_habits", "target_per_week"},
		{"goal_habit_log", "count"},
		// Phase 7.1 — category grouping
		{"fin_budget_targets", "category_group"},
		// Phase 7.5 — daily account balance snapshots
		{"fin_account_balance_history", "balance_cents"},
		{"fin_account_balance_history", "on_budget"},
		// Phase 7.5 — transfer linkage + split flag
		{"fin_transactions", "transfer_id"},
		{"fin_transactions", "is_parent"},
		// Phase 7.5 — finance settings (savings target)
		{"fin_settings", "savings_target_pct"},
		// Phase 7.5 — per-account savings destination override
		{"fin_accounts", "is_savings_destination"},
		// Phase 7.5 — classifier flags
		{"fin_accounts", "include_in_income"},
		{"fin_transactions", "flow_override"},
	}
	for _, c := range cases {
		t.Run(c.table+"."+c.column, func(t *testing.T) {
			if !columnExists(t, d, c.table, c.column) {
				t.Errorf("expected %s.%s to exist after migration", c.table, c.column)
			}
		})
	}
}

// columnExists queries PRAGMA table_info and returns true if `column` is one
// of the named columns of `table`.
func columnExists(t *testing.T, d *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := d.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == column {
			return true
		}
	}
	return false
}

// TestMigrate_Idempotent — running Migrate twice in a row should be a no-op.
func TestMigrate_Idempotent(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "idem.db")
	d, err := Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	if err := Migrate(d); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(d); err != nil {
		t.Fatalf("second migrate (should be no-op): %v", err)
	}
}

// TestMigrate_SeededNotifRules — seed rules cover every category the engine
// fires. Catches "added a notification category in code but forgot the seed."
func TestMigrate_SeededNotifRules(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "rules.db")
	d, err := Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	wantCategories := []string{
		"purchase", "budget_threshold", "anomaly",
		"daily_brief", "evening_retro",
		"supplement", "health_insight", "goal_milestone",
		"subscription_new", "weekly_retro",
		"forecast_warning",
	}
	for _, cat := range wantCategories {
		var found int
		_ = d.QueryRow(`SELECT COUNT(*) FROM notif_rules WHERE category = ?`, cat).Scan(&found)
		if found == 0 {
			t.Errorf("notif_rules missing seed for category %q", cat)
		}
	}
}

// TestMigrate_SeedsRentAndVehiclePayments — migration 00007 seeds two
// user-requested budget targets so the iOS category picker offers them
// even before any transactions are re-categorized into them.
func TestMigrate_SeedsRentAndVehiclePayments(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "seeds.db")
	d, err := Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, c := range []struct {
		name, group string
	}{
		{"Rent", "Living Expenses"},
		{"Vehicle Payments", "Transportation"},
	} {
		var got string
		err := d.QueryRow(
			`SELECT category_group FROM fin_budget_targets WHERE category = ?`,
			c.name).Scan(&got)
		if err != nil {
			t.Errorf("%s row missing: %v", c.name, err)
			continue
		}
		if got != c.group {
			t.Errorf("%s.category_group = %q, want %q", c.name, got, c.group)
		}
	}
}

// TestMigrate_UsageJSONWritable — the column added in migration 00003 must
// accept the JSON shape the AI handler writes (input/output/cache_*_tokens).
func TestMigrate_UsageJSONWritable(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "usage.db")
	d, err := Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed a conversation so the FK in ai_messages resolves.
	if _, err := d.Exec(
		`INSERT INTO ai_conversations (id, started_at, title, pillar_scope, archived)
		 VALUES ('c1', 0, 't', '[]', 0)`); err != nil {
		t.Fatalf("seed conv: %v", err)
	}

	usage := `{"input_tokens":42,"output_tokens":7,"cache_creation_input_tokens":120,"cache_read_input_tokens":3500}`
	if _, err := d.Exec(
		`INSERT INTO ai_messages (id, conv_id, role, content_json, usage_json, created_at)
		 VALUES ('m1', 'c1', 'assistant', '[]', ?, 0)`, usage); err != nil {
		t.Fatalf("insert with usage_json: %v", err)
	}

	var got string
	if err := d.QueryRow(`SELECT usage_json FROM ai_messages WHERE id = 'm1'`).Scan(&got); err != nil {
		t.Fatalf("read usage_json: %v", err)
	}
	if !strings.Contains(got, `"input_tokens":42`) {
		t.Errorf("usage_json round-trip lost data: %s", got)
	}
}

