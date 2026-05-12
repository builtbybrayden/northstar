package health

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/brayden/northstar-server/internal/notify"
)

// Detector fires health_insight notifications for biometric red flags.
type Detector struct {
	DB       *sql.DB
	Notifier *notify.Composer
	// DropThreshold: minimum recovery-point drop from prior day to fire.
	DropThreshold int
	// SleepDebtMinThreshold: fires when sleep debt exceeds this (minutes).
	SleepDebtMinThreshold int
	// SleepScoreThreshold: fires when sleep score is below this.
	SleepScoreThreshold int
	// StrainOverreachThreshold + RecoveryOverreachThreshold: an "overreach"
	// warning fires when today's strain is high AND today's recovery is low.
	StrainOverreachThreshold   float64
	RecoveryOverreachThreshold int
}

func NewDetector(db *sql.DB, n *notify.Composer) *Detector {
	return &Detector{
		DB:                         db,
		Notifier:                   n,
		DropThreshold:              30,
		SleepDebtMinThreshold:      90,
		SleepScoreThreshold:        50,
		StrainOverreachThreshold:   18.0,
		RecoveryOverreachThreshold: 50,
	}
}

// CheckRecoveryDrop compares today's recovery against yesterday's. Fires
// health_insight if today is below threshold below yesterday.
func (d *Detector) CheckRecoveryDrop(ctx context.Context) error {
	rows, err := d.DB.QueryContext(ctx,
		`SELECT date, score FROM health_recovery
		   WHERE score IS NOT NULL
		   ORDER BY date DESC LIMIT 2`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type pt struct {
		date  string
		score int
	}
	var rec []pt
	for rows.Next() {
		var p pt
		if err := rows.Scan(&p.date, &p.score); err != nil {
			return err
		}
		rec = append(rec, p)
	}
	if len(rec) < 2 {
		return nil
	}
	today, yesterday := rec[0], rec[1]
	drop := yesterday.score - today.score
	if drop < d.DropThreshold {
		return nil
	}

	body := fmt.Sprintf("Recovery dropped from %d%% to %d%% overnight. Consider an easy day.", yesterday.score, today.score)
	_, _, err = d.Notifier.Fire(ctx, notify.Event{
		Category: notify.CatHealthInsight,
		Priority: notify.PriHigh,
		Title:    fmt.Sprintf("Recovery dropped %d points", drop),
		Body:     body,
		DedupKey: fmt.Sprintf("health_insight:recovery_drop:%s", today.date),
		Payload: map[string]any{
			"today_date":      today.date,
			"today_score":     today.score,
			"yesterday_score": yesterday.score,
			"drop":            drop,
		},
	})
	return err
}

// CheckSleepIssues fires up to one health_insight per day when sleep debt is
// high or sleep score is low. Dedup keys include date + reason so a debt-fire
// and score-fire on the same day don't collide.
func (d *Detector) CheckSleepIssues(ctx context.Context) error {
	var date string
	var debt, score sql.NullInt64
	err := d.DB.QueryRowContext(ctx,
		`SELECT date, debt_min, score FROM health_sleep
		   WHERE date IS NOT NULL
		   ORDER BY date DESC LIMIT 1`).Scan(&date, &debt, &score)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	if debt.Valid && int(debt.Int64) >= d.SleepDebtMinThreshold {
		mins := int(debt.Int64)
		_, _, err := d.Notifier.Fire(ctx, notify.Event{
			Category: notify.CatHealthInsight,
			Priority: notify.PriNormal,
			Title:    fmt.Sprintf("Sleep debt %d min", mins),
			Body:     "Carrying real debt — pull an early night if you can.",
			DedupKey: fmt.Sprintf("health_insight:sleep_debt:%s", date),
			Payload: map[string]any{
				"date":     date,
				"debt_min": mins,
				"reason":   "sleep_debt",
			},
		})
		if err != nil {
			return err
		}
	}

	if score.Valid && int(score.Int64) > 0 && int(score.Int64) < d.SleepScoreThreshold {
		s := int(score.Int64)
		_, _, err := d.Notifier.Fire(ctx, notify.Event{
			Category: notify.CatHealthInsight,
			Priority: notify.PriNormal,
			Title:    fmt.Sprintf("Rough night — sleep score %d", s),
			Body:     "Recovery will lag. Match training load to readiness.",
			DedupKey: fmt.Sprintf("health_insight:sleep_score:%s", date),
			Payload: map[string]any{
				"date":   date,
				"score":  s,
				"reason": "sleep_score",
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// CheckOverreach fires when today's strain is high relative to today's recovery —
// the classic "you're flooring it with the parking brake on" pattern.
func (d *Detector) CheckOverreach(ctx context.Context) error {
	var date string
	var strain sql.NullFloat64
	var recovery sql.NullInt64
	err := d.DB.QueryRowContext(ctx,
		`SELECT t.date, t.score, r.score
		   FROM health_strain t
		   LEFT JOIN health_recovery r ON r.date = t.date
		   WHERE t.score IS NOT NULL
		   ORDER BY t.date DESC LIMIT 1`).Scan(&date, &strain, &recovery)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if !strain.Valid || !recovery.Valid {
		return nil
	}
	if strain.Float64 < d.StrainOverreachThreshold {
		return nil
	}
	if int(recovery.Int64) >= d.RecoveryOverreachThreshold {
		return nil
	}

	_, _, err = d.Notifier.Fire(ctx, notify.Event{
		Category: notify.CatHealthInsight,
		Priority: notify.PriHigh,
		Title:    fmt.Sprintf("Overreach: strain %.1f on %d%% recovery", strain.Float64, recovery.Int64),
		Body:     "High output on low readiness. Risk of overtraining — consider a deload.",
		DedupKey: fmt.Sprintf("health_insight:overreach:%s", date),
		Payload: map[string]any{
			"date":     date,
			"strain":   strain.Float64,
			"recovery": int(recovery.Int64),
			"reason":   "overreach",
		},
	})
	return err
}
