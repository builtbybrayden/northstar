package finance

import "testing"

func TestFindDip_NoCrossing(t *testing.T) {
	days := []ForecastDay{
		{Date: "2026-05-15", BalanceCents: 500000},
		{Date: "2026-05-16", BalanceCents: 480000},
		{Date: "2026-05-17", BalanceCents: 510000},
	}
	first, minDay := findDip(days, 0)
	if first != nil {
		t.Errorf("firstUnder should be nil when nothing crosses floor; got %+v", *first)
	}
	if minDay.Date != "2026-05-16" || minDay.BalanceCents != 480000 {
		t.Errorf("minDay = %+v, want 2026-05-16 at 480000", minDay)
	}
}

func TestFindDip_CrossesZero(t *testing.T) {
	days := []ForecastDay{
		{Date: "2026-05-15", BalanceCents: 50000},
		{Date: "2026-05-16", BalanceCents: 20000},
		{Date: "2026-05-17", BalanceCents: -10000},
		{Date: "2026-05-18", BalanceCents: -30000},
		{Date: "2026-05-19", BalanceCents: 5000},
	}
	first, minDay := findDip(days, 0)
	if first == nil {
		t.Fatal("firstUnder should fire when projection goes negative")
	}
	if first.Date != "2026-05-17" {
		t.Errorf("firstUnder.Date = %s, want 2026-05-17 (first sub-zero day)", first.Date)
	}
	if minDay.Date != "2026-05-18" || minDay.BalanceCents != -30000 {
		t.Errorf("minDay = %+v, want 2026-05-18 at -30000", minDay)
	}
}

func TestFindDip_RespectsCustomFloor(t *testing.T) {
	// Floor at $500. Some days are above $500 but below the cash floor.
	days := []ForecastDay{
		{Date: "2026-05-15", BalanceCents: 200000},
		{Date: "2026-05-16", BalanceCents: 80000},
		{Date: "2026-05-17", BalanceCents: 30000},
		{Date: "2026-05-18", BalanceCents: 90000},
	}
	first, _ := findDip(days, 50000) // floor = $500
	if first == nil {
		t.Fatal("expected fire when balance drops below floor $500")
	}
	if first.Date != "2026-05-17" {
		t.Errorf("firstUnder = %s, want 2026-05-17 (first day under $500)", first.Date)
	}
}

func TestFindDip_EmptyProjection(t *testing.T) {
	first, minDay := findDip(nil, 0)
	if first != nil {
		t.Errorf("empty projection should return nil firstUnder, got %+v", *first)
	}
	if minDay.Date != "" {
		t.Errorf("empty projection should return zero-value minDay, got %+v", minDay)
	}
}
