package health

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handlers struct {
	DB  *sql.DB
	Now func() time.Time
}

func NewHandlers(db *sql.DB) *Handlers {
	return &Handlers{DB: db, Now: time.Now}
}

// ─── DTOs ─────────────────────────────────────────────────────────────────

type RecoveryRow struct {
	Date   string   `json:"date"`
	Score  *int     `json:"score"`
	HRVms  *float64 `json:"hrv_ms"`
	RHR    *int     `json:"rhr"`
}
type SleepRow struct {
	Date        string `json:"date"`
	DurationMin *int   `json:"duration_min"`
	Score       *int   `json:"score"`
	DebtMin     *int   `json:"debt_min"`
}
type StrainRow struct {
	Date  string   `json:"date"`
	Score *float64 `json:"score"`
	AvgHR *int     `json:"avg_hr"`
	MaxHR *int     `json:"max_hr"`
}

type SupplementDef struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Dose             string `json:"dose"`
	Category         string `json:"category"`           // supplement/peptide/medication
	ScheduleJSON     string `json:"schedule_json"`
	CycleDaysOn      *int   `json:"cycle_days_on"`
	CycleDaysOff     *int   `json:"cycle_days_off"`
	ReminderEnabled  bool   `json:"reminder_enabled"`
	Active           bool   `json:"active"`
	PrescribingDoc   string `json:"prescribing_doc"`
	Notes            string `json:"notes"`
	CreatedAt        int64  `json:"created_at"`
}
type SupplementDefInput struct {
	Name            *string `json:"name,omitempty"`
	Dose            *string `json:"dose,omitempty"`
	Category        *string `json:"category,omitempty"`
	ScheduleJSON    *string `json:"schedule_json,omitempty"`
	CycleDaysOn     *int    `json:"cycle_days_on,omitempty"`
	CycleDaysOff    *int    `json:"cycle_days_off,omitempty"`
	ReminderEnabled *bool   `json:"reminder_enabled,omitempty"`
	Active          *bool   `json:"active,omitempty"`
	PrescribingDoc  *string `json:"prescribing_doc,omitempty"`
	Notes           *string `json:"notes,omitempty"`
}

type SupplementDose struct {
	ID      string `json:"id"`
	DefID   string `json:"def_id"`
	TakenAt int64  `json:"taken_at"`
	Notes   string `json:"notes"`
}

type TodayResponse struct {
	Date       string         `json:"date"`
	Recovery   *RecoveryRow   `json:"recovery"`
	Sleep      *SleepRow      `json:"sleep"`
	Strain     *StrainRow     `json:"strain"`
	Verdict    string         `json:"verdict"`            // push/maintain/recover
	StrainGoal string         `json:"strain_goal"`         // "16.0–18.5"
}

// ─── /api/health/today ────────────────────────────────────────────────────

func (h *Handlers) Today(w http.ResponseWriter, r *http.Request) {
	date := h.Now().UTC().Format("2006-01-02")
	out := TodayResponse{Date: date}

	if rec, err := h.fetchRecovery(r, date); err == nil {
		out.Recovery = rec
	}
	if sl, err := h.fetchSleep(r, date); err == nil {
		out.Sleep = sl
	}
	if st, err := h.fetchStrain(r, date); err == nil {
		out.Strain = st
	}
	if out.Recovery != nil && out.Recovery.Score != nil {
		switch {
		case *out.Recovery.Score >= 70:
			out.Verdict = "push"
			out.StrainGoal = "16.0–18.5"
		case *out.Recovery.Score >= 40:
			out.Verdict = "maintain"
			out.StrainGoal = "10.0–14.0"
		default:
			out.Verdict = "recover"
			out.StrainGoal = "≤ 8.0"
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) fetchRecovery(r *http.Request, date string) (*RecoveryRow, error) {
	row := h.DB.QueryRowContext(r.Context(),
		`SELECT date, score, hrv_ms, rhr FROM health_recovery WHERE date = ?`, date)
	return scanRecovery(row)
}
func (h *Handlers) fetchSleep(r *http.Request, date string) (*SleepRow, error) {
	row := h.DB.QueryRowContext(r.Context(),
		`SELECT date, duration_min, score, debt_min FROM health_sleep WHERE date = ?`, date)
	return scanSleep(row)
}
func (h *Handlers) fetchStrain(r *http.Request, date string) (*StrainRow, error) {
	row := h.DB.QueryRowContext(r.Context(),
		`SELECT date, score, avg_hr, max_hr FROM health_strain WHERE date = ?`, date)
	return scanStrain(row)
}

func scanRecovery(row *sql.Row) (*RecoveryRow, error) {
	var out RecoveryRow
	var score, rhr sql.NullInt64
	var hrv sql.NullFloat64
	if err := row.Scan(&out.Date, &score, &hrv, &rhr); err != nil {
		return nil, err
	}
	if score.Valid {
		v := int(score.Int64)
		out.Score = &v
	}
	if hrv.Valid {
		v := hrv.Float64
		out.HRVms = &v
	}
	if rhr.Valid {
		v := int(rhr.Int64)
		out.RHR = &v
	}
	return &out, nil
}
func scanSleep(row *sql.Row) (*SleepRow, error) {
	var out SleepRow
	var dur, score, debt sql.NullInt64
	if err := row.Scan(&out.Date, &dur, &score, &debt); err != nil {
		return nil, err
	}
	if dur.Valid {
		v := int(dur.Int64)
		out.DurationMin = &v
	}
	if score.Valid {
		v := int(score.Int64)
		out.Score = &v
	}
	if debt.Valid {
		v := int(debt.Int64)
		out.DebtMin = &v
	}
	return &out, nil
}
func scanStrain(row *sql.Row) (*StrainRow, error) {
	var out StrainRow
	var score sql.NullFloat64
	var avg, max sql.NullInt64
	if err := row.Scan(&out.Date, &score, &avg, &max); err != nil {
		return nil, err
	}
	if score.Valid {
		v := score.Float64
		out.Score = &v
	}
	if avg.Valid {
		v := int(avg.Int64)
		out.AvgHR = &v
	}
	if max.Valid {
		v := int(max.Int64)
		out.MaxHR = &v
	}
	return &out, nil
}

// ─── Range listings ───────────────────────────────────────────────────────

func (h *Handlers) Recovery(w http.ResponseWriter, r *http.Request) {
	days := atoiClamp(r.URL.Query().Get("days"), 14, 1, 365)
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT date, score, hrv_ms, rhr FROM health_recovery
		   ORDER BY date DESC LIMIT ?`, days)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []RecoveryRow{}
	for rows.Next() {
		var row RecoveryRow
		var score, rhr sql.NullInt64
		var hrv sql.NullFloat64
		if err := rows.Scan(&row.Date, &score, &hrv, &rhr); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if score.Valid { v := int(score.Int64); row.Score = &v }
		if hrv.Valid   { v := hrv.Float64;     row.HRVms = &v }
		if rhr.Valid   { v := int(rhr.Int64);  row.RHR   = &v }
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) Sleep(w http.ResponseWriter, r *http.Request) {
	days := atoiClamp(r.URL.Query().Get("days"), 14, 1, 365)
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT date, duration_min, score, debt_min FROM health_sleep
		   ORDER BY date DESC LIMIT ?`, days)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []SleepRow{}
	for rows.Next() {
		var row SleepRow
		var dur, score, debt sql.NullInt64
		if err := rows.Scan(&row.Date, &dur, &score, &debt); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if dur.Valid   { v := int(dur.Int64);   row.DurationMin = &v }
		if score.Valid { v := int(score.Int64); row.Score = &v }
		if debt.Valid  { v := int(debt.Int64);  row.DebtMin = &v }
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) Strain(w http.ResponseWriter, r *http.Request) {
	days := atoiClamp(r.URL.Query().Get("days"), 14, 1, 365)
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT date, score, avg_hr, max_hr FROM health_strain
		   ORDER BY date DESC LIMIT ?`, days)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []StrainRow{}
	for rows.Next() {
		var row StrainRow
		var score sql.NullFloat64
		var avg, max sql.NullInt64
		if err := rows.Scan(&row.Date, &score, &avg, &max); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if score.Valid { v := score.Float64;  row.Score = &v }
		if avg.Valid   { v := int(avg.Int64); row.AvgHR = &v }
		if max.Valid   { v := int(max.Int64); row.MaxHR = &v }
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, out)
}

// ─── Supplements ──────────────────────────────────────────────────────────

func (h *Handlers) ListSupplementDefs(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("inactive") == "1"
	q := `SELECT id, name, COALESCE(dose,''), COALESCE(category,'supplement'),
	             COALESCE(schedule_json,''), cycle_days_on, cycle_days_off,
	             reminder_enabled, active, COALESCE(prescribing_doc,''),
	             COALESCE(notes,''), created_at
	        FROM health_supplement_defs`
	if !includeInactive {
		q += ` WHERE active = 1`
	}
	q += ` ORDER BY name ASC`
	rows, err := h.DB.QueryContext(r.Context(), q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []SupplementDef{}
	for rows.Next() {
		var d SupplementDef
		var cycleOn, cycleOff sql.NullInt64
		var reminder, active int
		if err := rows.Scan(&d.ID, &d.Name, &d.Dose, &d.Category, &d.ScheduleJSON,
			&cycleOn, &cycleOff, &reminder, &active,
			&d.PrescribingDoc, &d.Notes, &d.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if cycleOn.Valid { v := int(cycleOn.Int64); d.CycleDaysOn = &v }
		if cycleOff.Valid { v := int(cycleOff.Int64); d.CycleDaysOff = &v }
		d.ReminderEnabled = reminder == 1
		d.Active = active == 1
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) CreateSupplementDef(w http.ResponseWriter, r *http.Request) {
	var in SupplementDefInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	if in.Name == nil || *in.Name == "" {
		writeErrMsg(w, http.StatusBadRequest, "name required")
		return
	}
	id := uuid.NewString()
	now := h.Now().Unix()
	category := "supplement"
	if in.Category != nil && *in.Category != "" {
		category = *in.Category
	}
	dose, sched, doc, notes := "", "{}", "", ""
	if in.Dose != nil          { dose = *in.Dose }
	if in.ScheduleJSON != nil  { sched = *in.ScheduleJSON }
	if in.PrescribingDoc != nil { doc = *in.PrescribingDoc }
	if in.Notes != nil         { notes = *in.Notes }
	reminder := 1
	if in.ReminderEnabled != nil && !*in.ReminderEnabled {
		reminder = 0
	}
	active := 1
	if in.Active != nil && !*in.Active {
		active = 0
	}

	var cycleOn, cycleOff any
	if in.CycleDaysOn != nil   { cycleOn = *in.CycleDaysOn }
	if in.CycleDaysOff != nil  { cycleOff = *in.CycleDaysOff }

	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO health_supplement_defs
		   (id, name, dose, category, schedule_json, cycle_days_on, cycle_days_off,
		    reminder_enabled, active, prescribing_doc, notes, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, *in.Name, dose, category, sched, cycleOn, cycleOff,
		reminder, active, doc, notes, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *Handlers) UpdateSupplementDef(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	var in SupplementDefInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	sets := []string{}
	args := []any{}
	if in.Name != nil           { sets = append(sets, "name = ?");           args = append(args, *in.Name) }
	if in.Dose != nil           { sets = append(sets, "dose = ?");           args = append(args, *in.Dose) }
	if in.Category != nil       { sets = append(sets, "category = ?");       args = append(args, *in.Category) }
	if in.ScheduleJSON != nil   { sets = append(sets, "schedule_json = ?");  args = append(args, *in.ScheduleJSON) }
	if in.CycleDaysOn != nil    { sets = append(sets, "cycle_days_on = ?");  args = append(args, *in.CycleDaysOn) }
	if in.CycleDaysOff != nil   { sets = append(sets, "cycle_days_off = ?"); args = append(args, *in.CycleDaysOff) }
	if in.ReminderEnabled != nil{ sets = append(sets, "reminder_enabled = ?"); args = append(args, boolToInt(*in.ReminderEnabled)) }
	if in.Active != nil         { sets = append(sets, "active = ?");         args = append(args, boolToInt(*in.Active)) }
	if in.PrescribingDoc != nil { sets = append(sets, "prescribing_doc = ?"); args = append(args, *in.PrescribingDoc) }
	if in.Notes != nil          { sets = append(sets, "notes = ?");          args = append(args, *in.Notes) }
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "noop": true})
		return
	}
	q := "UPDATE health_supplement_defs SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	args = append(args, id)
	if _, err := h.DB.ExecContext(r.Context(), q, args...); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handlers) DeleteSupplementDef(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	// Soft-archive by default — preserves the log history
	if _, err := h.DB.ExecContext(r.Context(),
		`UPDATE health_supplement_defs SET active = 0 WHERE id = ?`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type logSupplementInput struct {
	DefID   string `json:"def_id"`
	TakenAt *int64 `json:"taken_at,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

func (h *Handlers) LogSupplementDose(w http.ResponseWriter, r *http.Request) {
	var in logSupplementInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	if in.DefID == "" {
		writeErrMsg(w, http.StatusBadRequest, "def_id required")
		return
	}
	taken := h.Now().Unix()
	if in.TakenAt != nil {
		taken = *in.TakenAt
	}
	id := uuid.NewString()
	if _, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO health_supplement_log (id, def_id, taken_at, notes) VALUES (?,?,?,?)`,
		id, in.DefID, taken, in.Notes); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, SupplementDose{
		ID: id, DefID: in.DefID, TakenAt: taken, Notes: in.Notes,
	})
}

func (h *Handlers) ListSupplementDoses(w http.ResponseWriter, r *http.Request) {
	days := atoiClamp(r.URL.Query().Get("days"), 30, 1, 365)
	cutoff := h.Now().AddDate(0, 0, -days).Unix()
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, def_id, taken_at, COALESCE(notes,'') FROM health_supplement_log
		   WHERE taken_at >= ?
		   ORDER BY taken_at DESC`, cutoff)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []SupplementDose{}
	for rows.Next() {
		var d SupplementDose
		if err := rows.Scan(&d.ID, &d.DefID, &d.TakenAt, &d.Notes); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

// ─── Mood log ─────────────────────────────────────────────────────────────

type moodInput struct {
	Mood   *int    `json:"mood,omitempty"`
	Energy *int    `json:"energy,omitempty"`
	Focus  *int    `json:"focus,omitempty"`
	Notes  *string `json:"notes,omitempty"`
}

func (h *Handlers) PutMood(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	if date == "" {
		date = h.Now().UTC().Format("2006-01-02")
	}
	var in moodInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	var mood, energy, focus any
	notes := ""
	if in.Mood != nil   { mood = *in.Mood }
	if in.Energy != nil { energy = *in.Energy }
	if in.Focus != nil  { focus = *in.Focus }
	if in.Notes != nil  { notes = *in.Notes }
	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO health_mood_log (date, mood, energy, focus, notes)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(date) DO UPDATE SET
		   mood = excluded.mood, energy = excluded.energy,
		   focus = excluded.focus, notes = excluded.notes`,
		date, mood, energy, focus, notes)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── helpers ──────────────────────────────────────────────────────────────

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
func atoiClamp(s string, def, min, max int) int {
	if s == "" { return def }
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' { return def }
		n = n*10 + int(c-'0')
		if n > max { return max }
	}
	if n < min { return min }
	return n
}

