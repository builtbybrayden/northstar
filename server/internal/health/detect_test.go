package health

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/builtbybrayden/northstar/server/internal/notify"
	_ "modernc.org/sqlite"
)

// inProcDB spins up an in-memory SQLite with just enough schema for the
// detector + notification engine to operate.
func inProcDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	stmts := []string{
		`CREATE TABLE health_recovery (
		   date TEXT PRIMARY KEY, score INTEGER, hrv_ms REAL, rhr INTEGER,
		   source TEXT NOT NULL DEFAULT 'whoop')`,
		`CREATE TABLE health_sleep (
		   date TEXT PRIMARY KEY, duration_min INTEGER, score INTEGER,
		   debt_min INTEGER, source TEXT NOT NULL DEFAULT 'whoop')`,
		`CREATE TABLE health_strain (
		   date TEXT PRIMARY KEY, score REAL, avg_hr INTEGER, max_hr INTEGER,
		   source TEXT NOT NULL DEFAULT 'whoop')`,
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
		// Seed the health_insight rule
		`INSERT INTO notif_rules
		   (id, category, enabled, bypass_quiet, max_per_day, updated_at)
		 VALUES ('seed-h', 'health_insight', 1, 0, 99, 0)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return d
}

type captureSender struct{ count int }

func (c *captureSender) Send(_ context.Context, _ notify.PreparedNotification) error {
	c.count++
	return nil
}
func (*captureSender) Mode() string { return "test" }

func TestCheckRecoveryDrop_Fires(t *testing.T) {
	d := inProcDB(t)
	// Yesterday 84, today 28 → drop of 56 > 30 threshold
	_, _ = d.Exec(`INSERT INTO health_recovery (date, score, source) VALUES ('2026-05-11', 84, 'whoop')`)
	_, _ = d.Exec(`INSERT INTO health_recovery (date, score, source) VALUES ('2026-05-12', 28, 'whoop')`)

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)

	if err := det.CheckRecoveryDrop(context.Background()); err != nil {
		t.Fatalf("CheckRecoveryDrop: %v", err)
	}
	if send.count != 1 {
		t.Fatalf("expected 1 send, got %d", send.count)
	}
}

func TestCheckRecoveryDrop_NoFireOnSmallDrop(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_recovery (date, score, source) VALUES ('2026-05-11', 84, 'whoop')`)
	_, _ = d.Exec(`INSERT INTO health_recovery (date, score, source) VALUES ('2026-05-12', 72, 'whoop')`) // -12

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)
	_ = det.CheckRecoveryDrop(context.Background())
	if send.count != 0 {
		t.Fatalf("expected 0 sends (drop < 30), got %d", send.count)
	}
}

func TestCheckRecoveryDrop_NoFireOnInsufficientHistory(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_recovery (date, score, source) VALUES ('2026-05-12', 28, 'whoop')`)

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)
	_ = det.CheckRecoveryDrop(context.Background())
	if send.count != 0 {
		t.Fatalf("expected 0 sends (only 1 day in history), got %d", send.count)
	}
}

func TestCheckRecoveryDrop_DedupSecondCall(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_recovery (date, score, source) VALUES ('2026-05-11', 84, 'whoop')`)
	_, _ = d.Exec(`INSERT INTO health_recovery (date, score, source) VALUES ('2026-05-12', 28, 'whoop')`)

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	composer.Now = func() time.Time { return time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC) }
	det := NewDetector(d, composer)
	_ = det.CheckRecoveryDrop(context.Background())
	_ = det.CheckRecoveryDrop(context.Background())
	if send.count != 1 {
		t.Fatalf("expected dedup to keep send at 1, got %d", send.count)
	}
}

// ─── Sleep detector ──────────────────────────────────────────────────────

func TestCheckSleepIssues_FiresOnDebt(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_sleep (date, duration_min, score, debt_min)
	               VALUES ('2026-05-12', 360, 70, 120)`) // 120 min debt > 90

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)
	if err := det.CheckSleepIssues(context.Background()); err != nil {
		t.Fatalf("CheckSleepIssues: %v", err)
	}
	if send.count != 1 {
		t.Fatalf("expected 1 send for sleep debt, got %d", send.count)
	}
}

func TestCheckSleepIssues_FiresOnLowScore(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_sleep (date, duration_min, score, debt_min)
	               VALUES ('2026-05-12', 360, 42, 30)`) // score 42 < 50

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)
	_ = det.CheckSleepIssues(context.Background())
	if send.count != 1 {
		t.Fatalf("expected 1 send for low sleep score, got %d", send.count)
	}
}

func TestCheckSleepIssues_FiresBothOnSameDay(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_sleep (date, duration_min, score, debt_min)
	               VALUES ('2026-05-12', 300, 40, 150)`) // both rules trip

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)
	_ = det.CheckSleepIssues(context.Background())
	// Two distinct dedup keys → both fire
	if send.count != 2 {
		t.Fatalf("expected 2 sends (debt + score), got %d", send.count)
	}
}

func TestCheckSleepIssues_NoFireOnHealthyRow(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_sleep (date, duration_min, score, debt_min)
	               VALUES ('2026-05-12', 480, 88, 0)`)

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)
	_ = det.CheckSleepIssues(context.Background())
	if send.count != 0 {
		t.Fatalf("expected 0 sends on healthy sleep row, got %d", send.count)
	}
}

func TestCheckSleepIssues_DedupOnSecondCall(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_sleep (date, duration_min, score, debt_min)
	               VALUES ('2026-05-12', 360, 70, 120)`)

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)
	_ = det.CheckSleepIssues(context.Background())
	_ = det.CheckSleepIssues(context.Background())
	if send.count != 1 {
		t.Fatalf("expected dedup to hold at 1, got %d", send.count)
	}
}

// ─── Overreach detector ──────────────────────────────────────────────────

func TestCheckOverreach_Fires(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_recovery (date, score) VALUES ('2026-05-12', 35)`)
	_, _ = d.Exec(`INSERT INTO health_strain   (date, score) VALUES ('2026-05-12', 18.5)`)

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)
	if err := det.CheckOverreach(context.Background()); err != nil {
		t.Fatalf("CheckOverreach: %v", err)
	}
	if send.count != 1 {
		t.Fatalf("expected overreach fire, got %d sends", send.count)
	}
}

func TestCheckOverreach_NoFireOnHighRecovery(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_recovery (date, score) VALUES ('2026-05-12', 80)`)
	_, _ = d.Exec(`INSERT INTO health_strain   (date, score) VALUES ('2026-05-12', 19.0)`)

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)
	_ = det.CheckOverreach(context.Background())
	if send.count != 0 {
		t.Fatalf("expected no fire when recovery healthy, got %d", send.count)
	}
}

func TestCheckOverreach_NoFireOnLowStrain(t *testing.T) {
	d := inProcDB(t)
	_, _ = d.Exec(`INSERT INTO health_recovery (date, score) VALUES ('2026-05-12', 30)`)
	_, _ = d.Exec(`INSERT INTO health_strain   (date, score) VALUES ('2026-05-12', 8.0)`)

	send := &captureSender{}
	composer := notify.NewComposer(d, send)
	det := NewDetector(d, composer)
	_ = det.CheckOverreach(context.Background())
	if send.count != 0 {
		t.Fatalf("expected no fire when strain low, got %d", send.count)
	}
}
