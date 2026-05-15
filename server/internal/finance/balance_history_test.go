package finance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBalanceHistory_GroupsAndOrders(t *testing.T) {
	db := inMemFinanceDB(t)

	// Migration 00008 adds fin_account_balance_history; the in-mem schema
	// helper doesn't include it (kept minimal). Create it here.
	mustExecSaved(t, db, `CREATE TABLE fin_account_balance_history (
		account_id    TEXT    NOT NULL,
		date          TEXT    NOT NULL,
		balance_cents INTEGER NOT NULL,
		on_budget     INTEGER NOT NULL DEFAULT 1,
		PRIMARY KEY (account_id, date))`)

	today := time.Now().UTC().Format("2006-01-02")
	yest := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	tooOld := time.Now().UTC().AddDate(0, 0, -120).Format("2006-01-02")

	mustExecSaved(t, db, `INSERT INTO fin_account_balance_history
		(account_id, date, balance_cents, on_budget) VALUES
		('chk', '`+yest+`', 250000, 1),
		('rh',  '`+yest+`', 1500000, 0),
		('chk', '`+today+`', 240000, 1),
		('rh',  '`+today+`', 1525000, 0),
		('chk', '`+tooOld+`', 100, 1)`)

	h := NewHandlers(db)
	r := httptest.NewRequest(http.MethodGet, "/api/finance/balance-history?days=90", nil)
	w := httptest.NewRecorder()
	h.BalanceHistory(w, r)

	if w.Code != 200 {
		t.Fatalf("status: %d body=%s", w.Code, w.Body.String())
	}
	var resp balanceHistoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Days) != 2 {
		t.Fatalf("days = %d, want 2 (today + yesterday; tooOld dropped); got %+v", len(resp.Days), resp.Days)
	}
	if resp.Days[0].Date != yest {
		t.Errorf("first day = %s, want %s (chronological order)", resp.Days[0].Date, yest)
	}
	wantNW := int64(250000 + 1500000)
	if resp.Days[0].NetWorthCents != wantNW {
		t.Errorf("yest net_worth = %d, want %d", resp.Days[0].NetWorthCents, wantNW)
	}
	if resp.Days[0].OnBudgetCents != 250000 || resp.Days[0].OffBudgetCents != 1500000 {
		t.Errorf("yest split = on=%d off=%d, want 250000/1500000",
			resp.Days[0].OnBudgetCents, resp.Days[0].OffBudgetCents)
	}
}

func TestBalanceHistory_EmptyTable(t *testing.T) {
	db := inMemFinanceDB(t)
	mustExecSaved(t, db, `CREATE TABLE fin_account_balance_history (
		account_id    TEXT    NOT NULL,
		date          TEXT    NOT NULL,
		balance_cents INTEGER NOT NULL,
		on_budget     INTEGER NOT NULL DEFAULT 1,
		PRIMARY KEY (account_id, date))`)

	h := NewHandlers(db)
	r := httptest.NewRequest(http.MethodGet, "/api/finance/balance-history", nil)
	w := httptest.NewRecorder()
	h.BalanceHistory(w, r)
	if w.Code != 200 {
		t.Fatalf("status: %d body=%s", w.Code, w.Body.String())
	}
	var resp balanceHistoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Days) != 0 {
		t.Errorf("empty table should yield 0 days, got %d", len(resp.Days))
	}
}
