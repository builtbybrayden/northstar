package finance

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

// Forecast projects on-budget cash balance forward N days using detected
// recurring transactions plus an average discretionary spend rate.
//
// It is a single-user heuristic, NOT a financial model. Use it for "do I
// have a cash crunch coming up?" — not for retirement planning.
type Forecast struct {
	AsOf                string            `json:"as_of"`
	HorizonDays         int               `json:"horizon_days"`
	CurrentBalanceCents int64             `json:"current_balance_cents"`
	DailyDiscretionary  int64             `json:"daily_discretionary_cents"`
	Projected           []ForecastDay     `json:"projected"`
	Milestones          []ForecastEvent   `json:"milestones"`
	Recurring           []RecurringCharge `json:"recurring"`
}

// ForecastDay is one cell of the projection. `events` carries any recurring
// hits that landed on this day so the UI can annotate the line chart.
type ForecastDay struct {
	Date         string          `json:"date"`
	BalanceCents int64           `json:"balance_cents"`
	Events       []ForecastEvent `json:"events,omitempty"`
}

type ForecastEvent struct {
	Date        string `json:"date,omitempty"`
	Label       string `json:"label"`
	AmountCents int64  `json:"amount_cents,omitempty"`
}

type RecurringCharge struct {
	Label       string  `json:"label"`        // typically the payee
	AmountCents int64   `json:"amount_cents"` // signed; negative = outflow
	DayOfMonth  int     `json:"day_of_month"` // 1..28 clamped (so Feb still fires)
	Confidence  float64 `json:"confidence"`   // 0..1; fraction of recent months it appeared in
}

// rawTxn is the projection input — a minimal on-budget txn read from SQLite.
type rawTxn struct {
	date   time.Time
	payee  string
	amount int64 // signed cents
}

// Compute runs the projection. `today` is injected so tests can pin the date.
func Compute(ctx context.Context, db *sql.DB, today time.Time, horizonDays int) (Forecast, error) {
	if horizonDays <= 0 {
		horizonDays = 90
	}
	if horizonDays > 365 {
		horizonDays = 365
	}

	// 1. Current on-budget balance. Off-budget accounts (investments, house
	//    value) are intentionally excluded — they're not cash you can spend.
	var balance int64
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(balance_cents),0) FROM fin_accounts
		  WHERE closed=0 AND on_budget=1`).Scan(&balance); err != nil {
		return Forecast{}, err
	}

	// 2. Pull last 180 days of on-budget txns, honoring user category
	//    overrides so recurring detection matches what the dashboard shows.
	cutoff := today.AddDate(0, 0, -180).Format("2006-01-02")
	rows, err := db.QueryContext(ctx,
		`SELECT t.date, COALESCE(t.payee,''), t.amount_cents
		   FROM fin_transactions t
		   JOIN fin_accounts a ON a.actual_id = t.account_id
		  WHERE a.on_budget = 1 AND t.date >= ?
		    AND COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) NOT IN ('Transfer','Starting Balances')`, cutoff)
	if err != nil {
		return Forecast{}, err
	}
	defer rows.Close()

	var txns []rawTxn
	for rows.Next() {
		var r rawTxn
		var dateStr string
		if err := rows.Scan(&dateStr, &r.payee, &r.amount); err != nil {
			return Forecast{}, err
		}
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		r.date = t
		txns = append(txns, r)
	}
	if err := rows.Err(); err != nil {
		return Forecast{}, err
	}

	recurring := detectRecurring(txns, today)
	discretionary := dailyDiscretionary(txns, recurring, today)

	// 3. Day-by-day simulation.
	projected := make([]ForecastDay, 0, horizonDays)
	current := balance
	var milestones []ForecastEvent
	lowestDate := ""
	lowestBalance := balance
	for d := 1; d <= horizonDays; d++ {
		day := today.AddDate(0, 0, d)
		dayStr := day.Format("2006-01-02")
		dayOfMonth := day.Day()

		// Discretionary draws first so a same-day recurring inflow doesn't mask it.
		current -= discretionary

		var dayEvents []ForecastEvent
		for _, r := range recurring {
			if r.DayOfMonth == dayOfMonth {
				current += r.AmountCents
				dayEvents = append(dayEvents, ForecastEvent{
					Label:       r.Label,
					AmountCents: r.AmountCents,
				})
				milestones = append(milestones, ForecastEvent{
					Date:        dayStr,
					Label:       r.Label,
					AmountCents: r.AmountCents,
				})
			}
		}

		projected = append(projected, ForecastDay{
			Date:         dayStr,
			BalanceCents: current,
			Events:       dayEvents,
		})
		if current < lowestBalance {
			lowestBalance = current
			lowestDate = dayStr
		}
	}

	// 4. Flag the trough as a milestone if the projection dips materially
	//    below the starting balance (>5% drop).
	if lowestDate != "" && lowestBalance < balance-balance/20 {
		milestones = append(milestones, ForecastEvent{
			Date:  lowestDate,
			Label: "Projected low point",
		})
	}
	sort.SliceStable(milestones, func(i, j int) bool { return milestones[i].Date < milestones[j].Date })

	return Forecast{
		AsOf:                today.Format("2006-01-02"),
		HorizonDays:         horizonDays,
		CurrentBalanceCents: balance,
		DailyDiscretionary:  discretionary,
		Projected:           projected,
		Milestones:          milestones,
		Recurring:           recurring,
	}, nil
}

// detectRecurring groups transactions into cohorts keyed by (payee, bucketed
// amount, sign). A cohort that appears in ≥3 distinct months of the trailing
// 6 is treated as recurring. Returns the median day-of-month for each kept
// cohort, clamped to 1..28 so month-end charges still fire in February.
func detectRecurring(txns []rawTxn, today time.Time) []RecurringCharge {
	type cohort struct {
		payee       string
		amount      int64
		months      map[string]struct{}
		daysOfMonth []int
	}
	cohorts := map[string]*cohort{}

	sixMonthsAgo := today.AddDate(0, -6, 0)
	for _, t := range txns {
		if t.date.Before(sixMonthsAgo) {
			continue
		}
		if t.payee == "" || t.amount == 0 {
			continue
		}
		key := t.payee + "|" + bucketAmount(t.amount) + "|" + signKey(t.amount)
		c, ok := cohorts[key]
		if !ok {
			c = &cohort{
				payee:  t.payee,
				amount: t.amount,
				months: map[string]struct{}{},
			}
			cohorts[key] = c
		}
		c.months[t.date.Format("2006-01")] = struct{}{}
		c.daysOfMonth = append(c.daysOfMonth, t.date.Day())
	}

	var out []RecurringCharge
	for _, c := range cohorts {
		if len(c.months) < 3 {
			continue
		}
		dom := medianDay(c.daysOfMonth)
		if dom > 28 {
			dom = 28
		}
		if dom < 1 {
			dom = 1
		}
		out = append(out, RecurringCharge{
			Label:       c.payee,
			AmountCents: c.amount,
			DayOfMonth:  dom,
			Confidence:  float64(len(c.months)) / 6.0,
		})
	}
	// Stable sort: outflows by size descending, then label, so the UI lists
	// the largest recurring charge first.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].AmountCents != out[j].AmountCents {
			return absInt64(out[i].AmountCents) > absInt64(out[j].AmountCents)
		}
		return out[i].Label < out[j].Label
	})
	return out
}

// dailyDiscretionary is the average per-day on-budget outflow over the last
// 90 days, excluding anything we've already classified as recurring (so
// rent doesn't get counted twice).
func dailyDiscretionary(txns []rawTxn, recurring []RecurringCharge, today time.Time) int64 {
	recurringKey := map[string]bool{}
	for _, r := range recurring {
		recurringKey[r.Label+"|"+bucketAmount(r.AmountCents)+"|"+signKey(r.AmountCents)] = true
	}
	ninetyAgo := today.AddDate(0, 0, -90)
	var total int64
	for _, t := range txns {
		if t.date.Before(ninetyAgo) || t.amount >= 0 {
			continue
		}
		key := t.payee + "|" + bucketAmount(t.amount) + "|" + signKey(t.amount)
		if recurringKey[key] {
			continue
		}
		total += -t.amount
	}
	return total / 90
}

// bucketAmount rounds the absolute amount to the nearest dollar so two
// occurrences of a recurring charge with tiny variance still cluster.
func bucketAmount(cents int64) string {
	abs := absInt64(cents)
	dollars := (abs + 50) / 100
	return itoa(dollars)
}

func signKey(cents int64) string {
	if cents < 0 {
		return "-"
	}
	return "+"
}

func medianDay(days []int) int {
	if len(days) == 0 {
		return 1
	}
	cp := make([]int, len(days))
	copy(cp, days)
	sort.Ints(cp)
	return cp[len(cp)/2]
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// itoa avoids pulling strconv into the hot path. Forecast only formats
// small unsigned ints (rounded dollar amounts), so a custom impl keeps
// allocations down without the strconv overhead.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
