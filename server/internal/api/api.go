package api

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"github.com/builtbybrayden/northstar/server/internal/ai"
	"github.com/builtbybrayden/northstar/server/internal/auth"
	"github.com/builtbybrayden/northstar/server/internal/config"
	"github.com/builtbybrayden/northstar/server/internal/finance"
	"github.com/builtbybrayden/northstar/server/internal/goals"
	"github.com/builtbybrayden/northstar/server/internal/health"
	"github.com/builtbybrayden/northstar/server/internal/notify"
)

type Server struct {
	cfg           config.Config
	db            *sql.DB
	notifyHub     *notify.Hub
	adminWarnOnce sync.Once
}

func NewServer(cfg config.Config, db *sql.DB) *Server {
	return &Server{cfg: cfg, db: db}
}

// WithNotifyHub injects the live-notification fanout hub. When attached, the
// /api/notifications/stream endpoint serves live SSE events.
func (s *Server) WithNotifyHub(h *notify.Hub) *Server {
	s.notifyHub = h
	return s
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Public endpoints
	r.Get("/api/health", s.handleHealth)
	r.Post("/api/pair/initiate", s.handlePairInitiate)
	r.Post("/api/pair/redeem", s.handlePairRedeem)

	// Authenticated endpoints
	fin := finance.NewHandlers(s.db)
	nh := notify.NewHandlers(s.db)
	if s.notifyHub != nil {
		nh.WithHub(s.notifyHub)
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(s.db))
		r.Post("/api/devices/register-apns", s.handleRegisterAPNS)
		r.Get("/api/me", s.handleMe)
		r.Get("/api/pillars", s.handlePillars)

		if s.cfg.Pillars.Finance {
			r.Get("/api/finance/accounts", fin.Accounts)
			r.Patch("/api/finance/accounts/{id}", fin.UpdateAccount)
			r.Get("/api/finance/transactions", fin.Transactions)
			r.Patch("/api/finance/transactions/{id}", fin.UpdateTransaction)
			r.Get("/api/finance/summary", fin.Summary)
			r.Get("/api/finance/forecast", fin.ForecastEndpoint)
			r.Get("/api/finance/investments", fin.Investments)
			r.Get("/api/finance/balance-history", fin.BalanceHistory)
			r.Get("/api/finance/settings", fin.GetFinanceSettings)
			r.Patch("/api/finance/settings", fin.UpdateFinanceSettings)
			r.Get("/api/finance/budget-targets", fin.ListBudgetTargets)
			r.Patch("/api/finance/budget-targets/{category}", fin.UpdateBudgetTarget)
		}

		// Notifications (always on — categories are pillar-aware but the
		// rules + feed are cross-cutting)
		r.Get("/api/notifications/feed", nh.Feed)
		r.Get("/api/notifications/unread-count", nh.UnreadCount)
		r.Post("/api/notifications/{id}/read", nh.MarkRead)
		r.Get("/api/notifications/rules", nh.ListRules)
		r.Patch("/api/notifications/rules/{category}", nh.UpdateRule)
		r.Get("/api/notifications/stream", nh.Stream)

		// User settings (daily brief time, etc.)
		r.Get("/api/me/settings", s.handleGetSettings)
		r.Patch("/api/me/settings", s.handlePatchSettings)

		if s.cfg.Pillars.Health {
			hh := health.NewHandlers(s.db)
			r.Get("/api/health/today", hh.Today)
			r.Get("/api/health/recovery", hh.Recovery)
			r.Get("/api/health/sleep", hh.Sleep)
			r.Get("/api/health/strain", hh.Strain)
			r.Get("/api/health/supplements/defs", hh.ListSupplementDefs)
			r.Post("/api/health/supplements/defs", hh.CreateSupplementDef)
			r.Patch("/api/health/supplements/defs/{id}", hh.UpdateSupplementDef)
			r.Delete("/api/health/supplements/defs/{id}", hh.DeleteSupplementDef)
			r.Post("/api/health/supplements/log", hh.LogSupplementDose)
			r.Get("/api/health/supplements/log", hh.ListSupplementDoses)
			r.Put("/api/health/mood/{date}", hh.PutMood)
		}

		if s.cfg.Pillars.AI {
			disp := ai.NewToolDispatcher(s.db)
			var client *ai.Client
			var mock *ai.MockEngine
			var cli *ai.CLIEngine
			switch strings.ToLower(s.cfg.AI.Mode) {
			case "anthropic":
				if s.cfg.AI.APIKey != "" {
					client = ai.NewClient(s.cfg.AI.APIKey, s.cfg.AI.Model)
				} else {
					log.Printf("ai: mode=anthropic but NORTHSTAR_CLAUDE_API_KEY empty — falling back to mock")
					mock = ai.NewMockEngine(disp)
				}
			case "cli":
				cli = ai.NewCLIEngine(disp, s.cfg.AI.CLIBin, s.cfg.AI.Model)
				if s.cfg.AI.CLIBridgeURL != "" {
					cli = cli.WithBridge(s.cfg.AI.CLIBridgeURL, s.cfg.AI.CLIBridgeSecret)
					log.Printf("ai: cli mode via bridge %s", s.cfg.AI.CLIBridgeURL)
				} else {
					log.Printf("ai: cli mode via direct exec (binary=%s)", s.cfg.AI.CLIBin)
				}
			default:
				mock = ai.NewMockEngine(disp)
			}
			ah := ai.NewHandlers(s.db, disp, client, mock, cli)
			r.Get("/api/ai/conversations", ah.ListConversations)
			r.Post("/api/ai/conversations", ah.CreateConversation)
			r.Patch("/api/ai/conversations/{id}", ah.PatchConversation)
			r.Delete("/api/ai/conversations/{id}", ah.DeleteConversation)
			r.Get("/api/ai/conversations/{id}/messages", ah.GetMessages)
			r.Post("/api/ai/conversations/{id}/messages", ah.SendMessageStream)
		}

		if s.cfg.Pillars.Goals {
			gh := goals.NewHandlers(s.db)
			r.Get("/api/goals/milestones", gh.ListMilestones)
			r.Post("/api/goals/milestones", gh.CreateMilestone)
			r.Patch("/api/goals/milestones/{id}", gh.UpdateMilestone)
			r.Delete("/api/goals/milestones/{id}", gh.ArchiveMilestone)

			r.Get("/api/goals/daily/{date}", gh.GetDailyLog)
			r.Put("/api/goals/daily/{date}", gh.PutDailyLog)
			r.Get("/api/goals/daily", gh.GetDailyLog)   // defaults to today
			r.Put("/api/goals/daily", gh.PutDailyLog)

			r.Get("/api/goals/weekly/{week}", gh.GetWeekly)
			r.Put("/api/goals/weekly/{week}", gh.PutWeekly)
			r.Get("/api/goals/monthly/{month}", gh.GetMonthly)
			r.Put("/api/goals/monthly/{month}", gh.PutMonthly)

			r.Get("/api/goals/output", gh.ListOutput)
			r.Post("/api/goals/output", gh.CreateOutput)

			r.Get("/api/goals/networking", gh.ListNetworking)
			r.Post("/api/goals/networking", gh.CreateNetworking)

			r.Get("/api/goals/reminders", gh.ListReminders)
			r.Post("/api/goals/reminders", gh.CreateReminder)
			r.Patch("/api/goals/reminders/{id}", gh.UpdateReminder)
			r.Delete("/api/goals/reminders/{id}", gh.DeleteReminder)

			r.Get("/api/goals/brief", gh.GetBrief)

			r.Get("/api/goals/habits", gh.ListHabits)
			r.Post("/api/goals/habits", gh.CreateHabit)
			r.Patch("/api/goals/habits/{id}", gh.UpdateHabit)
			r.Delete("/api/goals/habits/{id}", gh.DeleteHabit)
			r.Put("/api/goals/habits/{id}/log/{date}", gh.PutHabitLog)
		}
	})

	return r
}

// ─── Health ────────────────────────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dbOK := true
	if err := s.db.PingContext(r.Context()); err != nil {
		dbOK = false
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "northstar-server",
		"version": "0.0.1-phase0",
		"db":      dbOK,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// ─── Pairing flow ──────────────────────────────────────────────────────────
//
// 1. Admin (or first-run setup) POSTs to /api/pair/initiate to mint a one-time
//    6-digit code + bearer token tied to a user. The server displays the code
//    on its admin UI as a QR.
// 2. The iOS app scans the QR, then POSTs /api/pair/redeem with the code.
//    The server returns the bearer token and a device ID. Code is consumed.

type pairInitiateReq struct {
	Email      string `json:"email,omitempty"`
	DeviceName string `json:"device_name,omitempty"`
}

type pairInitiateResp struct {
	Code      string `json:"code"`
	ExpiresAt int64  `json:"expires_at"`
}

func (s *Server) handlePairInitiate(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdminToken(r) {
		writeErr(w, http.StatusUnauthorized, "admin_required",
			errors.New("admin token required for pair/initiate"))
		return
	}

	var req pairInitiateReq
	_ = json.NewDecoder(r.Body).Decode(&req)

	now := time.Now()

	// Single-user model for v1: get-or-create the singleton user.
	userID, err := s.getOrCreateSingletonUser(r, req.Email)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "user_init_failed", err)
		return
	}

	code, err := auth.GeneratePairingCode()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "code_gen_failed", err)
		return
	}
	token, err := auth.GenerateToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token_gen_failed", err)
		return
	}
	expires := now.Add(s.cfg.PairingTTL)

	_, err = s.db.ExecContext(r.Context(),
		`INSERT INTO pairing_codes (code, user_id, token, expires_at) VALUES (?, ?, ?, ?)`,
		code, userID, token, expires.Unix())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "pair_insert_failed", err)
		return
	}

	writeJSON(w, http.StatusOK, pairInitiateResp{
		Code:      code,
		ExpiresAt: expires.Unix(),
	})
}

type pairRedeemReq struct {
	Code       string `json:"code"`
	DeviceName string `json:"device_name"`
}

type pairRedeemResp struct {
	DeviceID    string `json:"device_id"`
	BearerToken string `json:"bearer_token"`
	ServerInfo  struct {
		Version string `json:"version"`
	} `json:"server_info"`
}

func (s *Server) handlePairRedeem(w http.ResponseWriter, r *http.Request) {
	var req pairRedeemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err)
		return
	}
	if req.Code == "" {
		writeErr(w, http.StatusBadRequest, "code_required", errors.New("code is required"))
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "tx_failed", err)
		return
	}
	defer tx.Rollback()

	var userID, token string
	var expiresAt int64
	var consumedAt sql.NullInt64
	err = tx.QueryRowContext(r.Context(),
		`SELECT user_id, token, expires_at, consumed_at FROM pairing_codes WHERE code = ?`,
		req.Code).Scan(&userID, &token, &expiresAt, &consumedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "code_not_found", err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "pair_lookup_failed", err)
		return
	}
	if consumedAt.Valid && consumedAt.Int64 > 0 {
		writeErr(w, http.StatusGone, "code_already_used", errors.New("code already used"))
		return
	}
	if time.Now().Unix() > expiresAt {
		writeErr(w, http.StatusGone, "code_expired", errors.New("code expired"))
		return
	}

	now := time.Now().Unix()
	deviceID := uuid.NewString()
	tokenHash := auth.HashToken(token)
	deviceName := req.DeviceName
	if deviceName == "" {
		deviceName = "iPhone"
	}

	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO devices (id, user_id, name, token_hash, paired_at, last_seen) VALUES (?,?,?,?,?,?)`,
		deviceID, userID, deviceName, tokenHash, now, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "device_insert_failed", err)
		return
	}

	_, err = tx.ExecContext(r.Context(),
		`UPDATE pairing_codes SET consumed_at = ? WHERE code = ?`,
		now, req.Code)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "code_consume_failed", err)
		return
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "commit_failed", err)
		return
	}

	resp := pairRedeemResp{
		DeviceID:    deviceID,
		BearerToken: token,
	}
	resp.ServerInfo.Version = "0.0.1-phase0"
	writeJSON(w, http.StatusOK, resp)
}

// ─── APNs token registration ───────────────────────────────────────────────

type registerAPNSReq struct {
	APNSToken string `json:"apns_token"`
}

func (s *Server) handleRegisterAPNS(w http.ResponseWriter, r *http.Request) {
	deviceID, _ := auth.DeviceID(r.Context())
	var req registerAPNSReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err)
		return
	}
	if _, err := s.db.ExecContext(r.Context(),
		`UPDATE devices SET apns_token = ? WHERE id = ?`,
		req.APNSToken, deviceID); err != nil {
		writeErr(w, http.StatusInternalServerError, "update_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── Me / Pillars ──────────────────────────────────────────────────────────

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	deviceID, _ := auth.DeviceID(r.Context())
	var userID, deviceName string
	var pairedAt int64
	err := s.db.QueryRowContext(r.Context(),
		`SELECT user_id, name, paired_at FROM devices WHERE id = ?`,
		deviceID).Scan(&userID, &deviceName, &pairedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "me_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_id":   deviceID,
		"device_name": deviceName,
		"user_id":     userID,
		"paired_at":   pairedAt,
	})
}

func (s *Server) handlePillars(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"finance": s.cfg.Pillars.Finance,
		"goals":   s.cfg.Pillars.Goals,
		"health":  s.cfg.Pillars.Health,
		"ai":      s.cfg.Pillars.AI,
	})
}

// ─── User settings ────────────────────────────────────────────────────────
// settings_json is a free-form blob persisted on users.settings_json. v1 keys:
//   daily_brief_time     ("07:30")  — fires daily_brief notification
//   evening_retro_time   ("21:00")  — fires evening_retro notification
//   weekly_retro_time    ("Fri 21:00")
//   timezone             ("America/New_York")

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	deviceID, _ := auth.DeviceID(r.Context())
	var userID string
	if err := s.db.QueryRowContext(r.Context(),
		`SELECT user_id FROM devices WHERE id = ?`, deviceID).Scan(&userID); err != nil {
		writeErr(w, http.StatusInternalServerError, "lookup_failed", err)
		return
	}
	var raw string
	if err := s.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(settings_json,'{}') FROM users WHERE id = ?`, userID).Scan(&raw); err != nil {
		writeErr(w, http.StatusInternalServerError, "fetch_failed", err)
		return
	}
	settings := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &settings)
	// Defaults
	if _, ok := settings["daily_brief_time"]; !ok {
		settings["daily_brief_time"] = "07:30"
	}
	if _, ok := settings["evening_retro_time"]; !ok {
		settings["evening_retro_time"] = "21:00"
	}
	if _, ok := settings["timezone"]; !ok {
		settings["timezone"] = "UTC"
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handlePatchSettings(w http.ResponseWriter, r *http.Request) {
	deviceID, _ := auth.DeviceID(r.Context())
	var userID string
	if err := s.db.QueryRowContext(r.Context(),
		`SELECT user_id FROM devices WHERE id = ?`, deviceID).Scan(&userID); err != nil {
		writeErr(w, http.StatusInternalServerError, "lookup_failed", err)
		return
	}
	var raw string
	if err := s.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(settings_json,'{}') FROM users WHERE id = ?`, userID).Scan(&raw); err != nil {
		writeErr(w, http.StatusInternalServerError, "fetch_failed", err)
		return
	}
	current := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &current)

	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err)
		return
	}
	for k, v := range patch {
		if v == nil {
			delete(current, k)
		} else {
			current[k] = v
		}
	}
	b, _ := json.Marshal(current)
	if _, err := s.db.ExecContext(r.Context(),
		`UPDATE users SET settings_json = ?, updated_at = ? WHERE id = ?`,
		string(b), time.Now().Unix(), userID); err != nil {
		writeErr(w, http.StatusInternalServerError, "update_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, current)
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// checkAdminToken gates /api/pair/initiate. When NORTHSTAR_ADMIN_TOKEN is
// unset (fresh install / dev), it logs once per boot and lets requests through
// so first-time pairing isn't blocked. When set, requires a constant-time
// match against Authorization: Bearer <token>.
func (s *Server) checkAdminToken(r *http.Request) bool {
	if s.cfg.AdminToken == "" {
		s.warnAdminOpen()
		return true
	}
	const prefix = "Bearer "
	got := r.Header.Get("Authorization")
	if !strings.HasPrefix(got, prefix) {
		return false
	}
	got = got[len(prefix):]
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.AdminToken)) == 1
}

func (s *Server) warnAdminOpen() {
	s.adminWarnOnce.Do(func() {
		log.Printf("WARN: NORTHSTAR_ADMIN_TOKEN is unset — /api/pair/initiate is open. " +
			"Restrict at the network layer (LAN / Tailscale) or set the env var.")
	})
}

func (s *Server) getOrCreateSingletonUser(r *http.Request, email string) (string, error) {
	var id string
	err := s.db.QueryRowContext(r.Context(), `SELECT id FROM users LIMIT 1`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	now := time.Now().Unix()
	_, err = s.db.ExecContext(r.Context(),
		`INSERT INTO users (id, email, settings_json, created_at, updated_at) VALUES (?, ?, '{}', ?, ?)`,
		id, email, now, now)
	if err != nil {
		return "", err
	}
	return id, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   code,
		"message": err.Error(),
	})
}
