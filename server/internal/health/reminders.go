package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/brayden/northstar-server/internal/notify"
)

// SupplementSchedule is the shape persisted in health_supplement_defs.schedule_json.
// Forward-tolerant parser: also accepts a bare ["HH:MM", ...] array.
//
//	{"times": ["07:00", "19:00"], "days": ["mon","tue","wed","thu","fri"]}
//
// `days` is optional; absent or empty means every day. Day strings are
// lowercased three-letter abbreviations matching time.Weekday.String()[:3].
type SupplementSchedule struct {
	Times []string `json:"times"`
	Days  []string `json:"days,omitempty"`
}

// ParseSchedule reads schedule_json into a normalized struct. Returns a zero
// value (no times) on parse failure rather than an error — a malformed
// schedule should never crash the scheduler tick.
func ParseSchedule(raw string) SupplementSchedule {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SupplementSchedule{}
	}
	// Try the object form first.
	var sched SupplementSchedule
	if err := json.Unmarshal([]byte(raw), &sched); err == nil && len(sched.Times) > 0 {
		sched.normalize()
		return sched
	}
	// Fall back to a bare array of times.
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err == nil && len(arr) > 0 {
		sched.Times = arr
		sched.normalize()
		return sched
	}
	return SupplementSchedule{}
}

func (s *SupplementSchedule) normalize() {
	for i, t := range s.Times {
		s.Times[i] = strings.TrimSpace(t)
	}
	for i, d := range s.Days {
		s.Days[i] = strings.ToLower(strings.TrimSpace(d))
	}
}

// FiresAt reports whether the schedule should fire at the given local time.
// Compares to wall-clock HH:MM and the weekday allowlist.
func (s SupplementSchedule) FiresAt(local time.Time) (matched string, ok bool) {
	if len(s.Times) == 0 {
		return "", false
	}
	if len(s.Days) > 0 {
		want := strings.ToLower(local.Weekday().String()[:3])
		dayMatch := false
		for _, d := range s.Days {
			if d == want {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			return "", false
		}
	}
	hhmm := local.Format("15:04")
	for _, t := range s.Times {
		if t == hhmm {
			return t, true
		}
	}
	return "", false
}

// InCycleOn reports whether `now` falls in an "on" window of an on/off cycle
// anchored to `anchor`. If either count is 0 it's treated as no cycling
// (always on).
func InCycleOn(anchor, now time.Time, daysOn, daysOff int) bool {
	if daysOn <= 0 || daysOff <= 0 {
		return true
	}
	// Days elapsed since anchor (floor at 0).
	hours := now.Sub(anchor).Hours()
	if hours < 0 {
		return true // future anchor — treat as always on
	}
	day := int(hours / 24)
	cycle := daysOn + daysOff
	return (day % cycle) < daysOn
}

// SupplementReminders fires supplement push notifications for any active def
// whose schedule includes the current minute (in the user's local timezone),
// gated by cycle. Idempotent within a minute via dedup keys.
type SupplementReminders struct {
	DB       *sql.DB
	Notifier *notify.Composer
}

func NewSupplementReminders(db *sql.DB, n *notify.Composer) *SupplementReminders {
	return &SupplementReminders{DB: db, Notifier: n}
}

// Check runs one pass. `now` is the wall-clock minute; `loc` is the user
// timezone. Errors are logged-and-skipped per def so one bad row can't kill
// the tick.
func (s *SupplementReminders) Check(ctx context.Context, now time.Time, loc *time.Location) error {
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)

	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, name, COALESCE(dose,''), COALESCE(category,'supplement'),
		        COALESCE(schedule_json,''),
		        COALESCE(cycle_days_on, 0), COALESCE(cycle_days_off, 0),
		        created_at
		   FROM health_supplement_defs
		  WHERE active = 1 AND reminder_enabled = 1`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type defRow struct {
		ID, Name, Dose, Category, ScheduleJSON string
		CycleOn, CycleOff                      int
		CreatedAt                              int64
	}
	var defs []defRow
	for rows.Next() {
		var d defRow
		if err := rows.Scan(&d.ID, &d.Name, &d.Dose, &d.Category, &d.ScheduleJSON,
			&d.CycleOn, &d.CycleOff, &d.CreatedAt); err != nil {
			log.Printf("health.reminders: scan failed: %v", err)
			continue
		}
		defs = append(defs, d)
	}

	for _, d := range defs {
		sched := ParseSchedule(d.ScheduleJSON)
		match, ok := sched.FiresAt(local)
		if !ok {
			continue
		}
		anchor := time.Unix(d.CreatedAt, 0)
		if !InCycleOn(anchor, now, d.CycleOn, d.CycleOff) {
			continue
		}
		body := d.Dose
		if d.Category == "medication" && body != "" {
			body = "Medication: " + body
		}
		_, _, err := s.Notifier.Fire(ctx, notify.Event{
			Category: notify.CatSupplement,
			Priority: notify.PriNormal,
			Title:    fmt.Sprintf("%s — %s", d.Name, match),
			Body:     body,
			DedupKey: fmt.Sprintf("supplement:%s:%s:%s", d.ID, local.Format("2006-01-02"), match),
			Payload: map[string]any{
				"def_id":   d.ID,
				"name":     d.Name,
				"dose":     d.Dose,
				"category": d.Category,
				"time":     match,
			},
		})
		if err != nil {
			log.Printf("health.reminders: fire failed for %s: %v", d.ID, err)
		}
	}
	return nil
}
