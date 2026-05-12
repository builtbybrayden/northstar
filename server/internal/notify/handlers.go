package notify

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	DB  *sql.DB
	Hub *Hub // optional — required for /stream
}

func NewHandlers(db *sql.DB) *Handlers { return &Handlers{DB: db} }

// WithHub attaches a Hub. Required for /api/notifications/stream to function.
func (h *Handlers) WithHub(hub *Hub) *Handlers {
	h.Hub = hub
	return h
}

// ─── /api/notifications/feed ──────────────────────────────────────────────

type notificationDTO struct {
	ID             string         `json:"id"`
	Category       string         `json:"category"`
	Title          string         `json:"title"`
	Body           string         `json:"body"`
	Priority       int            `json:"priority"`
	Payload        map[string]any `json:"payload"`
	CreatedAt      int64          `json:"created_at"`
	ReadAt         *int64         `json:"read_at"`
	DeliveryStatus string         `json:"delivery_status"`
}

func (h *Handlers) Feed(w http.ResponseWriter, r *http.Request) {
	limit := atoiClamp(r.URL.Query().Get("limit"), 50, 1, 500)
	unread := r.URL.Query().Get("unread") == "1"

	q := `SELECT id, category, title, COALESCE(body,''), priority, payload_json,
	             created_at, read_at, delivery_status
	        FROM notifications`
	if unread {
		q += ` WHERE read_at IS NULL`
	}
	q += ` ORDER BY created_at DESC LIMIT ?`

	rows, err := h.DB.QueryContext(r.Context(), q, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	out := []notificationDTO{}
	for rows.Next() {
		var n notificationDTO
		var payloadJSON string
		var readAt sql.NullInt64
		if err := rows.Scan(&n.ID, &n.Category, &n.Title, &n.Body, &n.Priority,
			&payloadJSON, &n.CreatedAt, &readAt, &n.DeliveryStatus); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if readAt.Valid {
			v := readAt.Int64
			n.ReadAt = &v
		}
		if payloadJSON != "" {
			_ = json.Unmarshal([]byte(payloadJSON), &n.Payload)
		}
		out = append(out, n)
	}
	writeJSON(w, http.StatusOK, out)
}

// ─── /api/notifications/{id}/read ─────────────────────────────────────────

func (h *Handlers) MarkRead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	res, err := h.DB.ExecContext(r.Context(),
		`UPDATE notifications SET read_at = ? WHERE id = ? AND read_at IS NULL`,
		time.Now().Unix(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": n})
}

// ─── /api/notifications/unread-count ──────────────────────────────────────

func (h *Handlers) UnreadCount(w http.ResponseWriter, r *http.Request) {
	var n int
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM notifications WHERE read_at IS NULL`).Scan(&n)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread": n})
}

// ─── /api/notifications/rules ─────────────────────────────────────────────

type ruleDTO struct {
	Category        string `json:"category"`
	Enabled         bool   `json:"enabled"`
	QuietHoursStart string `json:"quiet_hours_start"`
	QuietHoursEnd   string `json:"quiet_hours_end"`
	BypassQuiet     bool   `json:"bypass_quiet"`
	Delivery        string `json:"delivery"`
	MaxPerDay       int    `json:"max_per_day"`
}

func (h *Handlers) ListRules(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT category, enabled, COALESCE(quiet_hours_start,''), COALESCE(quiet_hours_end,''),
		        bypass_quiet, delivery, max_per_day
		   FROM notif_rules ORDER BY category ASC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	out := []ruleDTO{}
	for rows.Next() {
		var d ruleDTO
		var enabled, bypass int
		if err := rows.Scan(&d.Category, &enabled, &d.QuietHoursStart, &d.QuietHoursEnd,
			&bypass, &d.Delivery, &d.MaxPerDay); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		d.Enabled = enabled == 1
		d.BypassQuiet = bypass == 1
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

type ruleUpdate struct {
	Enabled         *bool   `json:"enabled,omitempty"`
	QuietHoursStart *string `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd   *string `json:"quiet_hours_end,omitempty"`
	BypassQuiet     *bool   `json:"bypass_quiet,omitempty"`
	Delivery        *string `json:"delivery,omitempty"`
	MaxPerDay       *int    `json:"max_per_day,omitempty"`
}

func (h *Handlers) UpdateRule(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "category")
	if category == "" {
		writeErrMsg(w, http.StatusBadRequest, "category required")
		return
	}
	var u ruleUpdate
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	// Build dynamic UPDATE — only set fields the caller provided.
	sets := []string{}
	args := []any{}
	if u.Enabled != nil {
		sets = append(sets, "enabled = ?")
		args = append(args, boolToInt(*u.Enabled))
	}
	if u.QuietHoursStart != nil {
		sets = append(sets, "quiet_hours_start = ?")
		args = append(args, *u.QuietHoursStart)
	}
	if u.QuietHoursEnd != nil {
		sets = append(sets, "quiet_hours_end = ?")
		args = append(args, *u.QuietHoursEnd)
	}
	if u.BypassQuiet != nil {
		sets = append(sets, "bypass_quiet = ?")
		args = append(args, boolToInt(*u.BypassQuiet))
	}
	if u.Delivery != nil {
		switch *u.Delivery {
		case "push", "live_activity", "silent_badge":
			sets = append(sets, "delivery = ?")
			args = append(args, *u.Delivery)
		default:
			writeErrMsg(w, http.StatusBadRequest, "invalid delivery (push/live_activity/silent_badge)")
			return
		}
	}
	if u.MaxPerDay != nil {
		sets = append(sets, "max_per_day = ?")
		args = append(args, *u.MaxPerDay)
	}
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "noop": true})
		return
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now().Unix())

	query := "UPDATE notif_rules SET " + joinComma(sets) + " WHERE category = ?"
	args = append(args, category)

	res, err := h.DB.ExecContext(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErrMsg(w, http.StatusNotFound, "rule not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── /api/notifications/stream ────────────────────────────────────────────
//
// Server-Sent Events fanout. Open iOS apps subscribe; each successful
// Composer.Fire publishes a JSON line. Heartbeat every 25s keeps idle
// connections alive through NAT / proxies.

func (h *Handlers) Stream(w http.ResponseWriter, r *http.Request) {
	if h.Hub == nil {
		writeErrMsg(w, http.StatusServiceUnavailable, "live stream not enabled")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	sub, unsub := h.Hub.Subscribe()
	defer unsub()

	// initial "hello" event so the client knows the stream is alive
	_, _ = fmt.Fprint(w, "event: ready\ndata: {}\n\n")
	if flusher != nil {
		flusher.Flush()
	}

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	type wireEvent struct {
		Type      string         `json:"type"`
		ID        string         `json:"id"`
		Category  string         `json:"category"`
		Title     string         `json:"title"`
		Body      string         `json:"body"`
		Priority  int            `json:"priority"`
		Payload   map[string]any `json:"payload"`
		CreatedAt int64          `json:"created_at"`
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case n, ok := <-sub:
			if !ok {
				return
			}
			ev := wireEvent{
				Type:      "notification",
				ID:        n.ID,
				Category:  n.Category,
				Title:     n.Title,
				Body:      n.Body,
				Priority:  n.Priority,
				Payload:   n.Payload,
				CreatedAt: n.CreatedAt.Unix(),
			}
			b, _ := json.Marshal(ev)
			_, err := fmt.Fprintf(w, "data: %s\n\n", string(b))
			if err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
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
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func atoiClamp(s string, def, min, max int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
		if n > max {
			return max
		}
	}
	if n < min {
		return min
	}
	return n
}
func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
