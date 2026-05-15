package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/builtbybrayden/northstar/server/internal/api"
	"github.com/builtbybrayden/northstar/server/internal/config"
	"github.com/builtbybrayden/northstar/server/internal/db"
	"github.com/builtbybrayden/northstar/server/internal/finance"
	"github.com/builtbybrayden/northstar/server/internal/goals"
	"github.com/builtbybrayden/northstar/server/internal/health"
	"github.com/builtbybrayden/northstar/server/internal/notify"
	"github.com/builtbybrayden/northstar/server/internal/scheduler"
)

func main() {
	cfg := config.Load()

	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer d.Close()

	if err := db.Migrate(d); err != nil {
		log.Fatalf("db migrate: %v", err)
	}
	log.Printf("northstar-server starting · db=%s · listen=%s", cfg.DBPath, cfg.ListenAddr)

	hub := notify.NewHub()
	srv := api.NewServer(cfg, d).WithNotifyHub(hub)
	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sender := notify.FromEnv()
	composer := notify.NewComposer(d, sender).WithHub(hub)
	log.Printf("notify: sender=%s", sender.Mode())

	if cfg.Finance.Enabled {
		sc := finance.NewSidecarClient(cfg.Finance.SidecarURL, cfg.Finance.SidecarSecret)

		// When mode=actual, forward credentials to the sidecar's /init.
		// Mock mode skips this — the sidecar's mock provider has nothing to
		// initialize.
		if strings.EqualFold(cfg.Finance.Mode, "actual") {
			if cfg.Finance.ActualServerURL == "" || cfg.Finance.ActualPassword == "" || cfg.Finance.ActualSyncID == "" {
				log.Printf("finance: mode=actual but NORTHSTAR_ACTUAL_{SERVER_URL,PASSWORD,SYNC_ID} missing — sidecar will error until credentials are provided")
			} else {
				initCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
				err := sc.Init(initCtx, finance.InitParams{
					ServerURL:          cfg.Finance.ActualServerURL,
					Password:           cfg.Finance.ActualPassword,
					SyncID:             cfg.Finance.ActualSyncID,
					EncryptionPassword: cfg.Finance.ActualEncryption,
				})
				cancel()
				if err != nil {
					log.Printf("finance: sidecar init failed: %v (sync will keep retrying after the first error surfaces)", err)
				} else {
					log.Printf("finance: sidecar initialized against Actual server %s", cfg.Finance.ActualServerURL)
				}
			}
		}

		detector := finance.NewDetector(d, composer)
		syncer := finance.NewSyncer(d, sc, detector, cfg.Finance.SyncInterval)
		syncer.ForecastWarning = finance.NewForecastWarning(
			detector,
			cfg.Finance.ForecastCashFloorCents,
			cfg.Finance.ForecastHorizonDays)
		go syncer.Run(ctx)
	} else {
		log.Printf("finance sync disabled")
	}

	var supplementReminders *health.SupplementReminders
	if cfg.Pillars.Health {
		supplementReminders = health.NewSupplementReminders(d, composer)
	}

	if cfg.Pillars.Goals {
		gh := goals.NewHandlers(d)
		sch := scheduler.New(d, composer, gh)
		if supplementReminders != nil {
			sch.WithSupplements(supplementReminders)
		}
		go sch.Run(ctx)
	} else if supplementReminders != nil {
		// Goals disabled but Health enabled — run a bare scheduler just for supplements.
		sch := scheduler.New(d, composer, nil).WithSupplements(supplementReminders)
		go sch.Run(ctx)
	}

	if cfg.Health.Enabled {
		hsc := health.NewSidecarClient(cfg.Health.SidecarURL, cfg.Health.SidecarSecret)
		hdet := health.NewDetector(d, composer)
		hsy := health.NewSyncer(d, hsc, hdet, cfg.Health.SyncInterval)
		go hsy.Run(ctx)
	} else {
		log.Printf("health sync disabled")
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
