package goals

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func inProcDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	stmts := []string{
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
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return d
}

func makeHandlers(t *testing.T, fixedNow time.Time) *Handlers {
	t.Helper()
	d := inProcDB(t)
	h := NewHandlers(d)
	h.Now = func() time.Time { return fixedNow }
	return h
}

// ─── Cron parsing ─────────────────────────────────────────────────────────

func TestNextFiresAt(t *testing.T) {
	from := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC) // Tue 10:00
	cases := []struct {
		recur string
		want  time.Time
	}{
		{"0 7 * * *",  time.Date(2026, 5, 13, 7, 0, 0, 0, time.UTC)},  // tomorrow 7am
		{"30 7 * * 1", time.Date(2026, 5, 18, 7, 30, 0, 0, time.UTC)}, // next Monday 7:30
		{"0 9 1 * *",  time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)},   // next month 1st 9am
	}
	for _, c := range cases {
		got, err := NextFiresAt(c.recur, from)
		if err != nil {
			t.Errorf("NextFiresAt(%q) err: %v", c.recur, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("NextFiresAt(%q) = %v, want %v", c.recur, got, c.want)
		}
	}
}

func TestNextFiresAt_Invalid(t *testing.T) {
	_, err := NextFiresAt("not-a-cron", time.Now())
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

// ─── Streak counter ───────────────────────────────────────────────────────

func TestStreak_ConsecutiveDays(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	h := makeHandlers(t, now)

	// Seed 3 consecutive days of non-empty logs
	for i := 0; i < 3; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		_, err := h.DB.Exec(
			`INSERT INTO goal_daily_log (date, items_json, updated_at)
			 VALUES (?, '[{"id":"a","text":"t","done":false,"source":"manual"}]', ?)`,
			date, now.Unix())
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	r := httptest.NewRequest("PUT", "/api/goals/daily/"+now.Format("2006-01-02"), nil)
	if err := h.recomputeStreak(r, now.Format("2006-01-02")); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	log, err := h.loadDailyLog(r, now.Format("2006-01-02"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if log.StreakCount != 3 {
		t.Errorf("streak = %d, want 3", log.StreakCount)
	}
}

func TestStreak_GapResets(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	h := makeHandlers(t, now)

	// Today logged, yesterday SKIPPED, day before logged
	today := now.Format("2006-01-02")
	dayBefore := now.AddDate(0, 0, -2).Format("2006-01-02")
	for _, d := range []string{today, dayBefore} {
		_, _ = h.DB.Exec(
			`INSERT INTO goal_daily_log (date, items_json, updated_at)
			 VALUES (?, '[{"id":"x","text":"t","done":false,"source":"manual"}]', ?)`,
			d, now.Unix())
	}
	r := httptest.NewRequest("PUT", "/", nil)
	if err := h.recomputeStreak(r, today); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	log, _ := h.loadDailyLog(r, today)
	if log.StreakCount != 1 {
		t.Errorf("streak with gap = %d, want 1", log.StreakCount)
	}
}

func TestStreak_EmptyItemsBreaksStreak(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	h := makeHandlers(t, now)

	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	_, _ = h.DB.Exec(`INSERT INTO goal_daily_log (date, items_json, updated_at)
		VALUES (?, '[{"id":"a","text":"t","done":false,"source":"manual"}]', ?)`, today, now.Unix())
	// Yesterday has an empty items_json — should break streak
	_, _ = h.DB.Exec(`INSERT INTO goal_daily_log (date, items_json, updated_at)
		VALUES (?, '[]', ?)`, yesterday, now.Unix())

	r := httptest.NewRequest("PUT", "/", nil)
	_ = h.recomputeStreak(r, today)
	log, _ := h.loadDailyLog(r, today)
	if log.StreakCount != 1 {
		t.Errorf("streak with empty yesterday = %d, want 1", log.StreakCount)
	}
}

// ─── Brief composer ───────────────────────────────────────────────────────

func TestBrief_PullsTodayItemsAndRollovers(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	h := makeHandlers(t, now)

	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	// Today has 1 manual item
	_, _ = h.DB.Exec(`INSERT INTO goal_daily_log (date, items_json, updated_at)
		VALUES (?, '[{"id":"t1","text":"new today","done":false,"source":"manual"}]', ?)`,
		today, now.Unix())
	// Yesterday had 2 items, 1 done + 1 open → only 1 should roll over
	_, _ = h.DB.Exec(`INSERT INTO goal_daily_log (date, items_json, updated_at)
		VALUES (?, '[{"id":"y1","text":"y done","done":true,"source":"manual"},{"id":"y2","text":"y open","done":false,"source":"manual"}]', ?)`,
		yesterday, now.Unix())

	br, err := h.ComposeBrief(context.Background(), now)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if len(br.Items) != 2 {
		t.Fatalf("expected 2 items (1 today + 1 rollover), got %d", len(br.Items))
	}
	var rolloverCount int
	for _, it := range br.Items {
		if it.Source == "rollover" {
			rolloverCount++
		}
	}
	if rolloverCount != 1 {
		t.Errorf("rolloverCount = %d, want 1", rolloverCount)
	}
}

func TestBrief_MilestonesDueWithin7Days(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	h := makeHandlers(t, now)

	// Three milestones: due today, due in 5d, due in 30d
	insertMilestone := func(id, title, due, status string) {
		_, _ = h.DB.Exec(`INSERT INTO goal_milestones
			(id, title, description_md, due_date, status, flagship, display_order, created_at, updated_at)
			VALUES (?, ?, '', ?, ?, 0, 0, ?, ?)`,
			id, title, due, status, now.Unix(), now.Unix())
	}
	insertMilestone("m1", "Soon",    now.Format("2006-01-02"), "pending")
	insertMilestone("m2", "Within",  now.AddDate(0, 0, 5).Format("2006-01-02"), "in_progress")
	insertMilestone("m3", "Far",     now.AddDate(0, 0, 30).Format("2006-01-02"), "pending")
	insertMilestone("m4", "Done",    now.Format("2006-01-02"), "done") // excluded

	br, err := h.ComposeBrief(context.Background(), now)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if len(br.MilestonesDueSoon) != 2 {
		t.Errorf("expected 2 milestones in window (excluding done + 30d), got %d", len(br.MilestonesDueSoon))
	}
}

func TestBrief_ActiveRemindersInjected(t *testing.T) {
	now := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	h := makeHandlers(t, now)

	// One reminder firing today, one already past today (inactive in window)
	fires := time.Date(2026, 5, 12, 7, 0, 0, 0, time.UTC).Unix()
	_, _ = h.DB.Exec(`INSERT INTO goal_reminders
		(id, title, body, recurrence, next_fires_at, active, created_at)
		VALUES ('r1', 'Daily standup', '', '0 7 * * *', ?, 1, ?)`, fires, now.Unix())

	// Tomorrow's reminder (out of window)
	tom := now.AddDate(0, 0, 1).Unix()
	_, _ = h.DB.Exec(`INSERT INTO goal_reminders
		(id, title, body, recurrence, next_fires_at, active, created_at)
		VALUES ('r2', 'Tomorrow', '', '0 7 * * *', ?, 1, ?)`, tom, now.Unix())

	// Inactive reminder for today (should NOT appear)
	_, _ = h.DB.Exec(`INSERT INTO goal_reminders
		(id, title, body, recurrence, next_fires_at, active, created_at)
		VALUES ('r3', 'Off', '', '0 7 * * *', ?, 0, ?)`, fires, now.Unix())

	br, err := h.ComposeBrief(context.Background(), now)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if len(br.ActiveReminders) != 1 || br.ActiveReminders[0].Title != "Daily standup" {
		t.Errorf("expected 1 reminder (Daily standup), got %+v", br.ActiveReminders)
	}
	// Reminder should also be injected as a brief item
	found := false
	for _, it := range br.Items {
		if it.Source == "reminder" && it.Text == "Daily standup" {
			found = true
		}
	}
	if !found {
		t.Errorf("reminder not injected into items: %+v", br.Items)
	}
}

// ─── mondayOf helper ──────────────────────────────────────────────────────

func TestMondayOf(t *testing.T) {
	// 2026-05-12 is a Tuesday → Monday is 2026-05-11
	tue := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	want := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	got := mondayOf(tue)
	if !got.Equal(want) {
		t.Errorf("mondayOf(Tue) = %v, want %v", got, want)
	}

	// Sunday → previous Monday
	sun := time.Date(2026, 5, 17, 14, 0, 0, 0, time.UTC)
	wantSun := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	gotSun := mondayOf(sun)
	if !gotSun.Equal(wantSun) {
		t.Errorf("mondayOf(Sun) = %v, want %v", gotSun, wantSun)
	}
}
