package finance

import (
	"context"
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

func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
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
	// Optional filters used by the iOS category drilldown view.
	categoryFilter := r.URL.Query().Get("category")
	monthFilter := r.URL.Query().Get("month") // YYYY-MM
	// `flow` selects one of the three donut-tap views:
	//   spent  → on-budget outflows, transfers/parents/SB excluded
	//   income → on-budget inflows,  transfers/parents/SB excluded
	//   saved  → inflows into savings destination accounts that are
	//            user-initiated transfers (transfer_id IS NOT NULL)
	flowFilter := r.URL.Query().Get("flow")

	// Starting Balances are Actual's one-time onboarding rows used to align
	// the account with the real-world balance. They aren't real purchases and
	// they bury actual activity in the feed. Hide them by default; flag
	// `?include_starting=1` brings them back.
	//
	// Two-condition filter: category match catches rows Actual tagged
	// (most), payee match catches rows where the category is empty/null on
	// Actual's side (~33% of starting-balance rows in real-world data).
	includeStarting := r.URL.Query().Get("include_starting") == "1"

	clauses := []string{}
	args := []any{}
	if !includeStarting {
		clauses = append(clauses,
			`COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) != 'Starting Balances'`,
			`LOWER(COALESCE(t.payee,'')) NOT LIKE 'starting balance%'`)
	}
	if categoryFilter != "" {
		clauses = append(clauses,
			`COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) = ?`)
		args = append(args, categoryFilter)
	}
	if monthFilter != "" {
		clauses = append(clauses, `t.date LIKE ?`)
		args = append(args, monthFilter+"-%")
	}

	// flow= filters mirror the donut math one-to-one so the drilldown
	// list always reconciles to the headline number.
	var savingsDestIDs []string
	switch flowFilter {
	case "spent":
		clauses = append(clauses,
			`a.on_budget = 1`,
			`t.amount_cents < 0`,
			`t.transfer_id IS NULL`,
			`t.is_parent = 0`,
			`COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) != 'Transfer'`,
			`LOWER(COALESCE(t.payee,'')) NOT LIKE 'initial balance%'`,
			`LOWER(COALESCE(t.payee,'')) NOT LIKE 'opening balance%'`)
	case "income":
		clauses = append(clauses,
			`a.on_budget = 1`,
			`t.amount_cents > 0`,
			`t.transfer_id IS NULL`,
			`t.is_parent = 0`,
			`COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) != 'Transfer'`,
			`LOWER(COALESCE(t.payee,'')) NOT LIKE 'initial balance%'`,
			`LOWER(COALESCE(t.payee,'')) NOT LIKE 'opening balance%'`)
	case "saved":
		// Resolve savings destination accounts via the same heuristic
		// that powers computeSavedCents, then constrain to inflows that
		// are real transfers (transfer_id IS NOT NULL).
		ids, err := savingsDestinationIDs(r.Context(), h.DB)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if len(ids) == 0 {
			writeJSON(w, http.StatusOK, []transactionDTO{})
			return
		}
		savingsDestIDs = ids
		placeholders := stringsRepeatJoin("?", ",", len(ids))
		clauses = append(clauses,
			`t.amount_cents > 0`,
			`t.transfer_id IS NOT NULL`,
			`t.is_parent = 0`,
			`t.account_id IN (`+placeholders+`)`)
		for _, id := range ids {
			args = append(args, id)
		}
	}

	where := ""
	if len(clauses) > 0 {
		where = ` WHERE ` + joinAnd(clauses)
	}
	args = append(args, limit)
	_ = savingsDestIDs // documented above; the IDs are spliced into args

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT t.actual_id, t.account_id, COALESCE(a.name,''), t.date,
		        COALESCE(t.payee,''),
		        COALESCE(NULLIF(t.category_user, ''), COALESCE(t.category,'')) AS effective,
		        CASE WHEN NULLIF(t.category_user,'') IS NOT NULL
		             THEN COALESCE(t.category,'') ELSE '' END AS original,
		        t.amount_cents, COALESCE(t.notes,'')
		   FROM fin_transactions t
		   LEFT JOIN fin_accounts a ON a.actual_id = t.account_id`+where+`
		   ORDER BY t.date DESC, t.imported_at DESC
		   LIMIT ?`, args...)
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

// excludeNonSpendSQL is the predicate that drops zero-sum and bootstrap
// rows from aggregate calculations. Centralized so the txn list, summary,
// forecast, and AI tools all stay in sync. Five conditions:
//
//  1. Category != 'Transfer' — Actual's well-defined transfer category
//  2. Category != 'Starting Balances' — bootstrap import
//  3. Payee NOT LIKE 'starting/initial/opening balance%' — Actual leaves
//     ~30% of bootstrap rows with NULL category, payee fallback catches them
//  4. transfer_id IS NULL — covers both legs of any inter-account move
//     (CC payments, savings funding, broker deposits) regardless of how
//     Actual categorized them. Sourced from r.transfer_id on each side.
//  5. is_parent = 0 — split parents carry the gross amount that the
//     children already sum to; counting both double-counts the line.
const excludeNonSpendSQL = `COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) NOT IN ('Transfer','Starting Balances')
   AND LOWER(COALESCE(t.payee,'')) NOT LIKE 'starting balance%'
   AND LOWER(COALESCE(t.payee,'')) NOT LIKE 'initial balance%'
   AND LOWER(COALESCE(t.payee,'')) NOT LIKE 'opening balance%'
   AND t.transfer_id IS NULL
   AND t.is_parent = 0`

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

// savingsDestinationIDs returns the actual_ids of every open account
// flagged by isSavingsDestination. Used by /transactions?flow=saved
// and the saved-cents aggregator. Empty slice if none match.
func savingsDestinationIDs(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT actual_id, name FROM fin_accounts WHERE closed = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		if isSavingsDestination(name) {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// stringsRepeatJoin returns e.g. "?,?,?,?" for ("?", ",", 4). Saves us
// pulling in strings.Repeat + strings.TrimSuffix for a one-liner.
func stringsRepeatJoin(token, sep string, n int) string {
	if n <= 0 {
		return ""
	}
	if n == 1 {
		return token
	}
	return strings.Repeat(token+sep, n-1) + token
}

// isSavingsDestination returns true when an account looks like a place
// money goes to be saved, invested, or set aside for retirement. Used by
// the Finance summary to compute saved_cents as the month's contributions
// into those accounts (positive inflows), regardless of whether they're
// on-budget or off-budget in Actual.
//
// Real-estate accounts are explicitly excluded — house appreciation isn't
// "saving" in the cash-flow sense, and we don't want a Zillow re-estimate
// to inflate the donut.
func isSavingsDestination(name string) bool {
	lower := lowerASCII(name)
	// Retirement
	if containsAny(lower, "401k", "ira", "roth", "403b", "pension", "retirement") {
		return true
	}
	// Brokerage / equity
	if containsAny(lower, "robinhood", "fidelity", "schwab", "vanguard", "etrade", "brokerage") {
		return true
	}
	// Crypto
	if containsAny(lower, "crypto", "bitcoin", "btc", "eth") {
		return true
	}
	// Cash savings — Amex Savings, Chase Savings, HYSAs, money markets.
	// Deliberately requires the "savings" keyword to avoid catching
	// "Amex Checking" or "Chase Credit Card".
	if containsAny(lower, "savings", "hysa", "money market") {
		return true
	}
	return false
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
	// SavingsTargetPct is the user-configured savings target as a
	// percentage of income. Default 25. iOS uses it to compute the
	// saved donut denominator: target_cents = income × pct / 100.
	SavingsTargetPct    int               `json:"savings_target_pct"`
	SavingsTargetCents  int64             `json:"savings_target_cents"`
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
		   AND `+excludeNonSpendSQL+``, monthLike).
		Scan(&resp.IncomeCents, &resp.SpentCents); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	// Per-category spend joined to budget targets. The effective category
	// honors per-row user overrides (category_user) when present. Transfers
	// and split parents are excluded so a mis-categorized CC payment
	// can't inflate a budget line.
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT b.category,
		        COALESCE(b.category_group,'') AS grp,
		        COALESCE(-(SELECT SUM(amount_cents) FROM fin_transactions t
		                   JOIN fin_accounts a ON a.actual_id = t.account_id
		                   WHERE COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) = b.category
		                     AND t.date LIKE ?
		                     AND t.amount_cents < 0
		                     AND a.on_budget = 1
		                     AND t.transfer_id IS NULL
		                     AND t.is_parent = 0), 0) AS spent,
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

	// SavedCents = sum of positive inflows this month into accounts whose
	// name flags them as savings/retirement/brokerage destinations. This
	// replaces the earlier "income - spent" definition, which collapsed to
	// a meaningless residual when on-budget income lagged on-budget spend.
	//
	// Starting Balance bootstrap rows are excluded so the figure reflects
	// new contributions only, not the initial balance import.
	saved, err := computeSavedCents(r.Context(), h.DB, monthLike)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	resp.SavedCents = saved

	settings, err := readFinanceSettings(r.Context(), h.DB)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	resp.SavingsTargetPct = settings.SavingsTargetPct
	resp.SavingsTargetCents = (resp.IncomeCents * int64(settings.SavingsTargetPct)) / 100

	writeJSON(w, http.StatusOK, resp)
}

// computeSavedCents sums positive transactions during `monthLike` (e.g.
// "2026-05-%") on every account flagged by isSavingsDestination, excluding
// Starting Balance bootstrap rows.
func computeSavedCents(ctx context.Context, db *sql.DB, monthLike string) (int64, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT actual_id, name FROM fin_accounts WHERE closed = 0`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var destIDs []string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return 0, err
		}
		if isSavingsDestination(name) {
			destIDs = append(destIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(destIDs) == 0 {
		return 0, nil
	}

	placeholders := strings.Repeat("?,", len(destIDs)-1) + "?"
	args := make([]any, 0, len(destIDs)+1)
	args = append(args, monthLike)
	for _, id := range destIDs {
		args = append(args, id)
	}
	var saved int64
	// "Saved this month" = the user-initiated transfers INTO savings
	// destinations. Requires transfer_id IS NOT NULL so balance imports,
	// starting balances, market gains, and external direct deposits
	// don't count — only money the user deliberately moved from one of
	// their accounts to a savings destination. This matches the user's
	// mental model ("transfers into Robinhood / 401K / Amex+Chase
	// savings this month"); paycheck→401K direct deposits land outside
	// this definition by design, since the employer isn't an Actual
	// account and we can't distinguish them from balance updates.
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(t.amount_cents), 0)
		   FROM fin_transactions t
		  WHERE t.date LIKE ?
		    AND t.amount_cents > 0
		    AND t.account_id IN (`+placeholders+`)
		    AND t.transfer_id IS NOT NULL
		    AND t.is_parent = 0`,
		args...).Scan(&saved); err != nil {
		return 0, err
	}
	return saved, nil
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
