package notify

// SkipReason explains why Fire didn't send. Useful for tests and observability.
type SkipReason int

const (
	SkipNone SkipReason = iota
	SkipDedup
	SkipDisabled
	SkipQuietHours
	SkipDailyCap
	SkipUnknownCategory
)

func (s SkipReason) String() string {
	switch s {
	case SkipNone:
		return ""
	case SkipDedup:
		return "dedup"
	case SkipDisabled:
		return "rule_disabled"
	case SkipQuietHours:
		return "quiet_hours"
	case SkipDailyCap:
		return "daily_cap"
	case SkipUnknownCategory:
		return "unknown_category"
	default:
		return "unknown"
	}
}
