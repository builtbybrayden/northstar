package goals

import "encoding/json"

// MilestoneStatus values — keep in sync with frontend.
const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
	StatusArchived   = "archived"
)

// Milestone is a flagship-able goal with a due date.
type Milestone struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	DescriptionMD string `json:"description_md"`
	DueDate       string `json:"due_date"`     // YYYY-MM-DD
	Status        string `json:"status"`        // pending/in_progress/done/archived
	Flagship      bool   `json:"flagship"`
	DisplayOrder  int    `json:"display_order"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

// MilestoneInput is what the API accepts on POST/PATCH (all optional on PATCH).
type MilestoneInput struct {
	Title         *string `json:"title,omitempty"`
	DescriptionMD *string `json:"description_md,omitempty"`
	DueDate       *string `json:"due_date,omitempty"`
	Status        *string `json:"status,omitempty"`
	Flagship      *bool   `json:"flagship,omitempty"`
	DisplayOrder  *int    `json:"display_order,omitempty"`
}

// DailyItem is one entry in goal_daily_log.items_json.
type DailyItem struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	Done        bool   `json:"done"`
	Source      string `json:"source"`           // manual / reminder / milestone / rollover
	SourceRef   string `json:"source_ref,omitempty"` // reminder.id or milestone.id when applicable
}

// DailyLog represents one day's checklist + reflection.
type DailyLog struct {
	Date         string       `json:"date"`
	Items        []DailyItem  `json:"items"`
	ReflectionMD string       `json:"reflection_md"`
	StreakCount  int          `json:"streak_count"`
	UpdatedAt    int64        `json:"updated_at"`
}

// DailyLogInput is what the API accepts on PUT — items rewritten in one shot.
type DailyLogInput struct {
	Items        []DailyItem `json:"items"`
	ReflectionMD *string     `json:"reflection_md,omitempty"`
}

// WeeklyTracker covers a calendar week (Monday-anchored).
type WeeklyTracker struct {
	WeekOf       string          `json:"week_of"`
	Theme        string          `json:"theme"`
	WeeklyGoals  []DailyItem     `json:"weekly_goals"`
	RetroMD      string          `json:"retro_md"`
	UpdatedAt    int64           `json:"updated_at"`
}

type WeeklyTrackerInput struct {
	Theme       *string      `json:"theme,omitempty"`
	WeeklyGoals *[]DailyItem `json:"weekly_goals,omitempty"`
	RetroMD     *string      `json:"retro_md,omitempty"`
}

// MonthlyGoals represents a calendar month.
type MonthlyGoals struct {
	Month        string       `json:"month"`         // YYYY-MM
	MonthlyGoals []DailyItem  `json:"monthly_goals"`
	RetroMD      string       `json:"retro_md"`
	UpdatedAt    int64        `json:"updated_at"`
}

type MonthlyGoalsInput struct {
	MonthlyGoals *[]DailyItem `json:"monthly_goals,omitempty"`
	RetroMD      *string      `json:"retro_md,omitempty"`
}

// OutputLogEntry — CVE, blog post, talk, tool shipped, cert earned, PR, report.
type OutputLogEntry struct {
	ID        string `json:"id"`
	Date      string `json:"date"`
	Category  string `json:"category"`   // cve/blog/talk/tool/cert/pr/report
	Title     string `json:"title"`
	BodyMD    string `json:"body_md"`
	URL       string `json:"url"`
	CreatedAt int64  `json:"created_at"`
}

type OutputLogInput struct {
	Date     *string `json:"date,omitempty"`
	Category *string `json:"category,omitempty"`
	Title    *string `json:"title,omitempty"`
	BodyMD   *string `json:"body_md,omitempty"`
	URL      *string `json:"url,omitempty"`
}

// NetworkingLogEntry — who you talked to, why, what's next.
type NetworkingLogEntry struct {
	ID            string `json:"id"`
	Date          string `json:"date"`
	Person        string `json:"person"`
	Context       string `json:"context"`
	NextAction    string `json:"next_action"`
	NextActionDue string `json:"next_action_due"`
	CreatedAt     int64  `json:"created_at"`
}

type NetworkingLogInput struct {
	Date          *string `json:"date,omitempty"`
	Person        *string `json:"person,omitempty"`
	Context       *string `json:"context,omitempty"`
	NextAction    *string `json:"next_action,omitempty"`
	NextActionDue *string `json:"next_action_due,omitempty"`
}

// Reminder fires on a cron-like recurrence.
type Reminder struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Recurrence  string `json:"recurrence"`    // standard 5-field cron string
	NextFiresAt int64  `json:"next_fires_at"`
	Active      bool   `json:"active"`
	CreatedAt   int64  `json:"created_at"`
}

type ReminderInput struct {
	Title      *string `json:"title,omitempty"`
	Body       *string `json:"body,omitempty"`
	Recurrence *string `json:"recurrence,omitempty"`
	Active     *bool   `json:"active,omitempty"`
}

// Brief is the assembled daily/evening notification content.
type Brief struct {
	Date              string      `json:"date"`
	Items             []DailyItem `json:"items"`
	StreakCount       int         `json:"streak_count"`
	MilestonesDueSoon []Milestone `json:"milestones_due_soon"`
	ActiveReminders   []Reminder  `json:"active_reminders"`
}

// helpers

func encodeItems(items []DailyItem) (string, error) {
	if items == nil {
		items = []DailyItem{}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeItems(s string) []DailyItem {
	if s == "" {
		return []DailyItem{}
	}
	var out []DailyItem
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []DailyItem{}
	}
	return out
}
