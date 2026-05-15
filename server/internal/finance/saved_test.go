package finance

import (
	"context"
	"database/sql"
	"testing"
)

func TestIsSavingsDestination(t *testing.T) {
	cases := map[string]bool{
		// Retirement
		"401K Fidelity":      true,
		"Roth IRA":           true,
		"Vanguard 403b":      true,
		"Employer Pension":   true,
		"Retirement Account": true,
		// Brokerage / equity
		"Robinhood":           true,
		"Schwab Brokerage":    true,
		"Fidelity Individual": true,
		// Crypto
		"Coinbase BTC": true,
		"Crypto Wallet": true,
		// Cash savings — the user's stated targets
		"Amex Savings": true,
		"Chase Savings": true,
		"Ally HYSA":    true,
		"Money Market": true,
		// NOT savings destinations — must stay false
		"Amex Platinum":           false,
		"Chase Sapphire":          false,
		"Chase Checking":          false,
		"Wallet Cash":             false,
		"House (estimated value)": false,
		"Mortgage":                false,
		"":                        false,
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			got := isSavingsDestination(name)
			if got != want {
				t.Errorf("isSavingsDestination(%q) = %v, want %v", name, got, want)
			}
		})
	}
}

func TestComputeSavedCents(t *testing.T) {
	db := inMemFinanceDB(t)
	mustExecSaved(t, db, `INSERT INTO fin_accounts
		(actual_id, name, type, balance_cents, on_budget, closed, updated_at)
		VALUES
		('rh', 'Robinhood', '', 0, 0, 0, 0),
		('ch', 'Chase Checking', '', 0, 1, 0, 0),
		('old', 'Old Amex Savings', '', 0, 1, 1, 0)`)
	mustExecSaved(t, db, `INSERT INTO fin_transactions
		(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at)
		VALUES
		('t1', 'rh', '2026-05-03', 'ACH Transfer', '', 50000, '', 0),
		('t2', 'rh', '2026-05-15', 'Buy VOO',      '', 25000, '', 0),
		('t3', 'rh', '2026-05-01', 'Starting Balance', 'Starting Balances', 999999, '', 0),
		('t4', 'ch', '2026-05-04', 'Paycheck',    'Salary', 500000, '', 0),
		('t5', 'old','2026-05-10', 'Old deposit',  '', 20000, '', 0),
		('t6', 'rh', '2026-04-20', 'Old deposit',  '', 99999, '', 0)`)

	saved, err := computeSavedCents(context.Background(), db, "2026-05-%")
	if err != nil {
		t.Fatalf("computeSavedCents: %v", err)
	}
	// Want: t1 (50000) + t2 (25000) = 75000. Excluded:
	// t3 starting balance, t4 checking (not a destination),
	// t5 closed account, t6 April (wrong month).
	if want := int64(75000); saved != want {
		t.Errorf("saved = %d, want %d", saved, want)
	}
}

func TestComputeSavedCents_NoDestinationAccounts(t *testing.T) {
	db := inMemFinanceDB(t)
	mustExecSaved(t, db, `INSERT INTO fin_accounts
		(actual_id, name, type, balance_cents, on_budget, closed, updated_at)
		VALUES ('ch', 'Chase Checking', '', 0, 1, 0, 0)`)
	mustExecSaved(t, db, `INSERT INTO fin_transactions
		(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at)
		VALUES ('t1', 'ch', '2026-05-03', 'Paycheck', 'Salary', 500000, '', 0)`)

	saved, err := computeSavedCents(context.Background(), db, "2026-05-%")
	if err != nil {
		t.Fatalf("computeSavedCents: %v", err)
	}
	if saved != 0 {
		t.Errorf("no-destination saved = %d, want 0", saved)
	}
}

func mustExecSaved(t *testing.T, db *sql.DB, q string) {
	t.Helper()
	if _, err := db.Exec(q); err != nil {
		t.Fatalf("exec: %v\n%s", err, q)
	}
}
