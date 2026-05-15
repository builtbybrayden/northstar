package finance

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	_ "modernc.org/sqlite"
)

// inMemFinanceDB creates a minimal schema that matches the production migrations
// for what these handler tests exercise (fin_accounts + fin_transactions +
// fin_budget_targets including the 00004 category_user column).
func inMemFinanceDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	stmts := []string{
		`CREATE TABLE fin_accounts (
		   actual_id TEXT PRIMARY KEY, name TEXT, type TEXT,
		   balance_cents INTEGER NOT NULL DEFAULT 0,
		   on_budget INTEGER NOT NULL DEFAULT 1,
		   closed INTEGER NOT NULL DEFAULT 0,
		   updated_at INTEGER NOT NULL,
		   is_savings_destination INTEGER)`,
		`CREATE TABLE fin_transactions (
		   actual_id TEXT PRIMARY KEY, account_id TEXT, date TEXT,
		   payee TEXT, category TEXT, amount_cents INTEGER NOT NULL,
		   notes TEXT, imported_at INTEGER NOT NULL,
		   category_user TEXT,
		   transfer_id TEXT,
		   is_parent INTEGER NOT NULL DEFAULT 0)`,
		// category_group added by migration 00006; ordered last to match
		// what ALTER TABLE produces in production.
		`CREATE TABLE fin_budget_targets (
		   category TEXT PRIMARY KEY, monthly_cents INTEGER NOT NULL,
		   rationale TEXT,
		   threshold_pcts TEXT NOT NULL DEFAULT '[50,75,90,100]',
		   push_enabled INTEGER NOT NULL DEFAULT 1,
		   updated_at INTEGER NOT NULL,
		   category_group TEXT)`,
		`CREATE TABLE fin_settings (
		   id INTEGER PRIMARY KEY CHECK (id = 1),
		   savings_target_pct INTEGER NOT NULL DEFAULT 25,
		   updated_at INTEGER NOT NULL)`,
		`INSERT INTO fin_settings (id, savings_target_pct, updated_at)
		   VALUES (1, 25, 0)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return d
}

// seed an account + a single Groceries transaction.
func seedTxn(t *testing.T, db *sql.DB, id, category string) {
	t.Helper()
	_, err := db.Exec(`INSERT OR IGNORE INTO fin_accounts
		(actual_id, name, type, balance_cents, on_budget, closed, updated_at)
		VALUES ('a1', 'Chase', '', 0, 1, 0, 0)`)
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	_, err = db.Exec(`INSERT INTO fin_transactions
		(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at)
		VALUES (?, 'a1', '2026-05-10', 'Kroger', ?, -8412, '', 0)`, id, category)
	if err != nil {
		t.Fatalf("seed txn: %v", err)
	}
}

func patchTxn(t *testing.T, h *Handlers, id, body string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPatch, "/api/finance/transactions/"+id,
		bytes.NewReader([]byte(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.UpdateTransaction(w, r)
	var out map[string]any
	_ = json.NewDecoder(w.Body).Decode(&out)
	return w.Code, out
}

func readCategoryUser(t *testing.T, db *sql.DB, id string) sql.NullString {
	t.Helper()
	var s sql.NullString
	if err := db.QueryRow(
		`SELECT category_user FROM fin_transactions WHERE actual_id = ?`, id).
		Scan(&s); err != nil {
		t.Fatalf("query category_user: %v", err)
	}
	return s
}

func TestUpdateTransaction_SetOverride(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)
	seedTxn(t, db, "t1", "Groceries")

	code, out := patchTxn(t, h, "t1", `{"category":"Restaurants"}`)
	if code != 200 {
		t.Fatalf("status: %d  body=%v", code, out)
	}
	if out["category"] != "Restaurants" || out["category_original"] != "Groceries" {
		t.Errorf("unexpected body: %v", out)
	}
	got := readCategoryUser(t, db, "t1")
	if !got.Valid || got.String != "Restaurants" {
		t.Errorf("category_user = %+v, want valid 'Restaurants'", got)
	}
}

func TestUpdateTransaction_MatchUpstreamClearsOverride(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)
	seedTxn(t, db, "t1", "Groceries")
	// Start with an override in place.
	if _, err := db.Exec(`UPDATE fin_transactions SET category_user='Restaurants' WHERE actual_id='t1'`); err != nil {
		t.Fatal(err)
	}

	// Setting it back to the upstream value should clear the override, not
	// store a redundant "Groceries → Groceries" override.
	code, out := patchTxn(t, h, "t1", `{"category":"Groceries"}`)
	if code != 200 {
		t.Fatalf("status: %d  body=%v", code, out)
	}
	if out["category"] != "Groceries" {
		t.Errorf("category = %v, want Groceries", out["category"])
	}
	if out["category_original"] != "" {
		t.Errorf("category_original = %v, want empty", out["category_original"])
	}
	got := readCategoryUser(t, db, "t1")
	if got.Valid {
		t.Errorf("category_user should be NULL, got %+v", got)
	}
}

func TestUpdateTransaction_NullResetsOverride(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)
	seedTxn(t, db, "t1", "Groceries")
	if _, err := db.Exec(`UPDATE fin_transactions SET category_user='Restaurants' WHERE actual_id='t1'`); err != nil {
		t.Fatal(err)
	}

	code, out := patchTxn(t, h, "t1", `{"category":null}`)
	if code != 200 {
		t.Fatalf("status: %d  body=%v", code, out)
	}
	if out["category"] != "Groceries" {
		t.Errorf("category = %v, want Groceries", out["category"])
	}
	got := readCategoryUser(t, db, "t1")
	if got.Valid {
		t.Errorf("category_user should be NULL, got %+v", got)
	}
}

func TestUpdateTransaction_NotFound(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)
	code, _ := patchTxn(t, h, "ghost", `{"category":"Restaurants"}`)
	if code != 404 {
		t.Errorf("expected 404 for missing txn, got %d", code)
	}
}

func TestUpdateTransaction_BadJSONFails(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)
	seedTxn(t, db, "t1", "Groceries")
	code, _ := patchTxn(t, h, "t1", `{"category":42}`)
	if code != 400 {
		t.Errorf("expected 400 for non-string category, got %d", code)
	}
}

func TestListTransactions_ExposesEffectiveCategory(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)
	seedTxn(t, db, "t1", "Groceries")
	if _, err := db.Exec(`UPDATE fin_transactions SET category_user='Restaurants' WHERE actual_id='t1'`); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/finance/transactions", nil)
	w := httptest.NewRecorder()
	h.Transactions(w, r)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	body := w.Body.String()
	// Effective category is the override; category_original carries the
	// upstream value so the iOS detail sheet can show "Was: Groceries".
	if !strings.Contains(body, `"category":"Restaurants"`) {
		t.Errorf("missing effective category: %s", body)
	}
	if !strings.Contains(body, `"category_original":"Groceries"`) {
		t.Errorf("missing original category: %s", body)
	}
}

func TestInvestments_GroupsAndPercentages(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)
	_, err := db.Exec(`INSERT INTO fin_accounts
		(actual_id, name, type, balance_cents, on_budget, closed, updated_at)
		VALUES
		('rh-ind',   'Robinhood Individual',      '', 2840100, 0, 0, 0),
		('rh-cry',   'Robinhood Crypto',          '',  412800, 0, 0, 0),
		('401k',     'American Express 401k',     '', 1554200, 0, 0, 0),
		('house',    'House (estimated value)',   '',38500000, 0, 0, 0),
		('mort',     'Chase Mortgage (1040)',     '',-24800000,0, 0, 0),
		('check',    'Chase Checking',            '',  384217, 1, 0, 0)
	`)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/finance/investments", nil)
	w := httptest.NewRecorder()
	h.Investments(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d  body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		TotalCents int64            `json:"total_cents"`
		Groups     map[string]int64 `json:"groups"`
		Accounts   []struct {
			ID           string  `json:"id"`
			Name         string  `json:"name"`
			BalanceCents int64   `json:"balance_cents"`
			PctOfTotal   float64 `json:"pct_of_total"`
			Group        string  `json:"group"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	// On-budget Checking should be excluded.
	for _, a := range resp.Accounts {
		if a.ID == "check" {
			t.Errorf("on-budget account leaked into investments: %+v", a)
		}
	}
	// Classification spot-checks.
	for _, a := range resp.Accounts {
		switch a.ID {
		case "rh-ind":
			if a.Group != "equity" {
				t.Errorf("rh-ind group = %s, want equity", a.Group)
			}
		case "rh-cry":
			if a.Group != "crypto" {
				t.Errorf("rh-cry group = %s, want crypto", a.Group)
			}
		case "401k":
			if a.Group != "retirement" {
				t.Errorf("401k group = %s, want retirement", a.Group)
			}
		case "house":
			if a.Group != "real_estate" {
				t.Errorf("house group = %s, want real_estate", a.Group)
			}
		}
	}
	// Mortgage (negative balance, off-budget) is included — it's a liability
	// that nets against the house value. Total should be the sum.
	wantTotal := int64(2840100 + 412800 + 1554200 + 38500000 + -24800000)
	if resp.TotalCents != wantTotal {
		t.Errorf("total = %d, want %d", resp.TotalCents, wantTotal)
	}
	// Percentages should sum to ~100 (allowing for rounding).
	var sumPct float64
	for _, a := range resp.Accounts {
		sumPct += a.PctOfTotal
	}
	if sumPct < 99.5 || sumPct > 100.5 {
		t.Errorf("sum of percentages = %f, want ≈ 100", sumPct)
	}
}

func TestInvestments_ClassifyByName(t *testing.T) {
	cases := []struct {
		name, want string
	}{
		{"American Express 401k", "retirement"},
		{"Roth IRA", "retirement"},
		{"Robinhood Crypto", "crypto"},
		{"Coinbase BTC", "crypto"},
		{"House (estimated value)", "real_estate"},
		{"Fidelity Brokerage", "equity"},
		{"Schwab Individual", "equity"},
		{"Amex HYSA (4289)", "cash"},
		{"Treasury Direct", "cash"},
		{"Old timeshare", "other"},
	}
	for _, c := range cases {
		got := classifyInvestmentAccount(c.name)
		if got != c.want {
			t.Errorf("classify(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestListTransactions_CategoryAndMonthFilters(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)
	seedAcct := func(id string) {
		_, _ = db.Exec(`INSERT OR IGNORE INTO fin_accounts
			(actual_id, name, type, balance_cents, on_budget, closed, updated_at)
			VALUES (?, ?, '', 0, 1, 0, 0)`, id, id)
	}
	seedAcct("a1")
	mkTxn := func(id, date, payee, category string, amt int64) {
		_, _ = db.Exec(`INSERT INTO fin_transactions
			(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at)
			VALUES (?, 'a1', ?, ?, ?, ?, '', 0)`, id, date, payee, category, amt)
	}
	mkTxn("a", "2026-05-04", "Kroger",     "Groceries",   -3200)
	mkTxn("b", "2026-05-15", "Whole Foods","Groceries",   -8400)
	mkTxn("c", "2026-04-22", "Trader Joes","Groceries",   -2200)
	mkTxn("d", "2026-05-09", "Shell",      "Gas",         -4800)
	// SB ringer
	mkTxn("sb1", "2026-03-01", "Starting Balance", "", -50000)

	probe := func(query string) []transactionDTO {
		r := httptest.NewRequest(http.MethodGet, "/api/finance/transactions?"+query, nil)
		w := httptest.NewRecorder()
		h.Transactions(w, r)
		if w.Code != 200 {
			t.Fatalf("status %d  body=%s", w.Code, w.Body.String())
		}
		var got []transactionDTO
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		return got
	}

	// No filters → all current-period rows except starting balance
	all := probe("limit=50")
	for _, r := range all {
		if r.ID == "sb1" {
			t.Errorf("starting-balance row leaked through: %+v", r)
		}
	}
	if len(all) != 4 {
		t.Errorf("default list len=%d, want 4", len(all))
	}

	// category=Groceries → only Groceries rows
	groc := probe("category=Groceries")
	if len(groc) != 3 {
		t.Errorf("Groceries filter len=%d, want 3", len(groc))
	}
	for _, r := range groc {
		if r.Category != "Groceries" {
			t.Errorf("non-Groceries row: %+v", r)
		}
	}

	// category=Groceries&month=2026-05 → only May Groceries
	mayGroc := probe("category=Groceries&month=2026-05")
	if len(mayGroc) != 2 {
		t.Errorf("May Groceries len=%d, want 2", len(mayGroc))
	}

	// include_starting=1 brings the SB row back
	withSB := probe("include_starting=1&limit=50")
	if len(withSB) != 5 {
		t.Errorf("include_starting len=%d, want 5", len(withSB))
	}
}

func TestListTransactions_PayeeFiltersStartingBalanceWithEmptyCategory(t *testing.T) {
	// Real-world hole: ~33% of starting-balance rows have NULL category on
	// the Actual side, so the category-only filter let them through.
	db := inMemFinanceDB(t)
	h := NewHandlers(db)
	if _, err := db.Exec(`INSERT INTO fin_accounts
		(actual_id, name, type, balance_cents, on_budget, closed, updated_at)
		VALUES ('a1','Chase','',0,1,0,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO fin_transactions
		(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at)
		VALUES ('sb-empty', 'a1', '2026-03-01', 'Starting Balance', '', -50000, '', 0)`); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/finance/transactions", nil)
	w := httptest.NewRecorder()
	h.Transactions(w, r)
	if strings.Contains(w.Body.String(), `"sb-empty"`) {
		t.Errorf("SB with empty category leaked through: %s", w.Body.String())
	}
}

func TestSummary_OverrideMovesSpendBetweenCategories(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)
	// Seed both budgets so the override has somewhere to land.
	if _, err := db.Exec(`INSERT INTO fin_budget_targets
		(category, monthly_cents, rationale, threshold_pcts, push_enabled, updated_at)
		VALUES ('Groceries', 70000, '', '[50,75,90,100]', 1, 0),
		       ('Restaurants', 50000, '', '[50,75,90,100]', 1, 0)`); err != nil {
		t.Fatal(err)
	}
	// One transaction, currently in Groceries with an override to Restaurants.
	seedTxn(t, db, "t1", "Groceries")
	if _, err := db.Exec(`UPDATE fin_transactions SET category_user='Restaurants' WHERE actual_id='t1'`); err != nil {
		t.Fatal(err)
	}
	// Date the txn into the current month so the summary picks it up.
	if _, err := db.Exec(`UPDATE fin_transactions SET date = strftime('%Y-%m', 'now') || '-10' WHERE actual_id='t1'`); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/finance/summary", nil)
	w := httptest.NewRecorder()
	h.Summary(w, r)
	if w.Code != 200 {
		t.Fatalf("status: %d  body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Categories []struct {
			Category   string `json:"category"`
			SpentCents int64  `json:"spent_cents"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for _, c := range resp.Categories {
		switch c.Category {
		case "Groceries":
			if c.SpentCents != 0 {
				t.Errorf("Groceries should have 0 spend after override moves it: %d", c.SpentCents)
			}
		case "Restaurants":
			if c.SpentCents != 8412 {
				t.Errorf("Restaurants should hold the overridden spend: %d", c.SpentCents)
			}
		}
	}
}
