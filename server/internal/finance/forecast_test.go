package finance

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func forecastDB(t *testing.T) *sql.DB {
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
		   updated_at INTEGER NOT NULL)`,
		`CREATE TABLE fin_transactions (
		   actual_id TEXT PRIMARY KEY, account_id TEXT, date TEXT,
		   payee TEXT, category TEXT, amount_cents INTEGER NOT NULL,
		   notes TEXT, imported_at INTEGER NOT NULL,
		   category_user TEXT,
		   transfer_id TEXT,
		   is_parent INTEGER NOT NULL DEFAULT 0)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return d
}

func seedAcct(t *testing.T, db *sql.DB, id string, balance int64, onBudget bool) {
	t.Helper()
	b := 0
	if onBudget {
		b = 1
	}
	if _, err := db.Exec(`INSERT INTO fin_accounts
		(actual_id, name, type, balance_cents, on_budget, closed, updated_at)
		VALUES (?, ?, '', ?, ?, 0, 0)`, id, id, balance, b); err != nil {
		t.Fatal(err)
	}
}

func seedTxnDated(t *testing.T, db *sql.DB, id, acct, date, payee, category string, amount int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO fin_transactions
		(actual_id, account_id, date, payee, category, amount_cents, notes, imported_at)
		VALUES (?, ?, ?, ?, ?, ?, '', 0)`, id, acct, date, payee, category, amount); err != nil {
		t.Fatal(err)
	}
}

func TestDetectRecurring_RequiresThreeOfSixMonths(t *testing.T) {
	today := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	txns := []rawTxn{
		// "Netflix" appears in 4 months → recurring
		{date: mustDate("2026-01-08"), payee: "Netflix", amount: -1599},
		{date: mustDate("2026-02-08"), payee: "Netflix", amount: -1599},
		{date: mustDate("2026-03-08"), payee: "Netflix", amount: -1599},
		{date: mustDate("2026-04-08"), payee: "Netflix", amount: -1599},
		// "OneOff" only in 2 months → not recurring
		{date: mustDate("2026-03-10"), payee: "OneOff", amount: -5000},
		{date: mustDate("2026-04-10"), payee: "OneOff", amount: -5000},
	}
	got := detectRecurring(txns, today)
	if len(got) != 1 {
		t.Fatalf("expected 1 recurring, got %d: %+v", len(got), got)
	}
	if got[0].Label != "Netflix" || got[0].AmountCents != -1599 || got[0].DayOfMonth != 8 {
		t.Errorf("unexpected recurring: %+v", got[0])
	}
}

func TestDetectRecurring_DayOfMonthClampedTo28(t *testing.T) {
	today := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	txns := []rawTxn{
		{date: mustDate("2026-01-31"), payee: "BigBill", amount: -10000},
		{date: mustDate("2026-02-28"), payee: "BigBill", amount: -10000},
		{date: mustDate("2026-03-31"), payee: "BigBill", amount: -10000},
	}
	got := detectRecurring(txns, today)
	if len(got) != 1 {
		t.Fatalf("expected 1 recurring, got %d", len(got))
	}
	if got[0].DayOfMonth > 28 {
		t.Errorf("day_of_month %d should be clamped ≤28", got[0].DayOfMonth)
	}
}

func TestDetectRecurring_PreservesSign(t *testing.T) {
	today := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	txns := []rawTxn{
		// Salary inflow
		{date: mustDate("2026-01-01"), payee: "Payroll", amount: 500000},
		{date: mustDate("2026-02-01"), payee: "Payroll", amount: 500000},
		{date: mustDate("2026-03-01"), payee: "Payroll", amount: 500000},
		// Mortgage outflow
		{date: mustDate("2026-01-01"), payee: "Mortgage", amount: -200000},
		{date: mustDate("2026-02-01"), payee: "Mortgage", amount: -200000},
		{date: mustDate("2026-03-01"), payee: "Mortgage", amount: -200000},
	}
	got := detectRecurring(txns, today)
	if len(got) != 2 {
		t.Fatalf("expected 2 recurring, got %d", len(got))
	}
	// Larger absolute amount sorted first
	if got[0].Label != "Payroll" || got[0].AmountCents != 500000 {
		t.Errorf("first should be Payroll inflow, got %+v", got[0])
	}
	if got[1].Label != "Mortgage" || got[1].AmountCents != -200000 {
		t.Errorf("second should be Mortgage outflow, got %+v", got[1])
	}
}

func TestCompute_HorizonClampedAndProjectionLength(t *testing.T) {
	db := forecastDB(t)
	seedAcct(t, db, "a1", 1_000_000, true)
	today := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)

	for _, days := range []int{30, 90, 365, 500} {
		f, err := Compute(context.Background(), db, today, days)
		if err != nil {
			t.Fatalf("Compute(%d): %v", days, err)
		}
		want := days
		if want > 365 {
			want = 365
		}
		if f.HorizonDays != want {
			t.Errorf("horizon = %d, want %d", f.HorizonDays, want)
		}
		if len(f.Projected) != want {
			t.Errorf("projected len = %d, want %d", len(f.Projected), want)
		}
	}
}

func TestCompute_RecurringHitsLandOnExpectedDay(t *testing.T) {
	db := forecastDB(t)
	seedAcct(t, db, "a1", 0, true)
	// Seed a $1000 monthly inflow on day 1 of 4 different months in the
	// trailing window.
	for _, m := range []string{"2026-01-01", "2026-02-01", "2026-03-01", "2026-04-01"} {
		seedTxnDated(t, db, "salary-"+m, "a1", m, "Payroll", "Salary", 100000)
	}
	today := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	f, err := Compute(context.Background(), db, today, 60)
	if err != nil {
		t.Fatal(err)
	}

	// Should detect Payroll as recurring with day_of_month=1
	if len(f.Recurring) != 1 || f.Recurring[0].Label != "Payroll" || f.Recurring[0].DayOfMonth != 1 {
		t.Fatalf("recurring = %+v", f.Recurring)
	}
	// Find the next two June/July 1st cells and confirm they hold inflows.
	var hits int
	for _, day := range f.Projected {
		for _, ev := range day.Events {
			if ev.Label == "Payroll" && ev.AmountCents == 100000 {
				hits++
			}
		}
	}
	if hits < 2 {
		t.Errorf("expected ≥2 Payroll hits in 60-day window, got %d", hits)
	}
}

func TestCompute_TransferCategoryExcluded(t *testing.T) {
	db := forecastDB(t)
	seedAcct(t, db, "a1", 0, true)
	for _, m := range []string{"2026-01-15", "2026-02-15", "2026-03-15"} {
		seedTxnDated(t, db, "xfer-"+m, "a1", m, "InternalShuffle", "Transfer", -50000)
	}
	today := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	f, err := Compute(context.Background(), db, today, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Recurring) != 0 {
		t.Errorf("Transfer-category recurring should be excluded: %+v", f.Recurring)
	}
}

func TestCompute_OverrideMovesRecurringToFollowEffectiveCategory(t *testing.T) {
	// Recurring detection itself doesn't depend on category, but the
	// "exclude Transfer" gate does. So an override that re-tags a
	// non-Transfer txn AS Transfer should make it drop out.
	db := forecastDB(t)
	seedAcct(t, db, "a1", 0, true)
	for _, m := range []string{"2026-01-15", "2026-02-15", "2026-03-15"} {
		seedTxnDated(t, db, "x-"+m, "a1", m, "AmbiguousMove", "Restaurants", -100)
	}
	// Override one of them to Transfer — it shouldn't matter for the count
	// (still 3 months under "Restaurants" effective), so we apply the
	// override to ALL three.
	if _, err := db.Exec(`UPDATE fin_transactions SET category_user='Transfer'`); err != nil {
		t.Fatal(err)
	}
	today := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	f, err := Compute(context.Background(), db, today, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Recurring) != 0 {
		t.Errorf("override → Transfer should drop the cohort: %+v", f.Recurring)
	}
}

func TestCompute_OnBudgetBalanceOnly(t *testing.T) {
	db := forecastDB(t)
	seedAcct(t, db, "checking", 10_000, true)
	seedAcct(t, db, "house", 5_000_000, false) // off-budget; should be excluded
	today := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	f, err := Compute(context.Background(), db, today, 7)
	if err != nil {
		t.Fatal(err)
	}
	if f.CurrentBalanceCents != 10_000 {
		t.Errorf("current = %d, want 10000 (off-budget house excluded)", f.CurrentBalanceCents)
	}
}

func TestCompute_LowPointMilestoneFiresOnMaterialDrop(t *testing.T) {
	db := forecastDB(t)
	seedAcct(t, db, "a1", 100_000, true)
	today := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	// Seed 90 days of unique-payee outflows ending on `today`, each $5.
	// 90 days × $5 = $450 burn / 90 = $5/day. Over a 90-day horizon that's
	// $450, which is >5% of the $1,000 starting balance → low-point fires.
	for d := 0; d < 90; d++ {
		date := today.AddDate(0, 0, -d).Format("2006-01-02")
		seedTxnDated(t, db, "burn-"+itoa(int64(d)), "a1", date,
			"Var"+itoa(int64(d)), "Misc", -500)
	}
	f, err := Compute(context.Background(), db, today, 90)
	if err != nil {
		t.Fatal(err)
	}
	if f.DailyDiscretionary <= 0 {
		t.Fatalf("expected positive daily discretionary, got %d", f.DailyDiscretionary)
	}
	var sawLow bool
	for _, m := range f.Milestones {
		if m.Label == "Projected low point" {
			sawLow = true
		}
	}
	if !sawLow {
		t.Errorf("expected 'Projected low point' milestone, got: %+v", f.Milestones)
	}
}

func TestForecastEndpoint_Smoke(t *testing.T) {
	db := forecastDB(t)
	seedAcct(t, db, "a1", 1_000_000, true)
	h := NewHandlers(db)
	r := httptest.NewRequest(http.MethodGet, "/api/finance/forecast?days=30", nil)
	w := httptest.NewRecorder()
	h.ForecastEndpoint(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d  body=%s", w.Code, w.Body.String())
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
