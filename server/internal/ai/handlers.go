package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handlers struct {
	DB         *sql.DB
	Dispatcher *ToolDispatcher
	Client     *Client      // nil in mock mode
	Mock       *MockEngine  // non-nil in mock mode
	Now        func() time.Time
}

func NewHandlers(db *sql.DB, disp *ToolDispatcher, client *Client, mock *MockEngine) *Handlers {
	return &Handlers{DB: db, Dispatcher: disp, Client: client, Mock: mock, Now: time.Now}
}

// ─── /api/ai/conversations ────────────────────────────────────────────────

type conversationDTO struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	StartedAt   int64   `json:"started_at"`
	PillarScope []string `json:"pillar_scope"`
}

func (h *Handlers) ListConversations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, COALESCE(title,''), started_at, COALESCE(pillar_scope,'[]')
		   FROM ai_conversations
		  WHERE archived = 0
		  ORDER BY started_at DESC LIMIT 100`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []conversationDTO{}
	for rows.Next() {
		var c conversationDTO
		var scope string
		if err := rows.Scan(&c.ID, &c.Title, &c.StartedAt, &scope); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		_ = json.Unmarshal([]byte(scope), &c.PillarScope)
		if c.PillarScope == nil {
			c.PillarScope = []string{}
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, out)
}

type createConversationInput struct {
	Title string `json:"title,omitempty"`
}

func (h *Handlers) CreateConversation(w http.ResponseWriter, r *http.Request) {
	var in createConversationInput
	_ = json.NewDecoder(r.Body).Decode(&in)
	id := uuid.NewString()
	now := h.Now().Unix()
	if _, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO ai_conversations (id, started_at, title, pillar_scope, archived)
		 VALUES (?, ?, ?, '[]', 0)`, id, now, in.Title); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, conversationDTO{
		ID: id, Title: in.Title, StartedAt: now, PillarScope: []string{},
	})
}

func (h *Handlers) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	if _, err := h.DB.ExecContext(r.Context(),
		`UPDATE ai_conversations SET archived = 1 WHERE id = ?`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type patchConversationInput struct {
	Title *string `json:"title,omitempty"`
}

// PatchConversation lets the client rename a conversation. Currently only
// title is mutable; pillar_scope edits would land here too when wired.
func (h *Handlers) PatchConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	var in patchConversationInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}
	if in.Title == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "noop": true})
		return
	}
	title := strings.TrimSpace(*in.Title)
	if len(title) > 200 {
		title = title[:200]
	}
	res, err := h.DB.ExecContext(r.Context(),
		`UPDATE ai_conversations SET title = ? WHERE id = ? AND archived = 0`,
		title, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErrMsg(w, http.StatusNotFound, "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "title": title})
}

// ─── Conversation messages ────────────────────────────────────────────────

type messageDTO struct {
	ID            string          `json:"id"`
	Role          string          `json:"role"`
	Content       json.RawMessage `json:"content"`
	ToolCalls     json.RawMessage `json:"tool_calls,omitempty"`
	CreatedAt     int64           `json:"created_at"`
}

func (h *Handlers) GetMessages(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, role, content_json, COALESCE(tool_calls_json,'null'), created_at
		   FROM ai_messages WHERE conv_id = ? ORDER BY created_at ASC`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []messageDTO{}
	for rows.Next() {
		var m messageDTO
		var content, tools string
		if err := rows.Scan(&m.ID, &m.Role, &content, &tools, &m.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		m.Content = json.RawMessage(content)
		if tools != "null" && tools != "" {
			m.ToolCalls = json.RawMessage(tools)
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
}

// ─── SSE: send a user message + stream the assistant reply ────────────────

type sendMessageInput struct {
	Text string `json:"text"`
}

func (h *Handlers) SendMessageStream(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")
	if convID == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	var in sendMessageInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Text) == "" {
		writeErrMsg(w, http.StatusBadRequest, "text required")
		return
	}

	// Verify conversation exists
	var ok int
	if err := h.DB.QueryRowContext(r.Context(),
		`SELECT 1 FROM ai_conversations WHERE id = ? AND archived = 0`, convID).Scan(&ok); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErrMsg(w, http.StatusNotFound, "conversation not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx-style buffering
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	emit := func(ev OutEvent) {
		b, _ := json.Marshal(ev)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
		if flusher != nil {
			flusher.Flush()
		}
	}

	// 1. Persist user message
	userBlock := []contentBlock{{Type: "text", Text: in.Text}}
	userJSON, _ := json.Marshal(userBlock)
	userID := uuid.NewString()
	now := h.Now().Unix()
	if _, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO ai_messages (id, conv_id, role, content_json, created_at)
		 VALUES (?, ?, 'user', ?, ?)`,
		userID, convID, string(userJSON), now); err != nil {
		emit(OutEvent{Type: "error", Error: err.Error()})
		return
	}

	// 2. Reconstruct convo history for the model
	history, err := h.loadHistory(r.Context(), convID)
	if err != nil {
		emit(OutEvent{Type: "error", Error: err.Error()})
		return
	}

	var assistantBlocks []contentBlock
	var usage Usage
	var streamErr error
	if h.Client != nil {
		// Real Claude mode
		executor := func(ctx context.Context, name string, raw json.RawMessage) (string, error) {
			return h.Dispatcher.Dispatch(ctx, name, raw)
		}
		assistantBlocks, usage, streamErr = h.Client.StreamConversation(
			r.Context(),
			SystemBlocks(h.Now()),
			Defs(),
			history,
			executor,
			emit,
		)
	} else if h.Mock != nil {
		assistantBlocks, streamErr = h.Mock.Stream(r.Context(), history, emit)
	} else {
		streamErr = errors.New("no AI backend configured")
	}

	if streamErr != nil {
		emit(OutEvent{Type: "error", Error: streamErr.Error()})
		return
	}

	// 3. Persist assistant turn (with token-usage telemetry when present)
	assistantJSON, _ := json.Marshal(assistantBlocks)
	assistantID := uuid.NewString()
	var usageJSON sql.NullString
	if usage != (Usage{}) {
		if b, err := json.Marshal(usage); err == nil {
			usageJSON = sql.NullString{String: string(b), Valid: true}
		}
	}
	if _, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO ai_messages (id, conv_id, role, content_json, usage_json, created_at)
		 VALUES (?, ?, 'assistant', ?, ?, ?)`,
		assistantID, convID, string(assistantJSON), usageJSON, h.Now().Unix()); err != nil {
		emit(OutEvent{Type: "error", Error: err.Error()})
		return
	}

	// 4. Auto-title from first user message if conversation is still untitled
	h.maybeAutoTitle(r.Context(), convID, in.Text)

	emit(OutEvent{Type: "done"})
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func (h *Handlers) loadHistory(ctx context.Context, convID string) ([]apiMessage, error) {
	rows, err := h.DB.QueryContext(ctx,
		`SELECT role, content_json FROM ai_messages
		   WHERE conv_id = ? ORDER BY created_at ASC`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []apiMessage
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err != nil {
			return nil, err
		}
		var blocks []contentBlock
		if err := json.Unmarshal([]byte(content), &blocks); err != nil {
			return nil, err
		}
		out = append(out, apiMessage{Role: role, Content: blocks})
	}
	return out, nil
}

func (h *Handlers) maybeAutoTitle(ctx context.Context, convID, firstText string) {
	var existing string
	err := h.DB.QueryRowContext(ctx,
		`SELECT COALESCE(title,'') FROM ai_conversations WHERE id = ?`, convID).Scan(&existing)
	if err != nil || existing != "" {
		return
	}
	title := strings.TrimSpace(firstText)
	if len(title) > 60 {
		title = title[:57] + "…"
	}
	_, _ = h.DB.ExecContext(ctx,
		`UPDATE ai_conversations SET title = ? WHERE id = ?`, title, convID)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeErr(w http.ResponseWriter, status int, err error) { writeErrMsg(w, status, err.Error()) }
func writeErrMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
