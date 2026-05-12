package notify

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// inProcDB spins up an in-memory SQLite with the minimal schema needed for
// composer tests. Independent from the real migrations so the tests stay fast.
func inProcDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	stmts := []string{
		`CREATE TABLE notifications (
			id TEXT PRIMARY KEY, category TEXT NOT NULL, title TEXT NOT NULL,
			body TEXT, payload_json TEXT, priority INTEGER NOT NULL DEFAULT 5,
			dedup_key TEXT UNIQUE, created_at INTEGER NOT NULL,
			read_at INTEGER, delivered_at INTEGER,
			delivery_status TEXT NOT NULL DEFAULT 'pending')`,
		`CREATE TABLE notif_rules (
			id TEXT PRIMARY KEY, category TEXT NOT NULL UNIQUE,
			enabled INTEGER NOT NULL DEFAULT 1,
			quiet_hours_start TEXT, quiet_hours_end TEXT,
			bypass_quiet INTEGER NOT NULL DEFAULT 0,
			delivery TEXT NOT NULL DEFAULT 'push',
			max_per_day INTEGER NOT NULL DEFAULT 99,
			updated_at INTEGER NOT NULL)`,
		`CREATE TABLE notif_daily_counts (
			category TEXT NOT NULL, date TEXT NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (category, date))`,
		`CREATE TABLE devices (
			id TEXT PRIMARY KEY, user_id TEXT, name TEXT, token_hash TEXT,
			apns_token TEXT, paired_at INTEGER, last_seen INTEGER, revoked_at INTEGER)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return d
}

type capturingSender struct {
	calls []PreparedNotification
}

func (c *capturingSender) Send(_ context.Context, n PreparedNotification) error {
	c.calls = append(c.calls, n)
	return nil
}
func (*capturingSender) Mode() string { return "test" }

func seedRule(t *testing.T, d *sql.DB, category string, opts ruleOpts) {
	t.Helper()
	_, err := d.Exec(`INSERT INTO notif_rules
		(id, category, enabled, quiet_hours_start, quiet_hours_end, bypass_quiet, delivery, max_per_day, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'push', ?, 0)`,
		"seed-"+category, category,
		boolToTI(opts.enabled), opts.quietStart, opts.quietEnd,
		boolToTI(opts.bypassQuiet), opts.maxPerDay)
	if err != nil {
		t.Fatalf("seedRule: %v", err)
	}
}

type ruleOpts struct {
	enabled     bool
	quietStart  string
	quietEnd    string
	bypassQuiet bool
	maxPerDay   int
}

func boolToTI(b bool) int { if b { return 1 }; return 0 }

// ─── Tests ──────────────────────────────────────────────────────────────

func TestComposer_FireBasic(t *testing.T) {
	d := inProcDB(t)
	seedRule(t, d, CatPurchase, ruleOpts{enabled: true, maxPerDay: 10})

	send := &capturingSender{}
	c := NewComposer(d, send)
	c.Now = func() time.Time { return time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC) }

	id, reason, err := c.Fire(context.Background(), Event{
		Category: CatPurchase, Title: "test", Body: "ok",
		DedupKey: "purchase:tx-1", Priority: PriNormal,
	})
	if err != nil {
		t.Fatalf("fire: %v", err)
	}
	if reason != SkipNone || id == "" {
		t.Fatalf("expected sent, got reason=%v id=%q", reason, id)
	}
	if len(send.calls) != 1 {
		t.Fatalf("expected 1 send, got %d", len(send.calls))
	}
}

func TestComposer_DedupSkips(t *testing.T) {
	d := inProcDB(t)
	seedRule(t, d, CatPurchase, ruleOpts{enabled: true, maxPerDay: 10})

	send := &capturingSender{}
	c := NewComposer(d, send)
	c.Now = func() time.Time { return time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC) }

	ev := Event{Category: CatPurchase, Title: "t", DedupKey: "dup:1"}
	if _, r, _ := c.Fire(context.Background(), ev); r != SkipNone {
		t.Fatalf("first fire should send, got %v", r)
	}
	_, r, err := c.Fire(context.Background(), ev)
	if err != nil {
		t.Fatalf("second fire err: %v", err)
	}
	if r != SkipDedup {
		t.Fatalf("expected SkipDedup, got %v", r)
	}
	if len(send.calls) != 1 {
		t.Fatalf("dedup should keep send count at 1, got %d", len(send.calls))
	}
}

func TestComposer_QuietHoursSkips(t *testing.T) {
	d := inProcDB(t)
	seedRule(t, d, CatPurchase, ruleOpts{
		enabled: true, quietStart: "22:00", quietEnd: "07:00", maxPerDay: 10,
	})

	send := &capturingSender{}
	c := NewComposer(d, send)
	c.Now = func() time.Time { return time.Date(2026, 5, 12, 23, 30, 0, 0, time.UTC) }

	_, r, _ := c.Fire(context.Background(), Event{
		Category: CatPurchase, Title: "late", DedupKey: "late:1", Priority: PriNormal,
	})
	if r != SkipQuietHours {
		t.Fatalf("expected SkipQuietHours at 23:30, got %v", r)
	}
}

func TestComposer_CriticalBypassesQuietHours(t *testing.T) {
	d := inProcDB(t)
	seedRule(t, d, CatBudgetThreshold, ruleOpts{
		enabled: true, quietStart: "22:00", quietEnd: "07:00", maxPerDay: 10,
	})
	send := &capturingSender{}
	c := NewComposer(d, send)
	c.Now = func() time.Time { return time.Date(2026, 5, 12, 23, 30, 0, 0, time.UTC) }

	_, r, _ := c.Fire(context.Background(), Event{
		Category: CatBudgetThreshold, Title: "100%", DedupKey: "th:r:5:100",
		Priority: PriCritical,
	})
	if r != SkipNone {
		t.Fatalf("priority 9 should bypass quiet hours, got %v", r)
	}
}

func TestComposer_DailyCapEnforced(t *testing.T) {
	d := inProcDB(t)
	seedRule(t, d, CatPurchase, ruleOpts{enabled: true, maxPerDay: 2})
	send := &capturingSender{}
	c := NewComposer(d, send)
	c.Now = func() time.Time { return time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC) }

	for i := 0; i < 3; i++ {
		ev := Event{Category: CatPurchase, Title: "p", DedupKey: "p:" + sint(i)}
		_, r, _ := c.Fire(context.Background(), ev)
		if i < 2 && r != SkipNone {
			t.Fatalf("expected first 2 to send, got %v on i=%d", r, i)
		}
		if i == 2 && r != SkipDailyCap {
			t.Fatalf("expected SkipDailyCap on 3rd, got %v", r)
		}
	}
}

func TestComposer_RuleDisabledSkips(t *testing.T) {
	d := inProcDB(t)
	seedRule(t, d, CatPurchase, ruleOpts{enabled: false, maxPerDay: 10})
	send := &capturingSender{}
	c := NewComposer(d, send)
	c.Now = func() time.Time { return time.Now() }

	_, r, _ := c.Fire(context.Background(), Event{
		Category: CatPurchase, Title: "x", DedupKey: "x:1",
	})
	if r != SkipDisabled {
		t.Fatalf("expected SkipDisabled, got %v", r)
	}
}

func TestInQuietHours_Wrapping(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"before-start", clock(21, 0), false},
		{"at-start",     clock(22, 0), true},
		{"mid-night",    clock(2, 0),  true},
		{"at-end",       clock(7, 0),  false},
		{"after-end",    clock(7, 30), false},
		{"noon",         clock(12, 0), false},
	}
	for _, c := range cases {
		got := inQuietHours(c.now, "22:00", "07:00")
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestInQuietHours_NonWrapping(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"before", clock(8, 30), false},
		{"at-start", clock(9, 0), true},
		{"middle", clock(10, 30), true},
		{"at-end", clock(11, 0), false},
		{"after", clock(11, 30), false},
	}
	for _, c := range cases {
		got := inQuietHours(c.now, "09:00", "11:00")
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func clock(h, m int) time.Time { return time.Date(2026, 5, 12, h, m, 0, 0, time.UTC) }
func sint(i int) string { return string(rune('a' + i)) }
