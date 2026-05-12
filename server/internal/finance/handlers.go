package finance

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handlers groups the finance REST endpoints. Mounted at /api/finance/* by the
// router. All handlers expect bearer auth applied upstream.
type Handlers struct {
	DB *sql.DB
}

func NewHandlers(db *sql.DB) *Handlers {
	return &Handlers{DB: db}
}

// ─── /api/finance/budget-targets ──────────────────────────────────────────

type budgetTargetDTO struct {
	Category       string `json:"category"`
	MonthlyCents   int64  `json:"monthly_cents"`
	Rationale      string `json:"rationale"`
	ThresholdPcts  []int  `json:"threshold_pcts"`
	PushEnabled    bool   `json:"push_enabled"`
}

func (h *Handlers) ListBudgetTargets(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT category, monthly_cents, COALESCE(rationale,''), threshold_pcts, push_enabled
		   FROM fin_budget_targets ORDER BY category ASC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	out := []budgetTargetDTO{}
	for rows.Next() {
		var d budgetTargetDTO
		var pctRaw string
		var pushInt int
		if err := rows.Scan(&d.Category, &d.MonthlyCents, &d.Rationale, &pctRaw, &pushInt); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		d.ThresholdPcts = parseThresholdsForDTO(pctRaw)
		d.PushEnabled = pushInt == 1
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

type budgetTargetUpdate struct {
	MonthlyCents   *int64 `json:"monthly_cents,omitempty"`
	ThresholdPcts  *[]int `json:"threshold_pcts,omitempty"`
	PushEnabled    *bool  `json:"push_enabled,omitempty"`
}

func (h *Handlers) UpdateBudgetTarget(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "category")
	if category == "" {
		writeErrMsg(w, http.StatusBadRequest, "category required")
		return
	}
	var u budgetTargetUpdate
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}

	sets := []string{}
	args := []any{}
	if u.MonthlyCents != nil {
		if *u.MonthlyCents < 0 {
			writeErrMsg(w, http.StatusBadRequest, "monthly_cents must be >= 0")
			return
		}
		sets = append(sets, "monthly_cents = ?")
		args = append(args, *u.MonthlyCents)
	}
	if u.ThresholdPcts != nil {
		for _, p := range *u.ThresholdPcts {
			if p < 1 || p > 200 {
				writeErrMsg(w, http.StatusBadRequest, "threshold pcts must be 1..200")
				return
			}
		}
		j, _ := json.Marshal(*u.ThresholdPcts)
		sets = append(sets, "threshold_pcts = ?")
		args = append(args, string(j))
	}
	if u.PushEnabled != nil {
		sets = append(sets, "push_enabled = ?")
		args = append(args, boolToInt(*u.PushEnabled))
	}
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "noop": true})
		return
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now().Unix())

	query := "UPDATE fin_budget_targets SET " + joinComma(sets) + " WHERE category = ?"
	args = append(args, category)

	res, err := h.DB.ExecContext(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Category didn't exist — insert it
		if u.MonthlyCents == nil {
			writeErrMsg(w, http.StatusBadRequest, "new category needs monthly_cents")
			return
		}
		pcts := "[50,75,90,100]"
		if u.ThresholdPcts != nil {
			j, _ := json.Marshal(*u.ThresholdPcts)
			pcts = string(j)
		}
		push := 1
		if u.PushEnabled != nil && !*u.PushEnabled {
			push = 0
		}
		_, err := h.DB.ExecContext(r.Context(),
			`INSERT INTO fin_budget_targets (category, monthly_cents, rationale, threshold_pcts, push_enabled, updated_at)
			 VALUES (?, ?, 'user', ?, ?, ?)`,
			category, *u.MonthlyCents, pcts, push, time.Now().Unix())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func parseThresholdsForDTO(raw string) []int {
	out := []int{}
	cur := 0
	have := false
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			cur = cur*10 + int(r-'0')
			have = true
		} else if have {
			out = append(out, cur)
			cur = 0
			have = false
		}
	}
	if have {
		out = append(out, cur)
	}
	return out
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


// ─── /api/finance/accounts ────────────────────────────────────────────────

type accountDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	BalanceCents int64  `json:"balance_cents"`
	OnBudget     bool   `json:"on_budget"`
	Closed       bool   `json:"closed"`
}

func (h *Handlers) Accounts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT actual_id, name, balance_cents, on_budget, closed
		   FROM fin_accounts WHERE closed = 0
		   ORDER BY on_budget DESC, name ASC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	out := []accountDTO{}
	for rows.Next() {
		var a accountDTO
		var onBudget, closed int
		if err := rows.Scan(&a.ID, &a.Name, &a.BalanceCents, &onBudget, &closed); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		a.OnBudget = onBudget == 1
		a.Closed = closed == 1
		out = append(out, a)
	}
	writeJSON(w, http.StatusOK, out)
}

// ─── /api/finance/transactions ────────────────────────────────────────────

type transactionDTO struct {
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
	AccountName  string `json:"account_name"`
	Date         string `json:"date"`
	Payee        string `json:"payee"`
	Category     string `json:"category"`
	AmountCents  int64  `json:"amount_cents"`
	Notes        string `json:"notes"`
}

func (h *Handlers) Transactions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		// Defensive parse — clamp to a sensible range.
		if n, ok := atoiSafe(v); ok && n > 0 && n <= 500 {
			limit = n
		}
	}

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT t.actual_id, t.account_id, COALESCE(a.name,''), t.date,
		        COALESCE(t.payee,''), COALESCE(t.category,''),
		        t.amount_cents, COALESCE(t.notes,'')
		   FROM fin_transactions t
		   LEFT JOIN fin_accounts a ON a.actual_id = t.account_id
		   ORDER BY t.date DESC, t.imported_at DESC
		   LIMIT ?`, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	out := []transactionDTO{}
	for rows.Next() {
		var t transactionDTO
		if err := rows.Scan(&t.ID, &t.AccountID, &t.AccountName, &t.Date,
			&t.Payee, &t.Category, &t.AmountCents, &t.Notes); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, out)
}

// ─── /api/finance/summary?month=YYYY-MM ────────────────────────────────────

type categorySummary struct {
	Category      string `json:"category"`
	SpentCents    int64  `json:"spent_cents"`
	BudgetedCents int64  `json:"budgeted_cents"`
	Pct           int    `json:"pct"`            // 0–100+ rounded
	Over          bool   `json:"over"`
}

type summaryResponse struct {
	Month               string            `json:"month"`
	NetWorthCents       int64             `json:"net_worth_cents"`
	OnBudgetCents       int64             `json:"on_budget_cents"`
	OffBudgetCents      int64             `json:"off_budget_cents"`
	IncomeCents         int64             `json:"income_cents"`
	SpentCents          int64             `json:"spent_cents"`
	BudgetedCents       int64             `json:"budgeted_cents"`
	SavedCents          int64             `json:"saved_cents"`
	Categories          []categorySummary `json:"categories"`
}

func (h *Handlers) Summary(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().UTC().Format("2006-01")
	}
	monthLike := month + "-%"

	resp := summaryResponse{Month: month, Categories: []categorySummary{}}

	// Net worth split
	if err := h.DB.QueryRowContext(r.Context(),
		`SELECT
		   COALESCE(SUM(CASE WHEN on_budget=1 THEN balance_cents END), 0),
		   COALESCE(SUM(CASE WHEN on_budget=0 THEN balance_cents END), 0)
		 FROM fin_accounts WHERE closed = 0`).
		Scan(&resp.OnBudgetCents, &resp.OffBudgetCents); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	resp.NetWorthCents = resp.OnBudgetCents + resp.OffBudgetCents

	// Income and spend for the month (joining account to filter on-budget only for spend)
	if err := h.DB.QueryRowContext(r.Context(),
		`SELECT
		   COALESCE(SUM(CASE WHEN t.amount_cents > 0 THEN t.amount_cents END), 0),
		   COALESCE(-SUM(CASE WHEN t.amount_cents < 0 THEN t.amount_cents END), 0)
		 FROM fin_transactions t
		 JOIN fin_accounts a ON a.actual_id = t.account_id
		 WHERE t.date LIKE ? AND a.on_budget = 1
		   AND COALESCE(t.category,'') != 'Transfer'`, monthLike).
		Scan(&resp.IncomeCents, &resp.SpentCents); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	// Per-category spend joined to budget targets
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT b.category,
		        COALESCE(-(SELECT SUM(amount_cents) FROM fin_transactions t
		                   JOIN fin_accounts a ON a.actual_id = t.account_id
		                   WHERE t.category = b.category
		                     AND t.date LIKE ?
		                     AND t.amount_cents < 0
		                     AND a.on_budget = 1), 0) AS spent,
		        b.monthly_cents
		   FROM fin_budget_targets b
		   ORDER BY b.category ASC`, monthLike)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	var totalBudgeted int64
	for rows.Next() {
		var c categorySummary
		if err := rows.Scan(&c.Category, &c.SpentCents, &c.BudgetedCents); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if c.BudgetedCents > 0 {
			c.Pct = int((c.SpentCents * 100) / c.BudgetedCents)
		}
		c.Over = c.SpentCents > c.BudgetedCents
		totalBudgeted += c.BudgetedCents
		resp.Categories = append(resp.Categories, c)
	}
	resp.BudgetedCents = totalBudgeted
	resp.SavedCents = resp.IncomeCents - resp.SpentCents

	writeJSON(w, http.StatusOK, resp)
}

// ─── Helpers ──────────────────────────────────────────────────────────────

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

func atoiSafe(s string) (int, bool) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
		if n > 1_000_000 {
			return 0, false
		}
	}
	return n, true
}
