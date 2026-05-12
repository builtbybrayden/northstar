// Package scheduler ticks once a minute and fires:
//   • daily_brief at the user's morning time
//   • evening_retro at the user's evening time
//   • any goal_reminders whose next_fires_at <= now
//
// All firings go through the notify.Composer so dedup, quiet hours, and
// rule disable work uniformly. Each fire writes a sentinel dedup_key so we
// don't double-fire on a server restart inside the same minute.
package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/builtbybrayden/northstar/server/internal/goals"
	"github.com/builtbybrayden/northstar/server/internal/health"
	"github.com/builtbybrayden/northstar/server/internal/notify"
)

type Scheduler struct {
	DB          *sql.DB
	Notifier    *notify.Composer
	Goals       *goals.Handlers
	Supplements *health.SupplementReminders // optional; nil disables supplement reminders
	Now         func() time.Time
}

func New(db *sql.DB, n *notify.Composer, g *goals.Handlers) *Scheduler {
	return &Scheduler{DB: db, Notifier: n, Goals: g, Now: time.Now}
}

// WithSupplements enables supplement-reminder ticking.
func (s *Scheduler) WithSupplements(r *health.SupplementReminders) *Scheduler {
	s.Supplements = r
	return s
}

// Run loops until ctx is cancelled. Ticks every minute aligned to the wall clock.
func (s *Scheduler) Run(ctx context.Context) {
	log.Printf("scheduler: starting")
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()

	// Initial tick so we don't wait a minute on first start
	s.Tick(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Printf("scheduler: stopping")
			return
		case <-t.C:
			s.Tick(ctx)
		}
	}
}

// Tick runs all scheduled checks once. Public for testing.
func (s *Scheduler) Tick(ctx context.Context) {
	now := s.Now()

	if s.Goals != nil {
		if err := s.checkBriefAndRetro(ctx, now); err != nil {
			log.Printf("scheduler: brief/retro check failed: %v", err)
		}
		if err := s.checkReminders(ctx, now); err != nil {
			log.Printf("scheduler: reminder check failed: %v", err)
		}
	}
	if s.Supplements != nil {
		loc, err := s.loadUserLocation(ctx)
		if err != nil {
			log.Printf("scheduler: tz load failed: %v", err)
			loc = time.UTC
		}
		if err := s.Supplements.Check(ctx, now, loc); err != nil {
			log.Printf("scheduler: supplement check failed: %v", err)
		}
	}
}

func (s *Scheduler) loadUserLocation(ctx context.Context) (*time.Location, error) {
	settings, err := s.loadUserSettings(ctx)
	if err != nil {
		return time.UTC, err
	}
	loc, err := time.LoadLocation(settings["timezone"])
	if err != nil {
		return time.UTC, nil
	}
	return loc, nil
}

// ─── Daily brief / evening retro ─────────────────────────────────────────

func (s *Scheduler) checkBriefAndRetro(ctx context.Context, now time.Time) error {
	settings, err := s.loadUserSettings(ctx)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(settings["timezone"])
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)

	hhmm := local.Format("15:04")

	if hhmm == settings["daily_brief_time"] {
		if err := s.fireBrief(ctx, now); err != nil {
			return err
		}
	}
	if hhmm == settings["evening_retro_time"] {
		if err := s.fireRetro(ctx, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) fireBrief(ctx context.Context, now time.Time) error {
	brief, err := s.Goals.ComposeBrief(ctx, now)
	if err != nil {
		return err
	}

	title := "Today's brief"
	open := 0
	for _, it := range brief.Items {
		if !it.Done {
			open++
		}
	}
	body := fmt.Sprintf("%d open · streak %d", open, brief.StreakCount)
	if len(brief.MilestonesDueSoon) > 0 {
		body += fmt.Sprintf(" · %d milestone(s) due in 7d", len(brief.MilestonesDueSoon))
	}

	payload, _ := json.Marshal(brief)
	var payloadMap map[string]any
	_ = json.Unmarshal(payload, &payloadMap)

	_, _, err = s.Notifier.Fire(ctx, notify.Event{
		Category: notify.CatDailyBrief,
		Priority: notify.PriNormal,
		Title:    title,
		Body:     body,
		DedupKey: "daily_brief:" + now.UTC().Format("2006-01-02"),
		Payload:  payloadMap,
	})
	return err
}

func (s *Scheduler) fireRetro(ctx context.Context, now time.Time) error {
	// Check if today's daily log is empty — that's the prompt.
	today := now.UTC().Format("2006-01-02")
	var raw string
	err := s.DB.QueryRowContext(ctx,
		`SELECT COALESCE(items_json,'[]') FROM goal_daily_log WHERE date = ?`, today).Scan(&raw)
	hasItems := err == nil && raw != "[]" && raw != ""

	title := "Log today"
	body := "Add today's wins and what's pending — keeps the streak alive."
	if hasItems {
		title = "Evening retro"
		body = "Wrap the day with a quick reflection."
	}
	_, _, err = s.Notifier.Fire(ctx, notify.Event{
		Category: notify.CatEveningRetro,
		Priority: notify.PriNormal,
		Title:    title,
		Body:     body,
		DedupKey: "evening_retro:" + today,
	})
	return err
}

// ─── Reminders ────────────────────────────────────────────────────────────

func (s *Scheduler) checkReminders(ctx context.Context, now time.Time) error {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, title, COALESCE(body,''), recurrence, COALESCE(next_fires_at,0)
		   FROM goal_reminders
		  WHERE active = 1 AND next_fires_at IS NOT NULL AND next_fires_at <= ?`,
		now.Unix())
	if err != nil {
		return err
	}
	defer rows.Close()

	type due struct {
		id, title, body, recurrence string
		fires                       int64
	}
	var fires []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.title, &d.body, &d.recurrence, &d.fires); err != nil {
			return err
		}
		fires = append(fires, d)
	}

	for _, d := range fires {
		_, _, err := s.Notifier.Fire(ctx, notify.Event{
			Category: notify.CatGoalMilestone, // reminders ride this rule channel
			Priority: notify.PriNormal,
			Title:    d.title,
			Body:     d.body,
			DedupKey: fmt.Sprintf("reminder:%s:%d", d.id, d.fires),
			Payload: map[string]any{
				"reminder_id": d.id,
				"fires_at":    d.fires,
			},
		})
		if err != nil {
			log.Printf("scheduler: fire reminder %s failed: %v", d.id, err)
			continue
		}
		// Compute next fire and persist
		next, err := goals.NextFiresAt(d.recurrence, now)
		if err != nil {
			log.Printf("scheduler: cron parse failed for %s: %v", d.id, err)
			continue
		}
		if _, err := s.DB.ExecContext(ctx,
			`UPDATE goal_reminders SET next_fires_at = ? WHERE id = ?`,
			next.Unix(), d.id); err != nil {
			log.Printf("scheduler: update next_fires_at failed: %v", err)
		}
	}
	return nil
}

// ─── Settings loader ─────────────────────────────────────────────────────

func (s *Scheduler) loadUserSettings(ctx context.Context) (map[string]string, error) {
	out := map[string]string{
		"daily_brief_time":   "07:30",
		"evening_retro_time": "21:00",
		"timezone":           "UTC",
	}
	var raw string
	err := s.DB.QueryRowContext(ctx,
		`SELECT COALESCE(settings_json, '{}') FROM users LIMIT 1`).Scan(&raw)
	if err != nil {
		return out, nil // single-user model — no rows = use defaults
	}
	parsed := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return out, nil
	}
	for k, v := range parsed {
		if vs, ok := v.(string); ok && vs != "" {
			out[k] = vs
		}
	}
	return out, nil
}
