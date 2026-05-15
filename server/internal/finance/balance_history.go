package finance

import (
	"net/http"
	"time"
)

// ─── /api/finance/balance-history?days=N ────────────────────────────────
//
// Returns one row per UTC day for the last N days (default 90, max 365),
// summarizing total on-budget + off-budget balances across all accounts
// that had a snapshot that day. Powers the net-worth line chart on iOS.
//
// Days with no snapshot are omitted, not zero-filled — the iOS chart
// interpolates between known points so a missed sync doesn't show a
// $0 dip.

type balanceHistoryDay struct {
	Date           string `json:"date"`
	NetWorthCents  int64  `json:"net_worth_cents"`
	OnBudgetCents  int64  `json:"on_budget_cents"`
	OffBudgetCents int64  `json:"off_budget_cents"`
}

type balanceHistoryResponse struct {
	Days []balanceHistoryDay `json:"days"`
}

func (h *Handlers) BalanceHistory(w http.ResponseWriter, r *http.Request) {
	days := 90
	if v := r.URL.Query().Get("days"); v != "" {
		if n, ok := atoiSafe(v); ok && n > 0 && n <= 365 {
			days = n
		}
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT date,
		        COALESCE(SUM(CASE WHEN on_budget=1 THEN balance_cents END), 0) AS on_b,
		        COALESCE(SUM(CASE WHEN on_budget=0 THEN balance_cents END), 0) AS off_b
		   FROM fin_account_balance_history
		  WHERE date >= ?
		  GROUP BY date
		  ORDER BY date ASC`, cutoff)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	resp := balanceHistoryResponse{Days: []balanceHistoryDay{}}
	for rows.Next() {
		var d balanceHistoryDay
		if err := rows.Scan(&d.Date, &d.OnBudgetCents, &d.OffBudgetCents); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		d.NetWorthCents = d.OnBudgetCents + d.OffBudgetCents
		resp.Days = append(resp.Days, d)
	}
	writeJSON(w, http.StatusOK, resp)
}
