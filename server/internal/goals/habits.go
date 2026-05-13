package goals

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Habit is a definition; HabitEntry is one day's record for one habit.
type Habit struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DescriptionMD string `json:"description_md"`
	Color         string `json:"color"`
	TargetPerWeek int    `json:"target_per_week"`
	Active        bool   `json:"active"`
	DisplayOrder  int    `json:"display_order"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type HabitEntry struct {
	HabitID   string `json:"habit_id"`
	Date      string `json:"date"`
	Count     int    `json:"count"`
	Notes     string `json:"notes"`
	UpdatedAt int64  `json:"updated_at"`
}

// HabitWithStats is what the iOS list view consumes — definition + the last
// N days of entries inline so the heatmap renders without an extra round-trip.
type HabitWithStats struct {
	Habit
	StreakDays    int          `json:"streak_days"`
	DoneLast30    int          `json:"done_last_30"`
	Entries       []HabitEntry `json:"entries"`
}

// ─── /api/goals/habits ────────────────────────────────────────────────────

func (h *Handlers) ListHabits(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("inactive") == "1"
	q := `SELECT id, name, COALESCE(description_md,''), color, target_per_week,
	             active, display_order, created_at, updated_at
	        FROM goal_habits`
	if !includeInactive {
		q += ` WHERE active = 1`
	}
	q += ` ORDER BY active DESC, display_order ASC, created_at ASC`

	rows, err := h.DB.QueryContext(r.Context(), q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	var habits []Habit
	for rows.Next() {
		var hh Habit
		var active int
		if err := rows.Scan(&hh.ID, &hh.Name, &hh.DescriptionMD, &hh.Color,
			&hh.TargetPerWeek, &active, &hh.DisplayOrder,
			&hh.CreatedAt, &hh.UpdatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		hh.Active = active == 1
		habits = append(habits, hh)
	}

	// Default to a 90-day window so the iOS heatmap has ~13 weeks to render.
	days := 90
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	since := h.Now().AddDate(0, 0, -days).Format("2006-01-02")

	out := make([]HabitWithStats, 0, len(habits))
	for _, hh := range habits {
		entries, err := h.loadHabitEntries(r.Context(), hh.ID, since)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		streak, last30 := habitStats(entries, h.Now())
		out = append(out, HabitWithStats{
			Habit:       hh,
			StreakDays:  streak,
			DoneLast30:  last30,
			Entries:     entries,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type habitCreateInput struct {
	Name          string `json:"name"`
	DescriptionMD string `json:"description_md"`
	Color         string `json:"color"`
	TargetPerWeek int    `json:"target_per_week"`
}

func (h *Handlers) CreateHabit(w http.ResponseWriter, r *http.Request) {
	var in habitCreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeErrMsg(w, http.StatusBadRequest, "name required")
		return
	}
	if in.TargetPerWeek <= 0 || in.TargetPerWeek > 7 {
		in.TargetPerWeek = 7
	}
	if in.Color == "" {
		in.Color = "goals"
	}
	id := uuid.NewString()
	now := h.Now().Unix()
	if _, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO goal_habits
		   (id, name, description_md, color, target_per_week, active, display_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 1, 0, ?, ?)`,
		id, in.Name, in.DescriptionMD, in.Color, in.TargetPerWeek, now, now); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, Habit{
		ID: id, Name: in.Name, DescriptionMD: in.DescriptionMD,
		Color: in.Color, TargetPerWeek: in.TargetPerWeek, Active: true,
		CreatedAt: now, UpdatedAt: now,
	})
}

type habitUpdateInput struct {
	Name          *string `json:"name,omitempty"`
	DescriptionMD *string `json:"description_md,omitempty"`
	Color         *string `json:"color,omitempty"`
	TargetPerWeek *int    `json:"target_per_week,omitempty"`
	Active        *bool   `json:"active,omitempty"`
	DisplayOrder  *int    `json:"display_order,omitempty"`
}

func (h *Handlers) UpdateHabit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	var in habitUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	sets := []string{}
	args := []any{}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" {
			writeErrMsg(w, http.StatusBadRequest, "name cannot be blank")
			return
		}
		sets = append(sets, "name = ?")
		args = append(args, n)
	}
	if in.DescriptionMD != nil {
		sets = append(sets, "description_md = ?")
		args = append(args, *in.DescriptionMD)
	}
	if in.Color != nil {
		sets = append(sets, "color = ?")
		args = append(args, *in.Color)
	}
	if in.TargetPerWeek != nil {
		t := *in.TargetPerWeek
		if t < 1 {
			t = 1
		}
		if t > 7 {
			t = 7
		}
		sets = append(sets, "target_per_week = ?")
		args = append(args, t)
	}
	if in.Active != nil {
		sets = append(sets, "active = ?")
		v := 0
		if *in.Active {
			v = 1
		}
		args = append(args, v)
	}
	if in.DisplayOrder != nil {
		sets = append(sets, "display_order = ?")
		args = append(args, *in.DisplayOrder)
	}
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "noop": true})
		return
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, h.Now().Unix())

	q := "UPDATE goal_habits SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	args = append(args, id)
	res, err := h.DB.ExecContext(r.Context(), q, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrMsg(w, http.StatusNotFound, "habit not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handlers) DeleteHabit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	// Soft-delete: setting active=0 keeps history queryable but hides from
	// the default list. Use ?hard=1 to actually drop the rows.
	if r.URL.Query().Get("hard") == "1" {
		if _, err := h.DB.ExecContext(r.Context(),
			`DELETE FROM goal_habits WHERE id = ?`, id); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hard": true})
		return
	}
	if _, err := h.DB.ExecContext(r.Context(),
		`UPDATE goal_habits SET active = 0, updated_at = ? WHERE id = ?`,
		h.Now().Unix(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── /api/goals/habits/:id/log ────────────────────────────────────────────

type habitLogInput struct {
	Count int    `json:"count"`
	Notes string `json:"notes,omitempty"`
}

// PutHabitLog upserts the (habit, date) entry. Pass count=0 to record an
// explicit skip; passing no body (or count omitted) toggles between done
// (count=1) and skipped (count=0) so the iOS tap target stays simple.
func (h *Handlers) PutHabitLog(w http.ResponseWriter, r *http.Request) {
	habitID := chi.URLParam(r, "id")
	date := chi.URLParam(r, "date")
	if habitID == "" || date == "" {
		writeErrMsg(w, http.StatusBadRequest, "habit id and date required")
		return
	}

	// Sanity-check the habit exists so we return 404 cleanly instead of
	// silently writing orphan log rows. The FK isn't enforced unless
	// pragma foreign_keys = ON on the connection.
	var exists int
	if err := h.DB.QueryRowContext(r.Context(),
		`SELECT 1 FROM goal_habits WHERE id = ?`, habitID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			writeErrMsg(w, http.StatusNotFound, "habit not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	var in habitLogInput
	bodyEmpty := r.ContentLength == 0
	if !bodyEmpty {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeErrMsg(w, http.StatusBadRequest, "bad json")
			return
		}
	}

	now := h.Now().Unix()
	if bodyEmpty {
		// Toggle behavior — flip the current count.
		var cur int
		_ = h.DB.QueryRowContext(r.Context(),
			`SELECT count FROM goal_habit_log WHERE habit_id = ? AND date = ?`,
			habitID, date).Scan(&cur)
		newCount := 0
		if cur == 0 {
			newCount = 1
		}
		if _, err := h.DB.ExecContext(r.Context(),
			`INSERT INTO goal_habit_log (habit_id, date, count, notes, updated_at)
			 VALUES (?, ?, ?, '', ?)
			 ON CONFLICT(habit_id, date) DO UPDATE SET count = excluded.count, updated_at = excluded.updated_at`,
			habitID, date, newCount, now); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, HabitEntry{
			HabitID: habitID, Date: date, Count: newCount, UpdatedAt: now,
		})
		return
	}

	if in.Count < 0 {
		in.Count = 0
	}
	if _, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO goal_habit_log (habit_id, date, count, notes, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(habit_id, date) DO UPDATE SET
		   count = excluded.count, notes = excluded.notes, updated_at = excluded.updated_at`,
		habitID, date, in.Count, in.Notes, now); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, HabitEntry{
		HabitID: habitID, Date: date, Count: in.Count, Notes: in.Notes, UpdatedAt: now,
	})
}

// ─── helpers ──────────────────────────────────────────────────────────────

func (h *Handlers) loadHabitEntries(ctx context.Context, habitID, since string) ([]HabitEntry, error) {
	rows, err := h.DB.QueryContext(ctx,
		`SELECT date, count, COALESCE(notes,''), updated_at
		   FROM goal_habit_log
		  WHERE habit_id = ? AND date >= ?
		  ORDER BY date ASC`, habitID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HabitEntry{}
	for rows.Next() {
		var e HabitEntry
		e.HabitID = habitID
		if err := rows.Scan(&e.Date, &e.Count, &e.Notes, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// habitStats computes a current streak (consecutive days ending today where
// count > 0) and a count of done-days in the last 30 days.
func habitStats(entries []HabitEntry, now time.Time) (streak, last30 int) {
	byDate := map[string]int{}
	for _, e := range entries {
		byDate[e.Date] = e.Count
	}
	// Walk back from today.
	today := now.UTC().Format("2006-01-02")
	cursor, _ := time.Parse("2006-01-02", today)
	for {
		key := cursor.Format("2006-01-02")
		if byDate[key] > 0 {
			streak++
			cursor = cursor.AddDate(0, 0, -1)
			continue
		}
		break
	}
	// last30 — count done within the trailing 30 days.
	cutoff := now.AddDate(0, 0, -30).Format("2006-01-02")
	for date, count := range byDate {
		if date >= cutoff && count > 0 {
			last30++
		}
	}
	return
}
