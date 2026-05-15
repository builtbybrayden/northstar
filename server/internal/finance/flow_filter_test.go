package finance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// /api/finance/transactions?flow=X must return exactly the rows that
// roll up to the summary's headline number for that flow. This test
// seeds the same shape as the dedup regression and asserts each flow
// returns the expected subset.
func TestTransactions_FlowFilter(t *testing.T) {
	db := inMemFinanceDB(t)
	h := NewHandlers(db)

	mustExecSaved(t, db, `INSERT INTO fin_accounts
		(actual_id, name, type, balance_cents, on_budget, closed, updated_at) VALUES
		('chk', 'Chase Checking', '', 0, 1, 0, 0),
		('rh',  'Robinhood',     '', 0, 0, 0, 0),
		('cc',  'Amex Platinum', '', 0, 1, 0, 0)`)
	mustExecSaved(t, db, `INSERT INTO fin_transactions
		(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at, transfer_id, is_parent) VALUES
		('paycheck', 'chk', '2026-05-04', 'Employer',           'Salary', 520000, '', 0, NULL, 0),
		('rent-in',  'chk', '2026-05-01', 'Renter',             'Salary', 220000, '', 0, NULL, 0),
		('coffee',   'chk', '2026-05-05', 'Starbucks',          'Restaurants', -1200, '', 0, NULL, 0),
		('groc',     'cc',  '2026-05-06', 'Kroger',             'Groceries',   -8500, '', 0, NULL, 0),
		('ccp-out',  'chk', '2026-05-10', 'Payment to Amex',    'Transfer', -200000, '', 0, 'ccp-in', 0),
		('ccp-in',   'cc',  '2026-05-10', 'Payment from Chase', 'Transfer',  200000, '', 0, 'ccp-out', 0),
		('save-out', 'chk', '2026-05-12', 'Transfer to RH',     'Transfer', -100000, '', 0, 'save-in', 0),
		('save-in',  'rh',  '2026-05-12', 'Transfer from Chase','Transfer',  100000, '', 0, 'save-out', 0),
		('split-p',  'chk', '2026-05-14', 'Costco',             '',          -30000, '', 0, NULL, 1),
		('split-c',  'chk', '2026-05-14', 'Costco',             'Groceries', -30000, '', 0, NULL, 0),
		('sb',       'rh',  '2026-05-01', 'Initial Balance',    NULL,      9000000, '', 0, NULL, 0)`)

	for _, tc := range []struct {
		flow  string
		wantIDs []string
	}{
		{"income", []string{"paycheck", "rent-in"}},
		{"spent",  []string{"split-c", "groc", "coffee"}}, // ordered DESC by date
		{"saved",  []string{"save-in"}},
	} {
		tc := tc
		t.Run(tc.flow, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet,
				"/api/finance/transactions?flow="+tc.flow+"&month=2026-05&limit=50", nil)
			w := httptest.NewRecorder()
			h.Transactions(w, r)
			if w.Code != 200 {
				t.Fatalf("status: %d body=%s", w.Code, w.Body.String())
			}
			var got []transactionDTO
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("flow=%s rows=%d, want %d (%+v)", tc.flow, len(got), len(tc.wantIDs), got)
			}
			for i, want := range tc.wantIDs {
				if got[i].ID != want {
					t.Errorf("flow=%s pos=%d id=%s, want %s", tc.flow, i, got[i].ID, want)
				}
			}
		})
	}
}
