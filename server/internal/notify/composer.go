package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// Composer evaluates rules, dedupes, persists, and dispatches.
type Composer struct {
	DB     *sql.DB
	Sender Sender
	Hub    *Hub             // optional — fanout to live SSE subscribers
	Now    func() time.Time // injectable for tests
}

func NewComposer(db *sql.DB, sender Sender) *Composer {
	return &Composer{DB: db, Sender: sender, Now: time.Now}
}

// WithHub attaches a fanout hub for live notification streaming.
func (c *Composer) WithHub(h *Hub) *Composer {
	c.Hub = h
	return c
}

// Fire evaluates the event against the user's rule, dedups via dedup_key, and
// (if allowed) persists + sends. Returns the created notification ID, or empty
// string if skipped, along with a SkipReason explaining why if skipped.
func (c *Composer) Fire(ctx context.Context, ev Event) (string, SkipReason, error) {
	if ev.Category == "" {
		return "", SkipUnknownCategory, errors.New("event.Category required")
	}
	if ev.Priority == 0 {
		ev.Priority = PriNormal
	}

	now := c.Now()

	// 1. Dedup: if a notification with this dedup_key already exists, skip.
	if ev.DedupKey != "" {
		var existing string
		err := c.DB.QueryRowContext(ctx,
			`SELECT id FROM notifications WHERE dedup_key = ?`, ev.DedupKey,
		).Scan(&existing)
		if err == nil {
			return "", SkipDedup, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", SkipNone, fmt.Errorf("dedup lookup: %w", err)
		}
	}

	// 2. Rule lookup
	rule, err := c.loadRule(ctx, ev.Category)
	if err != nil {
		return "", SkipNone, err
	}
	if !rule.Enabled {
		return "", SkipDisabled, nil
	}

	// 3. Quiet hours (unless priority bypasses or rule has bypass_quiet)
	bypass := rule.BypassQuiet || ev.Priority >= 8
	if !bypass && inQuietHours(now, rule.QuietHoursStart, rule.QuietHoursEnd) {
		return "", SkipQuietHours, nil
	}

	// 4. Daily cap
	if rule.MaxPerDay > 0 {
		count, err := c.todayCount(ctx, ev.Category, now)
		if err != nil {
			return "", SkipNone, err
		}
		if count >= rule.MaxPerDay {
			return "", SkipDailyCap, nil
		}
	}

	// 5. Persist
	id := uuid.NewString()
	payloadJSON := "{}"
	if ev.Payload != nil {
		b, err := json.Marshal(ev.Payload)
		if err != nil {
			return "", SkipNone, err
		}
		payloadJSON = string(b)
	}
	var dedupKey any
	if ev.DedupKey != "" {
		dedupKey = ev.DedupKey
	}
	_, err = c.DB.ExecContext(ctx,
		`INSERT INTO notifications (id, category, title, body, payload_json, priority, dedup_key, created_at, delivery_status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
		id, ev.Category, ev.Title, ev.Body, payloadJSON, ev.Priority, dedupKey, now.Unix())
	if err != nil {
		return "", SkipNone, fmt.Errorf("persist notification: %w", err)
	}

	// 6. Bump daily count
	_, _ = c.DB.ExecContext(ctx,
		`INSERT INTO notif_daily_counts (category, date, count) VALUES (?, ?, 1)
		 ON CONFLICT(category, date) DO UPDATE SET count = count + 1`,
		ev.Category, now.UTC().Format("2006-01-02"))

	// 7. Dispatch via sender
	prep := PreparedNotification{
		ID:        id,
		Category:  ev.Category,
		Title:     ev.Title,
		Body:      ev.Body,
		Priority:  ev.Priority,
		Payload:   ev.Payload,
		CreatedAt: now,
	}
	// APNs tokens come from devices table — silent push-only if we have them
	tokens, err := c.deviceTokens(ctx)
	if err != nil {
		log.Printf("notify: device token fetch failed: %v", err)
	} else {
		prep.APNSTokens = tokens
	}

	status := "sent"
	if err := c.Sender.Send(ctx, prep); err != nil {
		log.Printf("notify: send failed: %v", err)
		status = "failed"
	}
	_, _ = c.DB.ExecContext(ctx,
		`UPDATE notifications SET delivered_at = ?, delivery_status = ? WHERE id = ?`,
		now.Unix(), status, id)

	// 8. Fan out to live SSE subscribers (open iOS apps).
	if c.Hub != nil {
		c.Hub.Publish(prep)
	}

	return id, SkipNone, nil
}

// loadRule reads notif_rules. If the row is missing (shouldn't happen — seed
// inserts cover every category — defensive), returns a defaulted Rule.
func (c *Composer) loadRule(ctx context.Context, category string) (Rule, error) {
	var r Rule
	var enabled, bypass int
	var quietStart, quietEnd sql.NullString
	r.Category = category
	err := c.DB.QueryRowContext(ctx,
		`SELECT enabled, quiet_hours_start, quiet_hours_end, bypass_quiet, delivery, max_per_day
		   FROM notif_rules WHERE category = ?`, category).
		Scan(&enabled, &quietStart, &quietEnd, &bypass, &r.Delivery, &r.MaxPerDay)
	if errors.Is(err, sql.ErrNoRows) {
		return Rule{Category: category, Enabled: true, Delivery: "push", MaxPerDay: 99}, nil
	}
	if err != nil {
		return r, err
	}
	r.Enabled = enabled == 1
	r.BypassQuiet = bypass == 1
	if quietStart.Valid {
		r.QuietHoursStart = quietStart.String
	}
	if quietEnd.Valid {
		r.QuietHoursEnd = quietEnd.String
	}
	return r, nil
}

func (c *Composer) todayCount(ctx context.Context, category string, now time.Time) (int, error) {
	var n int
	err := c.DB.QueryRowContext(ctx,
		`SELECT COALESCE(count, 0) FROM notif_daily_counts WHERE category = ? AND date = ?`,
		category, now.UTC().Format("2006-01-02")).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

func (c *Composer) deviceTokens(ctx context.Context) ([]string, error) {
	rows, err := c.DB.QueryContext(ctx,
		`SELECT apns_token FROM devices
		  WHERE apns_token IS NOT NULL AND apns_token != ''
		    AND (revoked_at IS NULL OR revoked_at = 0)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// inQuietHours returns true if `now` falls within [start, end) where start/end
// are "HH:MM" strings in local time. Handles wrap-around (22:00 → 07:00).
func inQuietHours(now time.Time, start, end string) bool {
	if start == "" || end == "" {
		return false
	}
	hm := func(s string) int {
		var h, m int
		fmt.Sscanf(s, "%d:%d", &h, &m)
		return h*60 + m
	}
	mins := now.Hour()*60 + now.Minute()
	s := hm(start)
	e := hm(end)
	if s == e {
		return false
	}
	if s < e {
		return mins >= s && mins < e
	}
	// Wraps midnight, e.g. 22:00 → 07:00
	return mins >= s || mins < e
}
