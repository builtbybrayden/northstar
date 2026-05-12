package goals

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ComposeBrief builds the daily brief for the given `now` time. It pulls:
//   1. Today's daily log items (if the user has already started it)
//   2. Active reminders that fire today (next_fires_at within today)
//   3. Milestones due in the next 7 days
//   4. Incomplete items from yesterday's daily log, rolled over with source=rollover
func (h *Handlers) ComposeBrief(ctx context.Context, now time.Time) (Brief, error) {
	today := now.UTC().Format("2006-01-02")
	yesterday := now.UTC().AddDate(0, 0, -1).Format("2006-01-02")

	// 1. Today's items (already-logged manual entries)
	var todayItems []DailyItem
	var streak int
	var raw string
	err := h.DB.QueryRowContext(ctx,
		`SELECT COALESCE(items_json,'[]'), streak_count FROM goal_daily_log WHERE date = ?`,
		today).Scan(&raw, &streak)
	if err == nil {
		todayItems = decodeItems(raw)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Brief{}, err
	}

	// 2. Yesterday's incomplete items — roll over (skip if already present today)
	seen := map[string]bool{}
	for _, it := range todayItems {
		seen[it.SourceRef+":"+it.Text] = true
	}
	var yRaw string
	err = h.DB.QueryRowContext(ctx,
		`SELECT COALESCE(items_json,'[]') FROM goal_daily_log WHERE date = ?`, yesterday).
		Scan(&yRaw)
	if err == nil {
		for _, it := range decodeItems(yRaw) {
			if it.Done {
				continue
			}
			key := it.SourceRef + ":" + it.Text
			if seen[key] {
				continue
			}
			rolled := it
			rolled.Done = false
			rolled.Source = "rollover"
			todayItems = append(todayItems, rolled)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Brief{}, err
	}

	// 3. Active reminders firing today
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	reminderRows, err := h.DB.QueryContext(ctx,
		`SELECT id, title, COALESCE(body,''), recurrence, COALESCE(next_fires_at,0), active, created_at
		   FROM goal_reminders
		  WHERE active = 1 AND next_fires_at IS NOT NULL AND next_fires_at <= ?
		  ORDER BY next_fires_at ASC`, endOfDay.Unix())
	if err != nil {
		return Brief{}, err
	}
	defer reminderRows.Close()
	reminders := []Reminder{}
	for reminderRows.Next() {
		var rm Reminder
		var active int
		if err := reminderRows.Scan(&rm.ID, &rm.Title, &rm.Body, &rm.Recurrence,
			&rm.NextFiresAt, &active, &rm.CreatedAt); err != nil {
			return Brief{}, err
		}
		rm.Active = active == 1
		reminders = append(reminders, rm)
		// Inject as a daily-brief item if not already
		key := "reminder:" + rm.ID + ":" + rm.Title
		if !seen["reminder:"+rm.ID+":"+rm.Title] {
			todayItems = append(todayItems, DailyItem{
				ID:        "rem-" + rm.ID,
				Text:      rm.Title,
				Done:      false,
				Source:    "reminder",
				SourceRef: rm.ID,
			})
			seen[key] = true
		}
	}

	// 4. Milestones due in next 7 days
	weekOut := now.AddDate(0, 0, 7).Format("2006-01-02")
	msRows, err := h.DB.QueryContext(ctx,
		`SELECT id, title, COALESCE(description_md,''), COALESCE(due_date,''),
		        status, flagship, display_order, created_at, updated_at
		   FROM goal_milestones
		  WHERE status NOT IN ('done','archived')
		    AND due_date IS NOT NULL AND due_date != ''
		    AND due_date <= ?
		  ORDER BY due_date ASC`, weekOut)
	if err != nil {
		return Brief{}, err
	}
	defer msRows.Close()
	milestones := []Milestone{}
	for msRows.Next() {
		var m Milestone
		var flagship int
		if err := msRows.Scan(&m.ID, &m.Title, &m.DescriptionMD, &m.DueDate,
			&m.Status, &flagship, &m.DisplayOrder, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return Brief{}, err
		}
		m.Flagship = flagship == 1
		milestones = append(milestones, m)
	}

	return Brief{
		Date:              today,
		Items:             todayItems,
		StreakCount:       streak,
		MilestonesDueSoon: milestones,
		ActiveReminders:   reminders,
	}, nil
}
