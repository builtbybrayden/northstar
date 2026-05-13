package finance

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
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
	CategoryGroup  string `json:"category_group"`
	MonthlyCents   int64  `json:"monthly_cents"`
	Rationale      string `json:"rationale"`
	ThresholdPcts  []int  `json:"threshold_pcts"`
	PushEnabled    bool   `json:"push_enabled"`
}

func (h *Handlers) ListBudgetTargets(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT category, COALESCE(category_group,''), monthly_cents,
		        COALESCE(rationale,''), threshold_pcts, push_enabled
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
		if err := rows.Scan(&d.Category, &d.CategoryGroup, &d.MonthlyCents,
			&d.Rationale, &pctRaw, &pushInt); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if d.CategoryGroup == "" {
			d.CategoryGroup = DefaultGroupFor(d.Category)
		}
		d.ThresholdPcts = parseThresholdsForDTO(pctRaw)
		d.PushEnabled = pushInt == 1
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

type budgetTargetUpdate struct {
	MonthlyCents   *int64  `json:"monthly_cents,omitempty"`
	ThresholdPcts  *[]int  `json:"threshold_pcts,omitempty"`
	PushEnabled    *bool   `json:"push_enabled,omitempty"`
	CategoryGroup  *string `json:"category_group,omitempty"`
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
	if u.CategoryGroup != nil {
		g := strings.TrimSpace(*u.CategoryGroup)
		if g == "" {
			// Empty value resets to the default heuristic.
			g = DefaultGroupFor(category)
		}
		sets = append(sets, "category_group = ?")
		args = append(args, g)
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
		group := DefaultGroupFor(category)
		if u.CategoryGroup != nil && strings.TrimSpace(*u.CategoryGroup) != "" {
			group = strings.TrimSpace(*u.CategoryGroup)
		}
		_, err := h.DB.ExecContext(r.Context(),
			`INSERT INTO fin_budget_targets (category, monthly_cents, rationale, threshold_pcts, push_enabled, category_group, updated_at)
			 VALUES (?, ?, 'user', ?, ?, ?, ?)`,
			category, *u.MonthlyCents, pcts, push, group, time.Now().Unix())
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
	ID               string `json:"id"`
	AccountID        string `json:"account_id"`
	AccountName      string `json:"account_name"`
	Date             string `json:"date"`
	Payee            string `json:"payee"`
	Category         string `json:"category"`
	CategoryOriginal string `json:"category_original,omitempty"`
	AmountCents      int64  `json:"amount_cents"`
	Notes            string `json:"notes"`
}

func (h *Handlers) Transactions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		// Defensive parse — clamp to a sensible range.
		if n, ok := atoiSafe(v); ok && n > 0 && n <= 500 {
			limit = n
		}
	}
	// Starting Balances are Actual's one-time onboarding rows used to align
	// the account with the real-world balance. They aren't real purchases and
	// they bury actual activity in the feed. Hide them by default; flag
	// `?include_starting=1` brings them back.
	includeStarting := r.URL.Query().Get("include_starting") == "1"
	startingFilter := ""
	if !includeStarting {
		startingFilter = ` WHERE COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) != 'Starting Balances'`
	}

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT t.actual_id, t.account_id, COALESCE(a.name,''), t.date,
		        COALESCE(t.payee,''),
		        COALESCE(NULLIF(t.category_user, ''), COALESCE(t.category,'')) AS effective,
		        CASE WHEN NULLIF(t.category_user,'') IS NOT NULL
		             THEN COALESCE(t.category,'') ELSE '' END AS original,
		        t.amount_cents, COALESCE(t.notes,'')
		   FROM fin_transactions t
		   LEFT JOIN fin_accounts a ON a.actual_id = t.account_id`+startingFilter+`
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
			&t.Payee, &t.Category, &t.CategoryOriginal,
			&t.AmountCents, &t.Notes); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, out)
}

// ─── PATCH /api/finance/transactions/:id ──────────────────────────────────

// transactionUpdate represents a partial transaction edit from the iOS app.
// `Category` uses a json.RawMessage so we can distinguish:
//   - field omitted entirely (no change)
//   - field set to `null`        (reset override; revert to upstream)
//   - field set to a string      (apply override)
// We don't expose payee/amount/account edits — those belong in Actual.
type transactionUpdate struct {
	Category json.RawMessage `json:"category"`
}

func (h *Handlers) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrMsg(w, http.StatusBadRequest, "id required")
		return
	}
	var u transactionUpdate
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "bad json")
		return
	}

	// Load upstream category so we can auto-clear the override when the user
	// picks the same value Actual already had.
	var upstream string
	switch err := h.DB.QueryRowContext(r.Context(),
		`SELECT COALESCE(category,'') FROM fin_transactions WHERE actual_id = ?`, id).
		Scan(&upstream); err {
	case sql.ErrNoRows:
		writeErrMsg(w, http.StatusNotFound, "transaction not found")
		return
	case nil:
	default:
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	if u.Category == nil || len(u.Category) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "noop": true})
		return
	}

	var newOverride sql.NullString
	if string(u.Category) == "null" {
		// explicit reset
		newOverride.Valid = false
	} else {
		var s string
		if err := json.Unmarshal(u.Category, &s); err != nil {
			writeErrMsg(w, http.StatusBadRequest, "category must be a string or null")
			return
		}
		s = trimSpaces(s)
		if s == "" || s == upstream {
			// User picked nothing or matched upstream → clear override.
			newOverride.Valid = false
		} else {
			newOverride.Valid = true
			newOverride.String = s
		}
	}

	_, err := h.DB.ExecContext(r.Context(),
		`UPDATE fin_transactions SET category_user = ? WHERE actual_id = ?`,
		nullStringToValue(newOverride), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	effective := upstream
	if newOverride.Valid {
		effective = newOverride.String
	}
	original := ""
	if newOverride.Valid {
		original = upstream
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"id":                id,
		"category":          effective,
		"category_original": original,
	})
}

func nullStringToValue(n sql.NullString) any {
	if !n.Valid {
		return nil
	}
	return n.String
}

func trimSpaces(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// ─── /api/finance/investments ────────────────────────────────────────────

type investmentAccountDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	BalanceCents int64   `json:"balance_cents"`
	PctOfTotal   float64 `json:"pct_of_total"`
	Group        string  `json:"group"` // equity | crypto | retirement | real_estate | cash | other
}

type investmentsResponse struct {
	TotalCents int64                   `json:"total_cents"`
	Groups     map[string]int64        `json:"groups"`
	Accounts   []investmentAccountDTO  `json:"accounts"`
}

// Investments returns a breakdown of off-budget account balances grouped
// into asset classes. Off-budget is Actual's flag for accounts you want to
// track but not budget against — investments, retirement, real-estate, etc.
// The grouping is a name-based heuristic; users can override the mapping
// by renaming an account in Actual.
func (h *Handlers) Investments(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT actual_id, name, balance_cents
		   FROM fin_accounts
		  WHERE closed = 0 AND on_budget = 0
		  ORDER BY balance_cents DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	resp := investmentsResponse{
		Groups:   map[string]int64{},
		Accounts: []investmentAccountDTO{},
	}
	for rows.Next() {
		var a investmentAccountDTO
		if err := rows.Scan(&a.ID, &a.Name, &a.BalanceCents); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		a.Group = classifyInvestmentAccount(a.Name)
		resp.Accounts = append(resp.Accounts, a)
		resp.TotalCents += a.BalanceCents
		resp.Groups[a.Group] += a.BalanceCents
	}
	if resp.TotalCents > 0 {
		for i := range resp.Accounts {
			resp.Accounts[i].PctOfTotal = float64(resp.Accounts[i].BalanceCents) /
				float64(resp.TotalCents) * 100
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// classifyInvestmentAccount buckets an account by name keywords. Heuristic;
// returns "other" for anything that doesn't match.
func classifyInvestmentAccount(name string) string {
	lower := lowerASCII(name)
	switch {
	case containsAny(lower, "401k", "ira", "roth", "403b", "pension", "retirement"):
		return "retirement"
	case containsAny(lower, "crypto", "bitcoin", "btc", "eth"):
		return "crypto"
	case containsAny(lower, "house", "real estate", "property", "home"):
		return "real_estate"
	case containsAny(lower, "robinhood", "fidelity", "schwab", "vanguard", "etrade", "brokerage", "individual"):
		return "equity"
	case containsAny(lower, "hysa", "savings", "money market", "treasury"):
		return "cash"
	}
	return "other"
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if n == "" {
			continue
		}
		if indexOf(haystack, n) >= 0 {
			return true
		}
	}
	return false
}

func lowerASCII(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	if len(sub) > len(s) {
		return -1
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ─── /api/finance/forecast?days=N ────────────────────────────────────────

func (h *Handlers) ForecastEndpoint(w http.ResponseWriter, r *http.Request) {
	days := 90
	if v := r.URL.Query().Get("days"); v != "" {
		if n, ok := atoiSafe(v); ok && n > 0 && n <= 365 {
			days = n
		}
	}
	f, err := Compute(r.Context(), h.DB, time.Now().UTC(), days)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// ─── /api/finance/summary?month=YYYY-MM ────────────────────────────────────

type categorySummary struct {
	Category      string `json:"category"`
	CategoryGroup string `json:"category_group"`
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
		   AND COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) NOT IN ('Transfer','Starting Balances')`, monthLike).
		Scan(&resp.IncomeCents, &resp.SpentCents); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	// Per-category spend joined to budget targets. The effective category
	// honors per-row user overrides (category_user) when present.
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT b.category,
		        COALESCE(b.category_group,'') AS grp,
		        COALESCE(-(SELECT SUM(amount_cents) FROM fin_transactions t
		                   JOIN fin_accounts a ON a.actual_id = t.account_id
		                   WHERE COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) = b.category
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
		if err := rows.Scan(&c.Category, &c.CategoryGroup, &c.SpentCents, &c.BudgetedCents); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if c.CategoryGroup == "" {
			c.CategoryGroup = DefaultGroupFor(c.Category)
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
