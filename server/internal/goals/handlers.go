package goals

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handlers groups every goals REST endpoint. Mounted at /api/goals/* by the
// router. All handlers expect bearer auth applied upstream.
type Handlers struct {
	DB  *sql.DB
	Now func() time.Time // injectable for tests
}

func NewHandlers(db *sql.DB) *Handlers {
	return &Handlers{DB: db, Now: time.Now}
}

// ─── Milestones ──────────────────────────────────────────────────────────

func (h *Handlers) ListMilestones(w http.ResponseWriter, r *http.Request) {
	includeArchived := r.URL.Query().Get("archived") == "1"
	q := `SELECT id, title, COALESCE(description_md,''), COALESCE(due_date,''),
	             status, flagship, display_order, created_at, updated_at
	        FROM goal_milestones`
	if !includeArchived {
		q += ` WHERE status != 'archived'`
	}
	q += ` ORDER BY flagship DESC, COALESCE(due_date, '9999-12-31') ASC, display_order ASC`

	rows, err := h.DB.QueryContext(r.Context(), q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	out := []Milestone{}
	for rows.Next() {
		var m Milestone
		var flagship int
		if err := rows.Scan(&m.ID, &m.Title, &m.DescriptionMD, &m.DueDate,
			&m.Status, &flagship, &m.DisplayOrder, &m.CreatedAt, &m.UpdatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		m.Flagship = flagship == 1
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) CreateMilestone(w http.ResponseWriter, r *http.Request) {
	var in MilestoneInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	if in.Title == nil || *in.Title == "" {
		writeErrMsg(w, http.StatusBadRequest, "title required")
		return
	}
	now := h.Now().Unix()
	id := uuid.NewString()
	status := StatusPending
	if in.Status != nil {
		status = *in.Status
	}
	var desc, due string
	if in.DescriptionMD != nil {
		desc = *in.DescriptionMD
	}
	if in.DueDate != nil {
		due = *in.DueDate
	}
	flagship := 0
	if in.Flagship != nil && *in.Flagship {
		flagship = 1
	}
	order := 0
	if in.DisplayOrder != nil {
		order = *in.DisplayOrder
	}
	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO goal_milestones
		  (id, title, description_md, due_date, status, flagship, display_order, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		id, *in.Title, desc, nullableDate(due), status, flagship, order, now, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	m, err := h.fetchMilestone(r, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (h *Handlers) UpdateMilestone(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	var in MilestoneInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	sets := []string{}
	args := []any{}
	if in.Title != nil {
		sets = append(sets, "title = ?"); args = append(args, *in.Title)
	}
	if in.DescriptionMD != nil {
		sets = append(sets, "description_md = ?"); args = append(args, *in.DescriptionMD)
	}
	if in.DueDate != nil {
		sets = append(sets, "due_date = ?"); args = append(args, nullableDate(*in.DueDate))
	}
	if in.Status != nil {
		if !validStatus(*in.Status) {
			writeErrMsg(w, http.StatusBadRequest, "invalid status")
			return
		}
		sets = append(sets, "status = ?"); args = append(args, *in.Status)
	}
	if in.Flagship != nil {
		sets = append(sets, "flagship = ?"); args = append(args, boolToInt(*in.Flagship))
	}
	if in.DisplayOrder != nil {
		sets = append(sets, "display_order = ?"); args = append(args, *in.DisplayOrder)
	}
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "noop": true})
		return
	}
	sets = append(sets, "updated_at = ?"); args = append(args, h.Now().Unix())
	q := "UPDATE goal_milestones SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	args = append(args, id)
	res, err := h.DB.ExecContext(r.Context(), q, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrMsg(w, http.StatusNotFound, "milestone not found")
		return
	}
	m, err := h.fetchMilestone(r, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handlers) ArchiveMilestone(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	_, err := h.DB.ExecContext(r.Context(),
		`UPDATE goal_milestones SET status = 'archived', updated_at = ? WHERE id = ?`,
		h.Now().Unix(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handlers) fetchMilestone(r *http.Request, id string) (Milestone, error) {
	var m Milestone
	var flagship int
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT id, title, COALESCE(description_md,''), COALESCE(due_date,''),
		        status, flagship, display_order, created_at, updated_at
		   FROM goal_milestones WHERE id = ?`, id).
		Scan(&m.ID, &m.Title, &m.DescriptionMD, &m.DueDate, &m.Status,
			&flagship, &m.DisplayOrder, &m.CreatedAt, &m.UpdatedAt)
	m.Flagship = flagship == 1
	return m, err
}

// ─── Daily Log ───────────────────────────────────────────────────────────

func (h *Handlers) GetDailyLog(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	if date == "" {
		date = h.Now().UTC().Format("2006-01-02")
	}
	log, err := h.loadDailyLog(r, date)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

func (h *Handlers) PutDailyLog(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	if date == "" {
		date = h.Now().UTC().Format("2006-01-02")
	}
	var in DailyLogInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	items, err := encodeItems(in.Items)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	reflection := ""
	if in.ReflectionMD != nil {
		reflection = *in.ReflectionMD
	}
	now := h.Now().Unix()
	_, err = h.DB.ExecContext(r.Context(),
		`INSERT INTO goal_daily_log (date, items_json, reflection_md, streak_count, updated_at)
		 VALUES (?, ?, ?, 0, ?)
		 ON CONFLICT(date) DO UPDATE SET
		   items_json = excluded.items_json,
		   reflection_md = COALESCE(excluded.reflection_md, reflection_md),
		   updated_at = excluded.updated_at`,
		date, items, reflection, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// Recompute streak (cheap — back-walks day by day until a gap)
	if err := h.recomputeStreak(r, date); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	log, err := h.loadDailyLog(r, date)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

// Streak counter — back-walks from a "today" anchor, counting consecutive days
// where items_json is non-empty (at least one item logged). Stops on first gap.
func (h *Handlers) recomputeStreak(r *http.Request, anchor string) error {
	streak := 0
	cur, err := time.Parse("2006-01-02", anchor)
	if err != nil {
		return err
	}
	for {
		date := cur.Format("2006-01-02")
		var raw string
		err := h.DB.QueryRowContext(r.Context(),
			`SELECT items_json FROM goal_daily_log WHERE date = ?`, date).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return err
		}
		if len(decodeItems(raw)) == 0 {
			break
		}
		streak++
		cur = cur.AddDate(0, 0, -1)
	}
	_, err = h.DB.ExecContext(r.Context(),
		`UPDATE goal_daily_log SET streak_count = ? WHERE date = ?`, streak, anchor)
	return err
}

func (h *Handlers) loadDailyLog(r *http.Request, date string) (DailyLog, error) {
	var raw, reflection string
	var streak int
	var updatedAt int64
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT COALESCE(items_json,'[]'), COALESCE(reflection_md,''), streak_count, updated_at
		   FROM goal_daily_log WHERE date = ?`, date).
		Scan(&raw, &reflection, &streak, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DailyLog{Date: date, Items: []DailyItem{}}, nil
	}
	if err != nil {
		return DailyLog{}, err
	}
	return DailyLog{
		Date: date, Items: decodeItems(raw), ReflectionMD: reflection,
		StreakCount: streak, UpdatedAt: updatedAt,
	}, nil
}

// ─── Weekly tracker ──────────────────────────────────────────────────────

func (h *Handlers) GetWeekly(w http.ResponseWriter, r *http.Request) {
	week := chi.URLParam(r, "week")
	if week == "" {
		week = mondayOf(h.Now()).Format("2006-01-02")
	}
	var theme, retro, raw string
	var updatedAt int64
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT COALESCE(theme,''), COALESCE(weekly_goals_json,'[]'),
		        COALESCE(retro_md,''), updated_at
		   FROM goal_weekly_tracker WHERE week_of = ?`, week).
		Scan(&theme, &raw, &retro, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusOK, WeeklyTracker{WeekOf: week, WeeklyGoals: []DailyItem{}})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, WeeklyTracker{
		WeekOf: week, Theme: theme, WeeklyGoals: decodeItems(raw),
		RetroMD: retro, UpdatedAt: updatedAt,
	})
}

func (h *Handlers) PutWeekly(w http.ResponseWriter, r *http.Request) {
	week := chi.URLParam(r, "week")
	if week == "" {
		week = mondayOf(h.Now()).Format("2006-01-02")
	}
	var in WeeklyTrackerInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	theme := ""
	if in.Theme != nil {
		theme = *in.Theme
	}
	itemsJSON := "[]"
	if in.WeeklyGoals != nil {
		s, err := encodeItems(*in.WeeklyGoals)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		itemsJSON = s
	}
	retro := ""
	if in.RetroMD != nil {
		retro = *in.RetroMD
	}
	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO goal_weekly_tracker (week_of, theme, weekly_goals_json, retro_md, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(week_of) DO UPDATE SET
		   theme = excluded.theme,
		   weekly_goals_json = excluded.weekly_goals_json,
		   retro_md = excluded.retro_md,
		   updated_at = excluded.updated_at`,
		week, theme, itemsJSON, retro, h.Now().Unix())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── Monthly ─────────────────────────────────────────────────────────────

func (h *Handlers) GetMonthly(w http.ResponseWriter, r *http.Request) {
	month := chi.URLParam(r, "month")
	if month == "" {
		month = h.Now().UTC().Format("2006-01")
	}
	var raw, retro string
	var updatedAt int64
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT COALESCE(monthly_goals_json,'[]'), COALESCE(retro_md,''), updated_at
		   FROM goal_monthly WHERE month = ?`, month).
		Scan(&raw, &retro, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusOK, MonthlyGoals{Month: month, MonthlyGoals: []DailyItem{}})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, MonthlyGoals{
		Month: month, MonthlyGoals: decodeItems(raw), RetroMD: retro, UpdatedAt: updatedAt,
	})
}

func (h *Handlers) PutMonthly(w http.ResponseWriter, r *http.Request) {
	month := chi.URLParam(r, "month")
	if month == "" {
		month = h.Now().UTC().Format("2006-01")
	}
	var in MonthlyGoalsInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	itemsJSON := "[]"
	if in.MonthlyGoals != nil {
		s, err := encodeItems(*in.MonthlyGoals)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		itemsJSON = s
	}
	retro := ""
	if in.RetroMD != nil {
		retro = *in.RetroMD
	}
	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO goal_monthly (month, monthly_goals_json, retro_md, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(month) DO UPDATE SET
		   monthly_goals_json = excluded.monthly_goals_json,
		   retro_md = excluded.retro_md,
		   updated_at = excluded.updated_at`,
		month, itemsJSON, retro, h.Now().Unix())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── Output Log ──────────────────────────────────────────────────────────

func (h *Handlers) ListOutput(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, date, category, title, COALESCE(body_md,''), COALESCE(url,''), created_at
		   FROM goal_output_log ORDER BY date DESC, created_at DESC LIMIT 200`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []OutputLogEntry{}
	for rows.Next() {
		var e OutputLogEntry
		if err := rows.Scan(&e.ID, &e.Date, &e.Category, &e.Title, &e.BodyMD, &e.URL, &e.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) CreateOutput(w http.ResponseWriter, r *http.Request) {
	var in OutputLogInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	if in.Title == nil || *in.Title == "" {
		writeErrMsg(w, http.StatusBadRequest, "title required")
		return
	}
	if in.Category == nil || *in.Category == "" {
		writeErrMsg(w, http.StatusBadRequest, "category required")
		return
	}
	date := h.Now().UTC().Format("2006-01-02")
	if in.Date != nil && *in.Date != "" {
		date = *in.Date
	}
	id := uuid.NewString()
	body := ""
	if in.BodyMD != nil {
		body = *in.BodyMD
	}
	url := ""
	if in.URL != nil {
		url = *in.URL
	}
	now := h.Now().Unix()
	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO goal_output_log (id, date, category, title, body_md, url, created_at)
		 VALUES (?,?,?,?,?,?,?)`,
		id, date, *in.Category, *in.Title, body, url, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, OutputLogEntry{
		ID: id, Date: date, Category: *in.Category, Title: *in.Title,
		BodyMD: body, URL: url, CreatedAt: now,
	})
}

// ─── Networking Log ──────────────────────────────────────────────────────

func (h *Handlers) ListNetworking(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, date, person, COALESCE(context,''), COALESCE(next_action,''),
		        COALESCE(next_action_due,''), created_at
		   FROM goal_networking_log ORDER BY date DESC, created_at DESC LIMIT 200`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []NetworkingLogEntry{}
	for rows.Next() {
		var e NetworkingLogEntry
		if err := rows.Scan(&e.ID, &e.Date, &e.Person, &e.Context,
			&e.NextAction, &e.NextActionDue, &e.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) CreateNetworking(w http.ResponseWriter, r *http.Request) {
	var in NetworkingLogInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	if in.Person == nil || *in.Person == "" {
		writeErrMsg(w, http.StatusBadRequest, "person required")
		return
	}
	date := h.Now().UTC().Format("2006-01-02")
	if in.Date != nil && *in.Date != "" {
		date = *in.Date
	}
	id := uuid.NewString()
	now := h.Now().Unix()
	ctx, na, dueAction := "", "", ""
	if in.Context != nil      { ctx = *in.Context }
	if in.NextAction != nil    { na = *in.NextAction }
	if in.NextActionDue != nil { dueAction = *in.NextActionDue }

	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO goal_networking_log (id, date, person, context, next_action, next_action_due, created_at)
		 VALUES (?,?,?,?,?,?,?)`,
		id, date, *in.Person, ctx, na, dueAction, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, NetworkingLogEntry{
		ID: id, Date: date, Person: *in.Person, Context: ctx,
		NextAction: na, NextActionDue: dueAction, CreatedAt: now,
	})
}

// ─── Reminders ───────────────────────────────────────────────────────────

func (h *Handlers) ListReminders(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, title, COALESCE(body,''), recurrence,
		        COALESCE(next_fires_at,0), active, created_at
		   FROM goal_reminders ORDER BY active DESC, next_fires_at ASC NULLS LAST`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []Reminder{}
	for rows.Next() {
		var rm Reminder
		var active int
		if err := rows.Scan(&rm.ID, &rm.Title, &rm.Body, &rm.Recurrence,
			&rm.NextFiresAt, &active, &rm.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		rm.Active = active == 1
		out = append(out, rm)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) CreateReminder(w http.ResponseWriter, r *http.Request) {
	var in ReminderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	if in.Title == nil || *in.Title == "" {
		writeErrMsg(w, http.StatusBadRequest, "title required")
		return
	}
	if in.Recurrence == nil || *in.Recurrence == "" {
		writeErrMsg(w, http.StatusBadRequest, "recurrence required")
		return
	}
	next, err := NextFiresAt(*in.Recurrence, h.Now())
	if err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid recurrence: "+err.Error())
		return
	}
	id := uuid.NewString()
	now := h.Now().Unix()
	body := ""
	if in.Body != nil {
		body = *in.Body
	}
	active := 1
	if in.Active != nil && !*in.Active {
		active = 0
	}
	_, err = h.DB.ExecContext(r.Context(),
		`INSERT INTO goal_reminders (id, title, body, recurrence, next_fires_at, active, created_at)
		 VALUES (?,?,?,?,?,?,?)`,
		id, *in.Title, body, *in.Recurrence, next.Unix(), active, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, Reminder{
		ID: id, Title: *in.Title, Body: body,
		Recurrence: *in.Recurrence, NextFiresAt: next.Unix(),
		Active: active == 1, CreatedAt: now,
	})
}

func (h *Handlers) UpdateReminder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	var in ReminderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	sets := []string{}
	args := []any{}
	if in.Title != nil {
		sets = append(sets, "title = ?"); args = append(args, *in.Title)
	}
	if in.Body != nil {
		sets = append(sets, "body = ?"); args = append(args, *in.Body)
	}
	if in.Recurrence != nil {
		next, err := NextFiresAt(*in.Recurrence, h.Now())
		if err != nil {
			writeErrMsg(w, http.StatusBadRequest, "invalid recurrence: "+err.Error())
			return
		}
		sets = append(sets, "recurrence = ?", "next_fires_at = ?")
		args = append(args, *in.Recurrence, next.Unix())
	}
	if in.Active != nil {
		sets = append(sets, "active = ?"); args = append(args, boolToInt(*in.Active))
	}
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "noop": true})
		return
	}
	q := "UPDATE goal_reminders SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	args = append(args, id)
	if _, err := h.DB.ExecContext(r.Context(), q, args...); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handlers) DeleteReminder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	if _, err := h.DB.ExecContext(r.Context(),
		`DELETE FROM goal_reminders WHERE id = ?`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── Brief ───────────────────────────────────────────────────────────────

func (h *Handlers) GetBrief(w http.ResponseWriter, r *http.Request) {
	b, err := h.ComposeBrief(r.Context(), h.Now())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// ─── helpers ─────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeErr(w http.ResponseWriter, status int, err error) {
	writeErrMsg(w, status, err.Error())
}
func writeErrMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
func boolToInt(b bool) int { if b { return 1 }; return 0 }
func nullableDate(s string) any { if s == "" { return nil }; return s }
func validStatus(s string) bool {
	switch s {
	case StatusPending, StatusInProgress, StatusDone, StatusArchived:
		return true
	}
	return false
}
func mondayOf(t time.Time) time.Time {
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	return t.AddDate(0, 0, -(wd - 1)).Truncate(24 * time.Hour)
}
