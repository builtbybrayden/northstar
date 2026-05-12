package goals

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// NextFiresAt parses a standard 5-field cron expression and returns the next
// fire time after `from`. Examples:
//   "0 7 * * *"     → every day at 07:00
//   "30 7 * * 1"    → every Monday at 07:30
//   "0 9 1 * *"     → 1st of month at 09:00
//   "0 21 * * 5"    → every Friday at 21:00
func NextFiresAt(recurrence string, from time.Time) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(recurrence)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron parse %q: %w", recurrence, err)
	}
	return sched.Next(from), nil
}
