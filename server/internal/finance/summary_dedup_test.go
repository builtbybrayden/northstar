package finance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Regression test for the 2-3x inflation the user reported on 2026-05-15.
// Without the transfer_id / is_parent filters, a $2000 CC payment shows
// up as +$2000 income (CC account) and -$2000 spend (checking), and a
// split parent doubles every line of that transaction. This test seeds
// one paycheck + one CC payment + one split parent and asserts the
// summary only reports the paycheck as income.
func TestSummary_ExcludesTransfersAndSplitParents(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)

	month := time.Now().UTC().Format("2006-01")
	day := time.Now().UTC().Format("2006-01-02")

	mustExecSaved(t, db, `INSERT INTO fin_accounts
		(actual_id, name, type, balance_cents, on_budget, closed, updated_at) VALUES
		('chk', 'Chase Checking', '', 0, 1, 0, 0),
		('cc',  'Amex Platinum',  '', 0, 1, 0, 0)`)

	// Paycheck +5200 on checking → real income
	mustExecSaved(t, db, `INSERT INTO fin_transactions
		(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at,
		 transfer_id, is_parent) VALUES
		('paycheck', 'chk', '`+day+`', 'Employer', 'Salary', 520000, '', 0, NULL, 0)`)

	// CC payment: -2000 on checking, +2000 on CC, BOTH linked via transfer_id.
	// Without the filter, this would inflate income by 2000 and spend by 2000.
	mustExecSaved(t, db, `INSERT INTO fin_transactions
		(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at,
		 transfer_id, is_parent) VALUES
		('ccp-out', 'chk', '`+day+`', 'Payment to Amex', '', -200000, '', 0, 'ccp-in', 0),
		('ccp-in',  'cc',  '`+day+`', 'Payment from Chase', '', 200000, '', 0, 'ccp-out', 0)`)

	// Split parent: -300 gross + two children at -100 / -200 with categories.
	// Without is_parent filter we'd double-count -300 (parent) + -300 (children).
	mustExecSaved(t, db, `INSERT INTO fin_transactions
		(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at,
		 transfer_id, is_parent) VALUES
		('split-p',  'chk', '`+day+`', 'Costco', '', -30000, '', 0, NULL, 1),
		('split-c1', 'chk', '`+day+`', 'Costco', 'Groceries', -10000, '', 0, NULL, 0),
		('split-c2', 'chk', '`+day+`', 'Costco', 'Pet Care',  -20000, '', 0, NULL, 0)`)

	// Starting balance bootstrap — should be filtered by the payee fallback
	// even though the category is NULL.
	mustExecSaved(t, db, `INSERT INTO fin_transactions
		(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at,
		 transfer_id, is_parent) VALUES
		('sb-init',  'chk', '`+day+`', 'Initial Balance', NULL, 1000000, '', 0, NULL, 0),
		('sb-open',  'cc',  '`+day+`', 'Opening Balance Adjustment', NULL, 500000, '', 0, NULL, 0)`)

	r := httptest.NewRequest(http.MethodGet, "/api/finance/summary?month="+month, nil)
	w := httptest.NewRecorder()
	h.Summary(w, r)

	if w.Code != 200 {
		t.Fatalf("status: %d body=%s", w.Code, w.Body.String())
	}
	var resp summaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.IncomeCents != 520000 {
		t.Errorf("income = %d, want 520000 (paycheck only; CC payment + starting balances must be filtered)", resp.IncomeCents)
	}
	// Spend = 100 + 200 = 30000 cents (split children, NOT the parent, NOT the CC payment)
	if resp.SpentCents != 30000 {
		t.Errorf("spent = %d, want 30000 (split children only; parent + CC payment must be filtered)", resp.SpentCents)
	}
}
