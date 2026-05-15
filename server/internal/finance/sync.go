package finance

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// Syncer periodically pulls from the Actual sidecar and mirrors into SQLite.
// Diff-based — only upserts on changes. Runs forever until ctx is cancelled.
type Syncer struct {
	DB              *sql.DB
	Sidecar         *SidecarClient
	Detector        *Detector // optional; nil disables notifications (Phase 1-only mode)
	ForecastWarning *ForecastWarning // optional; nil disables forecast warnings
	Interval        time.Duration
}

func NewSyncer(db *sql.DB, sc *SidecarClient, detector *Detector, interval time.Duration) *Syncer {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return &Syncer{DB: db, Sidecar: sc, Detector: detector, Interval: interval}
}

// Run loops. Returns when ctx is done.
func (s *Syncer) Run(ctx context.Context) {
	log.Printf("finance.sync: starting (interval=%s, detector=%v)", s.Interval, s.Detector != nil)
	if err := s.SyncOnce(ctx); err != nil {
		log.Printf("finance.sync: initial sync failed: %v", err)
	}
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("finance.sync: stopping")
			return
		case <-t.C:
			if err := s.SyncOnce(ctx); err != nil {
				log.Printf("finance.sync: tick failed: %v", err)
			}
		}
	}
}

// SyncOnce pulls all current data from the sidecar and upserts to SQLite.
// New-since-last-sync transactions trigger purchase + anomaly detection.
// After sync completes, the threshold checker runs.
//
// On the very first run (fin_transactions empty), suppress per-txn purchase +
// anomaly notifications so users aren't buried by historical backfill. The
// threshold checker still fires because that surfaces current-state
// over-budget categories, which is a one-time signal users want.
func (s *Syncer) SyncOnce(ctx context.Context) error {
	now := time.Now().Unix()

	initialBackfill := false
	if s.Detector != nil {
		var any int
		_ = s.DB.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM fin_transactions LIMIT 1)`).Scan(&any)
		if any == 0 {
			initialBackfill = true
			s.Detector.SetQuietPurchases(true)
			log.Printf("finance.sync: initial backfill detected — suppressing per-txn notifications for this pass")
			defer s.Detector.SetQuietPurchases(false)
		}
	}

	// 1. Accounts — straight upsert
	accs, err := s.Sidecar.Accounts(ctx)
	if err != nil {
		return err
	}
	today := time.Now().UTC().Format("2006-01-02")
	for _, a := range accs {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO fin_accounts (actual_id, name, type, balance_cents, on_budget, closed, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(actual_id) DO UPDATE SET
			   name = excluded.name,
			   balance_cents = excluded.balance_cents,
			   on_budget = excluded.on_budget,
			   closed = excluded.closed,
			   updated_at = excluded.updated_at`,
			a.ID, a.Name, "", a.Balance, boolToInt(!a.OffBudget), boolToInt(a.Closed), now)
		if err != nil {
			return err
		}
		// Snapshot today's balance for non-closed accounts. INSERT OR REPLACE
		// keyed on (account, date) — re-running the same day overwrites with
		// the latest balance, last sync of the day wins.
		if !a.Closed {
			_, err = s.DB.ExecContext(ctx,
				`INSERT OR REPLACE INTO fin_account_balance_history
				   (account_id, date, balance_cents, on_budget)
				 VALUES (?, ?, ?, ?)`,
				a.ID, today, a.Balance, boolToInt(!a.OffBudget))
			if err != nil {
				return err
			}
		}
	}

	// 2. Categories → name lookup
	cats, err := s.Sidecar.Categories(ctx)
	if err != nil {
		return err
	}
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}

	// 3. Transactions — diff against DB so we can fire purchase notifications
	//    on net-new rows only.
	txns, err := s.Sidecar.Transactions(ctx, "")
	if err != nil {
		return err
	}

	var newCount, updCount int
	for _, t := range txns {
		isNew, err := s.upsertTxn(ctx, t, catName[t.Category], now)
		if err != nil {
			return err
		}
		if isNew {
			newCount++
			if s.Detector != nil {
				if err := s.Detector.OnNewTransaction(ctx, t, catName[t.Category]); err != nil {
					log.Printf("finance.sync: detector failed for %s: %v", t.ID, err)
				}
			}
		} else {
			updCount++
		}
	}

	// 4. Seed budget targets from Actual on first run (don't overwrite user edits)
	month := time.Now().UTC().Format("2006-01")
	budgets, err := s.Sidecar.Budgets(ctx, month)
	if err != nil {
		log.Printf("finance.sync: budgets fetch failed (non-fatal): %v", err)
	} else {
		for _, b := range budgets.Categories {
			name := catName[b.ID]
			if name == "" {
				continue
			}
			_, err := s.DB.ExecContext(ctx,
				`INSERT OR IGNORE INTO fin_budget_targets
				   (category, monthly_cents, rationale, category_group, updated_at)
				 VALUES (?, ?, 'seeded from Actual', ?, ?)`,
				name, b.Budgeted, DefaultGroupFor(name), now)
			if err != nil {
				return err
			}
		}
	}

	// Backfill category_group for target rows that pre-date migration 00006
	// (or were inserted by the user with no group). Cheap to run every sync
	// since the WHERE clause is selective once everything's classified.
	rows, err := s.DB.QueryContext(ctx,
		`SELECT category FROM fin_budget_targets WHERE category_group IS NULL OR category_group = ''`)
	if err == nil {
		var cats []string
		for rows.Next() {
			var c string
			if rows.Scan(&c) == nil {
				cats = append(cats, c)
			}
		}
		rows.Close()
		for _, c := range cats {
			_, _ = s.DB.ExecContext(ctx,
				`UPDATE fin_budget_targets SET category_group = ?
				   WHERE category = ? AND (category_group IS NULL OR category_group = '')`,
				DefaultGroupFor(c), c)
		}
	}

	// 5. Threshold scan — fires budget_threshold notifications as needed
	if s.Detector != nil {
		if err := s.Detector.CheckThresholds(ctx); err != nil {
			log.Printf("finance.sync: threshold check failed: %v", err)
		}
	}

	// 6. Forecast warning — fires forecast_warning if projection dips
	// below the configured cash floor over the horizon. Optional.
	if s.ForecastWarning != nil {
		if err := s.ForecastWarning.Check(ctx, time.Now().UTC()); err != nil {
			log.Printf("finance.sync: forecast warning check failed: %v", err)
		}
	}

	if initialBackfill {
		log.Printf("finance.sync: synced %d accounts (%d new txns / %d updated) [initial backfill — purchase/anomaly suppressed]",
			len(accs), newCount, updCount)
	} else {
		log.Printf("finance.sync: synced %d accounts (%d new txns / %d updated)", len(accs), newCount, updCount)
	}
	return nil
}

// upsertTxn inserts the transaction. Returns (isNew, err). "New" means the
// actual_id did not exist before this call. We do this in two steps so we
// can return the boolean cleanly — `INSERT ... ON CONFLICT DO NOTHING` then
// check RowsAffected, then a separate UPDATE if it already existed.
//
// The UPDATE includes transfer_id + is_parent so historical rows that
// pre-date migration 00009 get backfilled on the next sync pass.
func (s *Syncer) upsertTxn(ctx context.Context, t SidecarTransaction, categoryName string, now int64) (bool, error) {
	transferID := sqlNullString(t.TransferID)
	isParent := boolToInt(t.IsParent)
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO fin_transactions (actual_id, account_id, date, payee, category, amount_cents, notes, imported_at, transfer_id, is_parent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(actual_id) DO NOTHING`,
		t.ID, t.Account, t.Date, t.Payee, categoryName, t.Amount, t.Notes, now, transferID, isParent)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 1 {
		return true, nil
	}
	_, err = s.DB.ExecContext(ctx,
		`UPDATE fin_transactions SET
		   account_id = ?, date = ?, payee = ?, category = ?, amount_cents = ?, notes = ?,
		   transfer_id = ?, is_parent = ?
		 WHERE actual_id = ?`,
		t.Account, t.Date, t.Payee, categoryName, t.Amount, t.Notes, transferID, isParent, t.ID)
	return false, err
}

// sqlNullString returns nil for empty strings (so the column stays NULL,
// which the index + filters expect) and the string itself otherwise.
func sqlNullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

