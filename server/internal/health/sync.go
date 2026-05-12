package health

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// Syncer pulls from the WHOOP sidecar and mirrors into SQLite. After sync,
// the detector runs to catch big recovery drops.
type Syncer struct {
	DB       *sql.DB
	Sidecar  *SidecarClient
	Detector *Detector
	Interval time.Duration
}

func NewSyncer(db *sql.DB, sc *SidecarClient, detector *Detector, interval time.Duration) *Syncer {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return &Syncer{DB: db, Sidecar: sc, Detector: detector, Interval: interval}
}

func (s *Syncer) Run(ctx context.Context) {
	log.Printf("health.sync: starting (interval=%s, detector=%v)", s.Interval, s.Detector != nil)
	if err := s.SyncOnce(ctx); err != nil {
		log.Printf("health.sync: initial sync failed: %v", err)
	}
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("health.sync: stopping")
			return
		case <-t.C:
			if err := s.SyncOnce(ctx); err != nil {
				log.Printf("health.sync: tick failed: %v", err)
			}
		}
	}
}

func (s *Syncer) SyncOnce(ctx context.Context) error {
	// 1. Recovery
	recs, err := s.Sidecar.Recovery(ctx, 14)
	if err != nil {
		return err
	}
	for _, r := range recs {
		score := nullInt(r.Score)
		rhr := nullInt(r.RHR)
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO health_recovery (date, score, hrv_ms, rhr, source)
			 VALUES (?, ?, ?, ?, 'whoop')
			 ON CONFLICT(date) DO UPDATE SET
			   score = excluded.score, hrv_ms = excluded.hrv_ms,
			   rhr = excluded.rhr, source = excluded.source`,
			r.Date, score, r.HRVms, rhr)
		if err != nil {
			return err
		}
	}

	// 2. Sleep
	sleeps, err := s.Sidecar.Sleep(ctx, 14)
	if err != nil {
		return err
	}
	for _, sl := range sleeps {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO health_sleep (date, duration_min, score, debt_min, source)
			 VALUES (?, ?, ?, ?, 'whoop')
			 ON CONFLICT(date) DO UPDATE SET
			   duration_min = excluded.duration_min, score = excluded.score,
			   debt_min = excluded.debt_min, source = excluded.source`,
			sl.Date, nullInt(sl.DurationMin), nullInt(sl.Score), nullInt(sl.DebtMin))
		if err != nil {
			return err
		}
	}

	// 3. Strain
	strains, err := s.Sidecar.Strain(ctx, 14)
	if err != nil {
		return err
	}
	for _, st := range strains {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO health_strain (date, score, avg_hr, max_hr, source)
			 VALUES (?, ?, ?, ?, 'whoop')
			 ON CONFLICT(date) DO UPDATE SET
			   score = excluded.score, avg_hr = excluded.avg_hr,
			   max_hr = excluded.max_hr, source = excluded.source`,
			st.Date, st.Score, nullInt(st.AvgHR), nullInt(st.MaxHR))
		if err != nil {
			return err
		}
	}

	// 4. Detectors — surface biometric red flags
	if s.Detector != nil {
		if err := s.Detector.CheckRecoveryDrop(ctx); err != nil {
			log.Printf("health.sync: recovery-drop detector failed: %v", err)
		}
		if err := s.Detector.CheckSleepIssues(ctx); err != nil {
			log.Printf("health.sync: sleep detector failed: %v", err)
		}
		if err := s.Detector.CheckOverreach(ctx); err != nil {
			log.Printf("health.sync: overreach detector failed: %v", err)
		}
	}

	log.Printf("health.sync: synced %d recovery, %d sleep, %d strain rows",
		len(recs), len(sleeps), len(strains))
	return nil
}

func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
