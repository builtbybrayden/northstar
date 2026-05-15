package finance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/builtbybrayden/northstar/server/internal/notify"
)

// Detector inspects new transactions during sync and fires notifications
// for purchases, budget-threshold crossings, and anomalies.
type Detector struct {
	DB       *sql.DB
	Notifier *notify.Composer

	// AnomalyMinAbsCents: a single transaction this big from a never-seen
	// payee fires anomaly. Default $200 = 20000 cents.
	AnomalyMinAbsCents int64
	// AnomalySpikeFactor: a payee's new charge is anomaly if it exceeds
	// median × this factor (and the payee has at least 4 prior charges).
	AnomalySpikeFactor float64

	// quietPurchases, when true, suppresses purchase + anomaly Fires for
	// the duration of an initial backfill so users aren't buried in
	// notifications on first sync. Set/cleared by Syncer.
	quietPurchases bool
}

func NewDetector(db *sql.DB, n *notify.Composer) *Detector {
	return &Detector{
		DB:                 db,
		Notifier:           n,
		AnomalyMinAbsCents: 20000,
		AnomalySpikeFactor: 3.0,
	}
}

// SetQuietPurchases toggles suppression of purchase + anomaly notifications.
// Syncer calls this around an initial-backfill SyncOnce so users aren't
// buried by per-txn pushes on first run.
func (d *Detector) SetQuietPurchases(q bool) { d.quietPurchases = q }

// OnNewTransaction fires the purchase + anomaly notifications for a single
// freshly-inserted transaction, and updates merchant stats.
func (d *Detector) OnNewTransaction(ctx context.Context, t SidecarTransaction, categoryName string) error {
	if t.Amount >= 0 {
		// Inflow (paychecks, refunds) — skip purchase + anomaly
		_ = d.touchMerchant(ctx, t)
		return nil
	}
	if d.quietPurchases {
		// Initial backfill — keep merchant stats current so later syncs detect
		// anomalies, but don't fire push for any of this batch.
		return d.touchMerchant(ctx, t)
	}
	abs := -t.Amount

	// Anomaly: new merchant + big charge
	prior, err := d.merchantStats(ctx, t.Payee)
	if err != nil {
		return err
	}
	isNewMerchant := prior == nil
	if isNewMerchant && abs >= d.AnomalyMinAbsCents {
		_, _, err := d.Notifier.Fire(ctx, notify.Event{
			Category: notify.CatAnomaly,
			Priority: notify.PriHigh,
			Title:    fmt.Sprintf("New merchant: %s  −$%s", t.Payee, dollars(abs)),
			Body:     fmt.Sprintf("First time spending here, %s.", categoryName),
			DedupKey: fmt.Sprintf("anomaly:new-merchant:%s:%s", t.Payee, t.ID),
			Payload: map[string]any{
				"transaction_id": t.ID,
				"payee":          t.Payee,
				"amount_cents":   t.Amount,
				"category":       categoryName,
				"reason":         "new_merchant_high_value",
			},
		})
		if err != nil {
			return err
		}
	} else if prior != nil && prior.TxnCount >= 4 && prior.MedianAbsCents > 0 {
		if float64(abs) > float64(prior.MedianAbsCents)*d.AnomalySpikeFactor {
			_, _, err := d.Notifier.Fire(ctx, notify.Event{
				Category: notify.CatAnomaly,
				Priority: notify.PriHigh,
				Title:    fmt.Sprintf("Unusual: %s  −$%s", t.Payee, dollars(abs)),
				Body: fmt.Sprintf("%.1f× your usual at this merchant.",
					float64(abs)/float64(prior.MedianAbsCents)),
				DedupKey: fmt.Sprintf("anomaly:spike:%s:%s", t.Payee, t.ID),
				Payload: map[string]any{
					"transaction_id": t.ID,
					"payee":          t.Payee,
					"amount_cents":   t.Amount,
					"category":       categoryName,
					"reason":         "median_spike",
					"median_cents":   prior.MedianAbsCents,
				},
			})
			if err != nil {
				return err
			}
		}
	}

	// Purchase notification — always fires (rate-limited by daily cap in rules)
	_, _, err = d.Notifier.Fire(ctx, notify.Event{
		Category: notify.CatPurchase,
		Priority: notify.PriNormal,
		Title:    fmt.Sprintf("%s  −$%s", t.Payee, dollars(abs)),
		Body:     fmt.Sprintf("%s", categoryName),
		DedupKey: fmt.Sprintf("purchase:%s", t.ID),
		Payload: map[string]any{
			"transaction_id": t.ID,
			"category":       categoryName,
			"amount_cents":   t.Amount,
		},
	})
	if err != nil {
		return err
	}

	return d.touchMerchant(ctx, t)
}

// CheckThresholds runs after a full sync. For each budgeted category, computes
// current-month spend %, then for each ladder threshold {50,75,90,100} fires a
// notification once per (category, month, threshold). fin_threshold_state
// remembers the highest-fired pct so the engine doesn't backfire on dips.
func (d *Detector) CheckThresholds(ctx context.Context) error {
	month := time.Now().UTC().Format("2006-01")
	monthLike := month + "-%"

	// Per-category: spent_abs_cents, monthly_cents, threshold_pcts, last_fired
	rows, err := d.DB.QueryContext(ctx, `
		SELECT b.category,
		       COALESCE((SELECT -SUM(t.amount_cents) FROM fin_transactions t
		                 JOIN fin_accounts a ON a.actual_id = t.account_id
		                 WHERE COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) = b.category
		                   AND t.date LIKE ?
		                   AND t.amount_cents < 0
		                   AND a.on_budget = 1
		                   AND t.transfer_id IS NULL
		                   AND t.is_parent = 0), 0) AS spent,
		       b.monthly_cents,
		       b.threshold_pcts,
		       b.push_enabled,
		       COALESCE((SELECT last_fired_pct FROM fin_threshold_state
		                 WHERE category = b.category AND month = ?), 0)
		  FROM fin_budget_targets b
	`, monthLike, month)
	if err != nil {
		return err
	}
	defer rows.Close()

	type catRow struct {
		Category      string
		Spent         int64
		Budget        int64
		ThresholdsRaw string
		PushEnabled   int
		LastFired     int
	}
	var cats []catRow
	for rows.Next() {
		var c catRow
		if err := rows.Scan(&c.Category, &c.Spent, &c.Budget, &c.ThresholdsRaw, &c.PushEnabled, &c.LastFired); err != nil {
			return err
		}
		cats = append(cats, c)
	}

	for _, c := range cats {
		if c.Budget <= 0 || c.PushEnabled == 0 {
			continue
		}
		ladder := parseThresholds(c.ThresholdsRaw) // sorted ascending
		pct := int((c.Spent * 100) / c.Budget)

		// Find the highest unfired threshold we've now crossed.
		var target int
		for _, t := range ladder {
			if pct >= t && t > c.LastFired {
				target = t // keep going to find max
			}
		}
		if target == 0 {
			continue
		}

		priority := notify.PriNormal
		if target >= 90 {
			priority = notify.PriHigh
		}
		if target >= 100 {
			priority = notify.PriCritical
		}

		_, _, err := d.Notifier.Fire(ctx, notify.Event{
			Category: notify.CatBudgetThreshold,
			Priority: priority,
			Title:    fmt.Sprintf("%s at %d%%  ($%s / $%s)", c.Category, pct, dollars(c.Spent), dollars(c.Budget)),
			Body:     thresholdBody(target, c.Spent, c.Budget),
			DedupKey: fmt.Sprintf("budget_threshold:%s:%s:%d", c.Category, month, target),
			Payload: map[string]any{
				"category":     c.Category,
				"month":        month,
				"threshold":    target,
				"spent_cents":  c.Spent,
				"budget_cents": c.Budget,
				"pct":          pct,
			},
		})
		if err != nil {
			return err
		}
		// Update fin_threshold_state regardless of skip reason — we know the
		// threshold was crossed; the notification dedup will prevent double-fire.
		_, err = d.DB.ExecContext(ctx, `
			INSERT INTO fin_threshold_state (category, month, last_fired_pct, fired_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(category, month) DO UPDATE SET
			  last_fired_pct = excluded.last_fired_pct,
			  fired_at = excluded.fired_at`,
			c.Category, month, target, time.Now().Unix())
		if err != nil {
			return err
		}
	}
	return nil
}

// ResetThresholdsForMonth — call at the start of a new month. Idempotent.
func (d *Detector) ResetThresholdsForMonth(ctx context.Context, month string) error {
	_, err := d.DB.ExecContext(ctx,
		`DELETE FROM fin_threshold_state WHERE month != ?`, month)
	return err
}

// ─── Merchant stats ──────────────────────────────────────────────────────

type merchantStat struct {
	Payee          string
	TxnCount       int
	MedianAbsCents int64
	MaxAbsCents    int64
}

func (d *Detector) merchantStats(ctx context.Context, payee string) (*merchantStat, error) {
	if payee == "" {
		return nil, nil
	}
	var m merchantStat
	err := d.DB.QueryRowContext(ctx,
		`SELECT payee, txn_count, median_abs_cents, max_abs_cents
		   FROM fin_merchant_stats WHERE payee = ?`, payee).
		Scan(&m.Payee, &m.TxnCount, &m.MedianAbsCents, &m.MaxAbsCents)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// touchMerchant increments stats. We approximate the median by recomputing
// against the actual transactions table since this is bounded (a few hundred
// txns per payee even for a busy user).
func (d *Detector) touchMerchant(ctx context.Context, t SidecarTransaction) error {
	if t.Payee == "" {
		return nil
	}
	// Pull all absolute amounts for this payee
	rows, err := d.DB.QueryContext(ctx,
		`SELECT amount_cents FROM fin_transactions WHERE payee = ? AND amount_cents < 0`,
		t.Payee)
	if err != nil {
		return err
	}
	defer rows.Close()

	var amts []int64
	var total int64
	var max int64
	for rows.Next() {
		var a int64
		if err := rows.Scan(&a); err != nil {
			return err
		}
		abs := -a
		amts = append(amts, abs)
		total += abs
		if abs > max {
			max = abs
		}
	}
	if len(amts) == 0 {
		return nil
	}
	sort.Slice(amts, func(i, j int) bool { return amts[i] < amts[j] })
	median := amts[len(amts)/2]
	now := time.Now().Unix()

	_, err = d.DB.ExecContext(ctx, `
		INSERT INTO fin_merchant_stats (payee, txn_count, total_abs_cents, median_abs_cents, max_abs_cents, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(payee) DO UPDATE SET
		  txn_count = excluded.txn_count,
		  total_abs_cents = excluded.total_abs_cents,
		  median_abs_cents = excluded.median_abs_cents,
		  max_abs_cents = MAX(max_abs_cents, excluded.max_abs_cents),
		  last_seen_at = excluded.last_seen_at`,
		t.Payee, len(amts), total, median, max, now, now)
	return err
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func parseThresholds(raw string) []int {
	// Very tolerant — parse comma + bracket separated ints.
	out := []int{}
	cur := 0
	have := false
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			cur = cur*10 + int(r-'0')
			have = true
		} else {
			if have {
				out = append(out, cur)
				cur = 0
				have = false
			}
		}
	}
	if have {
		out = append(out, cur)
	}
	sort.Ints(out)
	return out
}

func thresholdBody(pct int, spent, budget int64) string {
	if pct >= 100 {
		over := spent - budget
		return fmt.Sprintf("Over budget by $%s. Consider pausing this category for the rest of the month.", dollars(over))
	}
	if pct >= 90 {
		left := budget - spent
		return fmt.Sprintf("$%s left for the month — pace carefully.", dollars(left))
	}
	if pct >= 75 {
		return "Three-quarters in. Watch the pace."
	}
	return "Halfway there for the month."
}

func dollars(cents int64) string {
	if cents < 0 {
		cents = -cents
	}
	d := cents / 100
	c := cents % 100
	// Format with thousands separators
	out := fmt.Sprintf("%d.%02d", d, c)
	// Insert commas — simple loop since math.MaxInt64 has ≤19 digits.
	parts := []byte(out)
	// Find decimal point
	dot := 0
	for i, b := range parts {
		if b == '.' {
			dot = i
			break
		}
	}
	if dot <= 3 {
		return out
	}
	var b []byte
	for i := 0; i < dot; i++ {
		if i > 0 && (dot-i)%3 == 0 {
			b = append(b, ',')
		}
		b = append(b, parts[i])
	}
	b = append(b, parts[dot:]...)
	return string(b)
}

