package scheduler

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/brayden/northstar-server/internal/db"
	"github.com/brayden/northstar-server/internal/goals"
	"github.com/brayden/northstar-server/internal/health"
	"github.com/brayden/northstar-server/internal/notify"
	_ "modernc.org/sqlite"
)

func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "sched.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Seed the singleton user with default UTC settings so the brief/retro
	// time-of-day match works against the times we pass.
	_, _ = d.Exec(`INSERT INTO users (id, email, settings_json, created_at, updated_at)
	               VALUES ('u', '', ?, 0, 0)`,
		`{"daily_brief_time":"07:00","evening_retro_time":"21:00","timezone":"UTC"}`)
	t.Cleanup(func() { d.Close() })
	return d
}

type captureSender struct {
	count int
	cats  []string
}

func (c *captureSender) Send(_ context.Context, n notify.PreparedNotification) error {
	c.count++
	c.cats = append(c.cats, n.Category)
	return nil
}
func (*captureSender) Mode() string { return "test" }

func newScheduler(t *testing.T, d *sql.DB, now time.Time) (*Scheduler, *captureSender) {
	t.Helper()
	send := &captureSender{}
	c := notify.NewComposer(d, send)
	c.Now = func() time.Time { return now }
	s := New(d, c, goals.NewHandlers(d))
	s.Now = func() time.Time { return now }
	return s, send
}

// ─── Daily brief / evening retro ─────────────────────────────────────────

func TestTick_DailyBriefFires_AtUserTime(t *testing.T) {
	d := freshDB(t)
	at := time.Date(2026, 5, 12, 7, 0, 0, 0, time.UTC)
	s, send := newScheduler(t, d, at)

	s.Tick(context.Background())

	if send.count != 1 || send.cats[0] != notify.CatDailyBrief {
		t.Errorf("expected one daily_brief, got count=%d cats=%v", send.count, send.cats)
	}
}

func TestTick_DailyBriefDoesNotFire_OffMinute(t *testing.T) {
	d := freshDB(t)
	at := time.Date(2026, 5, 12, 7, 1, 0, 0, time.UTC) // one minute past
	s, send := newScheduler(t, d, at)

	s.Tick(context.Background())

	if send.count != 0 {
		t.Errorf("expected no fire off-minute, got count=%d cats=%v", send.count, send.cats)
	}
}

func TestTick_EveningRetroFires(t *testing.T) {
	d := freshDB(t)
	at := time.Date(2026, 5, 12, 21, 0, 0, 0, time.UTC)
	s, send := newScheduler(t, d, at)

	s.Tick(context.Background())

	if send.count != 1 || send.cats[0] != notify.CatEveningRetro {
		t.Errorf("expected one evening_retro, got count=%d cats=%v", send.count, send.cats)
	}
}

// ─── Reminders ───────────────────────────────────────────────────────────

func TestTick_ReminderFiresAndRollsForward(t *testing.T) {
	d := freshDB(t)
	at := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC) // not a brief/retro minute
	// A reminder due 1s ago, recurrence: every day at 14:00
	_, err := d.Exec(`INSERT INTO goal_reminders
	    (id, title, body, recurrence, next_fires_at, active, created_at)
	    VALUES ('r1', 'Daily review', '', '0 14 * * *', ?, 1, 0)`,
		at.Add(-time.Second).Unix())
	if err != nil {
		t.Fatalf("seed reminder: %v", err)
	}

	s, send := newScheduler(t, d, at)
	s.Tick(context.Background())

	// goal_milestone category rides the reminder channel per scheduler.go.
	if send.count != 1 || send.cats[0] != notify.CatGoalMilestone {
		t.Errorf("expected one reminder fire, got count=%d cats=%v", send.count, send.cats)
	}

	// next_fires_at should have moved forward into the future.
	var next int64
	_ = d.QueryRow(`SELECT next_fires_at FROM goal_reminders WHERE id = 'r1'`).Scan(&next)
	if next <= at.Unix() {
		t.Errorf("next_fires_at didn't roll forward: %d <= %d", next, at.Unix())
	}
}

func TestTick_ReminderDoesNotFire_WhenFuture(t *testing.T) {
	d := freshDB(t)
	at := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	// Due in 1 hour
	_, _ = d.Exec(`INSERT INTO goal_reminders
	    (id, title, body, recurrence, next_fires_at, active, created_at)
	    VALUES ('r1', 't', '', '0 15 * * *', ?, 1, 0)`,
		at.Add(time.Hour).Unix())

	s, send := newScheduler(t, d, at)
	s.Tick(context.Background())

	if send.count != 0 {
		t.Errorf("expected no fire when reminder is in the future, got %d", send.count)
	}
}

func TestTick_InactiveReminder_DoesNotFire(t *testing.T) {
	d := freshDB(t)
	at := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	_, _ = d.Exec(`INSERT INTO goal_reminders
	    (id, title, body, recurrence, next_fires_at, active, created_at)
	    VALUES ('r1', 't', '', '0 14 * * *', ?, 0, 0)`,
		at.Add(-time.Second).Unix())

	s, send := newScheduler(t, d, at)
	s.Tick(context.Background())

	if send.count != 0 {
		t.Errorf("inactive reminder fired anyway: %d", send.count)
	}
}

// ─── Supplements ─────────────────────────────────────────────────────────

func TestTick_SupplementChecksRun_WhenWithSupplementsSet(t *testing.T) {
	d := freshDB(t)
	at := time.Date(2026, 5, 12, 7, 30, 0, 0, time.UTC) // off the brief minute
	// Active supplement scheduled at 07:30
	_, _ = d.Exec(`INSERT INTO health_supplement_defs
	    (id, name, dose, category, schedule_json, reminder_enabled, active, created_at)
	    VALUES ('s1', 'BPC-157', '500mcg', 'peptide', '{"times":["07:30"]}', 1, 1, ?)`,
		at.Add(-24*time.Hour).Unix())

	send := &captureSender{}
	c := notify.NewComposer(d, send)
	c.Now = func() time.Time { return at }
	s := New(d, c, goals.NewHandlers(d)).
		WithSupplements(health.NewSupplementReminders(d, c))
	s.Now = func() time.Time { return at }

	s.Tick(context.Background())

	if send.count != 1 || send.cats[0] != notify.CatSupplement {
		t.Errorf("expected one supplement fire, got count=%d cats=%v", send.count, send.cats)
	}
}

func TestTick_SupplementChecksSkipped_WhenNotWired(t *testing.T) {
	d := freshDB(t)
	at := time.Date(2026, 5, 12, 7, 30, 0, 0, time.UTC)
	_, _ = d.Exec(`INSERT INTO health_supplement_defs
	    (id, name, dose, category, schedule_json, reminder_enabled, active, created_at)
	    VALUES ('s1', 'BPC-157', '500mcg', 'peptide', '{"times":["07:30"]}', 1, 1, ?)`,
		at.Add(-24*time.Hour).Unix())

	s, send := newScheduler(t, d, at) // NO WithSupplements
	s.Tick(context.Background())

	if send.count != 0 {
		t.Errorf("supplement should not fire without WithSupplements, got %d", send.count)
	}
}

// ─── Nil-Goals safety ────────────────────────────────────────────────────

func TestTick_NilGoals_DoesNotCrash(t *testing.T) {
	d := freshDB(t)
	at := time.Date(2026, 5, 12, 7, 0, 0, 0, time.UTC) // would be brief time
	send := &captureSender{}
	c := notify.NewComposer(d, send)
	c.Now = func() time.Time { return at }
	// Goals is nil — Health-only deployment shape
	s := New(d, c, nil).
		WithSupplements(health.NewSupplementReminders(d, c))
	s.Now = func() time.Time { return at }

	// Must not panic; just runs Supplements (none seeded here so no fires).
	s.Tick(context.Background())

	if send.count != 0 {
		t.Errorf("nothing should fire with nil Goals + no supplements: %d", send.count)
	}
}
