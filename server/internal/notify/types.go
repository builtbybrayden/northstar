package notify

import (
	"context"
	"time"
)

// Categories — one per notif_rules row. Keep in sync with the seed migration.
const (
	CatPurchase        = "purchase"
	CatBudgetThreshold = "budget_threshold"
	CatAnomaly         = "anomaly"
	CatDailyBrief      = "daily_brief"
	CatEveningRetro    = "evening_retro"
	CatSupplement      = "supplement"
	CatHealthInsight   = "health_insight"
	CatGoalMilestone   = "goal_milestone"
	CatSubscriptionNew = "subscription_new"
	CatWeeklyRetro     = "weekly_retro"
	CatForecastWarning = "forecast_warning"
)

// Priority levels. >=8 bypasses quiet hours.
const (
	PriLow      = 3
	PriNormal   = 5
	PriHigh     = 7
	PriCritical = 9
)

// Event is the input to the engine. The composer assembles a fully-formed
// Notification from one of these.
type Event struct {
	Category    string
	Title       string
	Body        string
	DedupKey    string         // unique-per-fire; e.g. "budget_threshold:Restaurants:2026-05:90"
	Priority    int            // 1–10
	Payload     map[string]any
}

// Rule is the user-editable per-category control row.
type Rule struct {
	Category        string
	Enabled         bool
	QuietHoursStart string // "HH:MM" or empty
	QuietHoursEnd   string
	BypassQuiet     bool
	Delivery        string // push | live_activity | silent_badge
	MaxPerDay       int
}

// Sender is how a fully-composed notification gets to the user's device(s).
// Two implementations: LogSender (default, just prints) and APNSSender.
type Sender interface {
	Send(ctx context.Context, n PreparedNotification) error
	Mode() string
}

// PreparedNotification is what the engine hands the sender after rules pass.
type PreparedNotification struct {
	ID         string
	Category   string
	Title      string
	Body       string
	Priority   int
	Payload    map[string]any
	CreatedAt  time.Time
	APNSTokens []string // empty in log mode; populated when sender is APNS
}
