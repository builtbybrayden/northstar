package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// LogSender just prints to stdout. The default — works without an Apple
// Developer account, so we can validate Phase 2 end-to-end before TestFlight.
type LogSender struct{}

func (LogSender) Send(_ context.Context, n PreparedNotification) error {
	payload, _ := json.Marshal(n.Payload)
	log.Printf("[NOTIFY %s/p%d] %s — %s   payload=%s tokens=%d",
		n.Category, n.Priority, n.Title, n.Body, payload, len(n.APNSTokens))
	return nil
}
func (LogSender) Mode() string { return "log" }

// APNSSender is the production sender. Inert until you set:
//   NORTHSTAR_APNS_MODE=apns
//   NORTHSTAR_APNS_KEY_PATH=/path/to/AuthKey_XXXX.p8
//   NORTHSTAR_APNS_KEY_ID=XXXX
//   NORTHSTAR_APNS_TEAM_ID=XXXX
//   NORTHSTAR_APNS_BUNDLE_ID=dev.northstar.app
//   NORTHSTAR_APNS_PRODUCTION=0  (1 for prod, 0 for sandbox/TestFlight)
//
// Implementation deferred until the user has an Apple Developer Program account.
// Pulls in `github.com/sideshow/apns2` at that point.
type APNSSender struct {
	KeyPath     string
	KeyID       string
	TeamID      string
	BundleID    string
	Production  bool
}

func (a *APNSSender) Send(_ context.Context, n PreparedNotification) error {
	return fmt.Errorf("apns sender not yet wired — set up Apple Dev creds + uncomment apns2 in go.mod")
}
func (*APNSSender) Mode() string { return "apns" }

// FromEnv selects a sender based on NORTHSTAR_APNS_MODE.
func FromEnv() Sender {
	mode := os.Getenv("NORTHSTAR_APNS_MODE")
	switch mode {
	case "apns":
		return &APNSSender{
			KeyPath:    os.Getenv("NORTHSTAR_APNS_KEY_PATH"),
			KeyID:      os.Getenv("NORTHSTAR_APNS_KEY_ID"),
			TeamID:     os.Getenv("NORTHSTAR_APNS_TEAM_ID"),
			BundleID:   os.Getenv("NORTHSTAR_APNS_BUNDLE_ID"),
			Production: os.Getenv("NORTHSTAR_APNS_PRODUCTION") == "1",
		}
	default:
		return LogSender{}
	}
}
