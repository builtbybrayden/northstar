package finance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// fin_settings is a single-row table (id=1) holding finance-pillar
// preferences. Currently just savings_target_pct, but the surface is
// stable so we can add more knobs (cash floor, default forecast
// horizon, etc.) without another migration.

type FinanceSettings struct {
	SavingsTargetPct int `json:"savings_target_pct"`
}

func readFinanceSettings(ctx context.Context, db *sql.DB) (FinanceSettings, error) {
	var s FinanceSettings
	err := db.QueryRowContext(ctx,
		`SELECT savings_target_pct FROM fin_settings WHERE id = 1`).
		Scan(&s.SavingsTargetPct)
	if errors.Is(err, sql.ErrNoRows) {
		// Migration 00011 seeds row 1 — if it's missing we're running
		// against an older DB. Re-seed defensively rather than 500'ing.
		s = FinanceSettings{SavingsTargetPct: 25}
		_, _ = db.ExecContext(ctx,
			`INSERT OR IGNORE INTO fin_settings (id, savings_target_pct, updated_at)
			 VALUES (1, 25, ?)`, time.Now().Unix())
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, nil
}

func (h *Handlers) GetFinanceSettings(w http.ResponseWriter, r *http.Request) {
	s, err := readFinanceSettings(r.Context(), h.DB)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

type financeSettingsUpdate struct {
	SavingsTargetPct *int `json:"savings_target_pct,omitempty"`
}

func (h *Handlers) UpdateFinanceSettings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var u financeSettingsUpdate
	if err := json.Unmarshal(body, &u); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if u.SavingsTargetPct == nil {
		writeErrMsg(w, http.StatusBadRequest, "no fields to update")
		return
	}
	if *u.SavingsTargetPct < 0 || *u.SavingsTargetPct > 100 {
		writeErrMsg(w, http.StatusBadRequest, "savings_target_pct must be between 0 and 100")
		return
	}
	if _, err := h.DB.ExecContext(r.Context(),
		`UPDATE fin_settings SET savings_target_pct = ?, updated_at = ? WHERE id = 1`,
		*u.SavingsTargetPct, time.Now().Unix()); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s, err := readFinanceSettings(r.Context(), h.DB)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}
