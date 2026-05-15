package finance

import (
	"context"
	"fmt"
	"time"

	"github.com/builtbybrayden/northstar/server/internal/notify"
)

// ForecastWarning checks the cash-flow projection after each finance
// sync. If the minimum on-budget balance over the horizon drops below
// the configured floor (default $0), it fires a single `forecast_warning`
// notification per (month, floor-crossing) — dedup_key is bucketed on
// month so the user gets at most one warning per month per floor breach.
//
// Heuristic: a "warning-worthy" projection has at least one day where
// projected_balance < floor. We surface the FIRST such day plus the
// minimum-balance day separately, because "you'll go under by Tuesday"
// is more actionable than "your lowest day is the 28th".
type ForecastWarning struct {
	Detector              *Detector
	CashFloorCents        int64
	HorizonDays           int
}

func NewForecastWarning(d *Detector, floorCents int64, horizonDays int) *ForecastWarning {
	if horizonDays <= 0 {
		horizonDays = 30
	}
	return &ForecastWarning{
		Detector:       d,
		CashFloorCents: floorCents,
		HorizonDays:    horizonDays,
	}
}

// Check runs one projection and fires a notification if the minimum
// balance over the horizon is below the floor. Safe to call after every
// sync — notification dedup handles the "fire once per month" semantics.
func (fw *ForecastWarning) Check(ctx context.Context, now time.Time) error {
	if fw.Detector == nil || fw.Detector.Notifier == nil {
		return nil
	}
	f, err := Compute(ctx, fw.Detector.DB, now, fw.HorizonDays)
	if err != nil {
		return fmt.Errorf("forecast_warning: compute: %w", err)
	}
	firstUnder, minDay := findDip(f.Projected, fw.CashFloorCents)
	if firstUnder == nil {
		return nil
	}
	firstUnderfloor := firstUnder

	month := now.UTC().Format("2006-01")
	floorLabel := "$0"
	if fw.CashFloorCents > 0 {
		floorLabel = "$" + dollars(fw.CashFloorCents)
	}

	body := fmt.Sprintf(
		"Projected to drop below %s on %s (lowest: $%s on %s). Current: $%s. Horizon: %d days.",
		floorLabel, firstUnderfloor.Date, dollars(minDay.BalanceCents), minDay.Date,
		dollars(f.CurrentBalanceCents), fw.HorizonDays)

	priority := notify.PriHigh
	if minDay.BalanceCents < 0 {
		priority = notify.PriCritical
	}

	_, _, err = fw.Detector.Notifier.Fire(ctx, notify.Event{
		Category: notify.CatForecastWarning,
		Priority: priority,
		Title:    fmt.Sprintf("Cash dip ahead — under %s by %s", floorLabel, firstUnderfloor.Date),
		Body:     body,
		// One warning per month per floor crossing. If the projection
		// gets worse later in the month (lower minimum), the dedup is
		// looser to allow a re-fire only when the floor is RAISED.
		DedupKey: fmt.Sprintf("forecast_warning:%s:floor=%d", month, fw.CashFloorCents),
		Payload: map[string]any{
			"first_underfloor_date": firstUnderfloor.Date,
			"first_underfloor_cents": firstUnderfloor.BalanceCents,
			"min_balance_date":       minDay.Date,
			"min_balance_cents":      minDay.BalanceCents,
			"current_balance_cents":  f.CurrentBalanceCents,
			"floor_cents":            fw.CashFloorCents,
			"horizon_days":           fw.HorizonDays,
		},
	})
	return err
}

// findDip scans the projection for (1) the first day where balance dips
// below floor, and (2) the minimum-balance day overall. Returns nil
// firstUnder if the projection never crosses floor. Pure function —
// straightforward to test without spinning up notifications.
func findDip(days []ForecastDay, floorCents int64) (firstUnder *ForecastDay, minDay ForecastDay) {
	if len(days) == 0 {
		return nil, ForecastDay{}
	}
	minDay = days[0]
	for i := range days {
		d := days[i]
		if d.BalanceCents < minDay.BalanceCents {
			minDay = d
		}
		if firstUnder == nil && d.BalanceCents < floorCents {
			firstUnder = &days[i]
		}
	}
	return firstUnder, minDay
}
