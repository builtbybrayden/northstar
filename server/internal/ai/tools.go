package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ToolDispatcher executes a named tool with a JSON argument blob and returns
// a JSON-serializable result. Tools are pillar-scoped read-only queries.
type ToolDispatcher struct {
	DB  *sql.DB
	Now func() time.Time
}

func NewToolDispatcher(db *sql.DB) *ToolDispatcher {
	return &ToolDispatcher{DB: db, Now: time.Now}
}

// Defs returns the Anthropic-format tool definitions. The last entry has
// cache_control set so the whole tool block is cached on Anthropic's side.
func Defs() []toolDef {
	defs := []toolDef{
		{
			Name:        "finance_summary",
			Description: "Current-month spend overview. Returns net worth, on/off-budget split, income, spent, saved, and per-category spend with budget + percentage. Use this for general 'how am I doing financially this month' questions.",
			InputSchema: schemaObj(map[string]any{
				"month": schemaStringDesc("YYYY-MM format. Defaults to current month if omitted."),
			}, nil),
		},
		{
			Name:        "finance_search_transactions",
			Description: "Search transactions by payee substring, category, or date range. Returns up to 50 matches. Use this when the user asks about specific spend like 'how much have I spent at Starbucks' or 'show me my dining out transactions'.",
			InputSchema: schemaObj(map[string]any{
				"payee":      schemaStringDesc("Case-insensitive substring of payee name."),
				"category":   schemaStringDesc("Exact category name (e.g. 'Restaurants')."),
				"since":      schemaStringDesc("YYYY-MM-DD inclusive lower bound."),
				"until":      schemaStringDesc("YYYY-MM-DD inclusive upper bound."),
			}, nil),
		},
		{
			Name:        "finance_category_history",
			Description: "Twelve months of spend for one category, total per month. Use this for 'is my dining out trending up' or 'how have my grocery costs changed'.",
			InputSchema: schemaObj(map[string]any{
				"category": schemaStringDesc("Category name (e.g. 'Restaurants')."),
			}, []string{"category"}),
		},
		{
			Name:        "finance_subscriptions",
			Description: "List likely recurring charges — same payee × similar amount appearing ≥2 months in a row. Use this for 'what subscriptions am I paying for'.",
			InputSchema: schemaObj(nil, nil),
		},
		{
			Name:        "goals_brief",
			Description: "Today's brief: open daily items, active reminders, milestones due in next 7 days, and current streak. Use this for 'what's on my plate today' or 'am I on track'.",
			InputSchema: schemaObj(nil, nil),
		},
		{
			Name:        "goals_milestones",
			Description: "List all goal milestones with status, due date, and flagship flag. Use this for 'what are my long-term goals' or 'am I on track for OSCP'.",
			InputSchema: schemaObj(map[string]any{
				"status_in": schemaStringArrayDesc("Filter to these statuses. Allowed: pending, in_progress, done, archived."),
			}, nil),
		},
		{
			Name:        "goals_recent_output",
			Description: "Recent entries in the output log (CVEs, blog posts, talks, tools shipped, certs earned). Use this for 'what have I shipped recently' or 'am I generating output'.",
			InputSchema: schemaObj(map[string]any{
				"days": schemaIntDesc("How many days back. Default 90."),
			}, nil),
		},
		{
			Name:        "health_today",
			Description: "Today's recovery score, HRV, RHR, sleep duration/score, strain, and a computed verdict (push/maintain/recover). Use this for 'how am I feeling today' or 'should I work out hard'.",
			InputSchema: schemaObj(nil, nil),
		},
		{
			Name:        "health_recovery_history",
			Description: "Last N days of recovery score + HRV + RHR. Use this for 'how has my recovery been this week' or correlation questions like 'did my sleep get worse'.",
			InputSchema: schemaObj(map[string]any{
				"days": schemaIntDesc("How many days back. Default 14, max 90."),
			}, nil),
		},
		{
			Name:        "health_supplements",
			Description: "Lists active supplement / peptide / medication definitions and recent doses. Use this to correlate stack with recovery, answer 'did I take my BPC-157 this week', or ground peptide recommendations.",
			InputSchema: schemaObj(map[string]any{
				"days": schemaIntDesc("Dose history window. Default 7."),
			}, nil),
		},
	}

	// Mark the last tool's cache_control so Anthropic caches the entire tool
	// block (cache extends through the prior 3 breakpoints).
	last := len(defs) - 1
	defs[last].CacheControl = &cacheControl{Type: "ephemeral"}
	return defs
}

// Dispatch executes a tool by name and returns the JSON-encoded string result.
func (d *ToolDispatcher) Dispatch(ctx context.Context, name string, raw json.RawMessage) (string, error) {
	switch name {
	case "finance_summary":
		return d.financeSummary(ctx, raw)
	case "finance_search_transactions":
		return d.financeSearchTransactions(ctx, raw)
	case "finance_category_history":
		return d.financeCategoryHistory(ctx, raw)
	case "finance_subscriptions":
		return d.financeSubscriptions(ctx)
	case "goals_brief":
		return d.goalsBrief(ctx)
	case "goals_milestones":
		return d.goalsMilestones(ctx, raw)
	case "goals_recent_output":
		return d.goalsRecentOutput(ctx, raw)
	case "health_today":
		return d.healthToday(ctx)
	case "health_recovery_history":
		return d.healthRecoveryHistory(ctx, raw)
	case "health_supplements":
		return d.healthSupplements(ctx, raw)
	}
	return "", fmt.Errorf("unknown tool: %s", name)
}

// ─── Finance tools ────────────────────────────────────────────────────────

type financeSummaryArgs struct {
	Month string `json:"month,omitempty"`
}

func (d *ToolDispatcher) financeSummary(ctx context.Context, raw json.RawMessage) (string, error) {
	var a financeSummaryArgs
	_ = json.Unmarshal(raw, &a)
	month := a.Month
	if month == "" {
		month = d.Now().UTC().Format("2006-01")
	}
	monthLike := month + "-%"

	type cat struct {
		Category      string `json:"category"`
		SpentCents    int64  `json:"spent_cents"`
		BudgetedCents int64  `json:"budgeted_cents"`
		Pct           int    `json:"pct"`
		Over          bool   `json:"over"`
	}
	type out struct {
		Month          string `json:"month"`
		NetWorthCents  int64  `json:"net_worth_cents"`
		OnBudgetCents  int64  `json:"on_budget_cents"`
		OffBudgetCents int64  `json:"off_budget_cents"`
		IncomeCents    int64  `json:"income_cents"`
		SpentCents     int64  `json:"spent_cents"`
		Categories     []cat  `json:"categories"`
	}
	var o out
	o.Month = month

	if err := d.DB.QueryRowContext(ctx,
		`SELECT
		   COALESCE(SUM(CASE WHEN on_budget=1 THEN balance_cents END), 0),
		   COALESCE(SUM(CASE WHEN on_budget=0 THEN balance_cents END), 0)
		 FROM fin_accounts WHERE closed = 0`).
		Scan(&o.OnBudgetCents, &o.OffBudgetCents); err != nil {
		return "", err
	}
	o.NetWorthCents = o.OnBudgetCents + o.OffBudgetCents

	if err := d.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN amount_cents > 0 THEN amount_cents END), 0),
		        COALESCE(-SUM(CASE WHEN amount_cents < 0 THEN amount_cents END), 0)
		   FROM fin_transactions t JOIN fin_accounts a ON a.actual_id = t.account_id
		  WHERE t.date LIKE ? AND a.on_budget = 1
		    AND COALESCE(t.category,'') != 'Transfer'`, monthLike).
		Scan(&o.IncomeCents, &o.SpentCents); err != nil {
		return "", err
	}

	rows, err := d.DB.QueryContext(ctx,
		`SELECT b.category,
		        COALESCE(-(SELECT SUM(amount_cents) FROM fin_transactions t
		                   JOIN fin_accounts a ON a.actual_id = t.account_id
		                   WHERE t.category = b.category AND t.date LIKE ?
		                     AND t.amount_cents < 0 AND a.on_budget = 1), 0),
		        b.monthly_cents
		   FROM fin_budget_targets b ORDER BY b.category ASC`, monthLike)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var c cat
		if err := rows.Scan(&c.Category, &c.SpentCents, &c.BudgetedCents); err != nil {
			return "", err
		}
		if c.BudgetedCents > 0 {
			c.Pct = int(c.SpentCents * 100 / c.BudgetedCents)
		}
		c.Over = c.SpentCents > c.BudgetedCents
		o.Categories = append(o.Categories, c)
	}
	return jsonString(o)
}

type financeSearchArgs struct {
	Payee    string `json:"payee,omitempty"`
	Category string `json:"category,omitempty"`
	Since    string `json:"since,omitempty"`
	Until    string `json:"until,omitempty"`
}

func (d *ToolDispatcher) financeSearchTransactions(ctx context.Context, raw json.RawMessage) (string, error) {
	var a financeSearchArgs
	_ = json.Unmarshal(raw, &a)
	clauses := []string{"1=1"}
	args := []any{}
	if a.Payee != "" {
		clauses = append(clauses, "LOWER(t.payee) LIKE LOWER(?)")
		args = append(args, "%"+a.Payee+"%")
	}
	if a.Category != "" {
		clauses = append(clauses, "t.category = ?")
		args = append(args, a.Category)
	}
	if a.Since != "" {
		clauses = append(clauses, "t.date >= ?")
		args = append(args, a.Since)
	}
	if a.Until != "" {
		clauses = append(clauses, "t.date <= ?")
		args = append(args, a.Until)
	}
	q := `SELECT t.actual_id, t.date, t.payee, t.category, t.amount_cents
	        FROM fin_transactions t WHERE ` + strings.Join(clauses, " AND ") +
		` ORDER BY t.date DESC LIMIT 50`
	rows, err := d.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type tx struct {
		ID          string `json:"id"`
		Date        string `json:"date"`
		Payee       string `json:"payee"`
		Category    string `json:"category"`
		AmountCents int64  `json:"amount_cents"`
	}
	type out struct {
		Count  int  `json:"count"`
		Total  int64 `json:"total_cents"`
		Rows   []tx `json:"rows"`
	}
	var o out
	for rows.Next() {
		var x tx
		var cat sql.NullString
		if err := rows.Scan(&x.ID, &x.Date, &x.Payee, &cat, &x.AmountCents); err != nil {
			return "", err
		}
		x.Category = cat.String
		o.Rows = append(o.Rows, x)
		o.Total += x.AmountCents
	}
	o.Count = len(o.Rows)
	return jsonString(o)
}

type financeCategoryArgs struct {
	Category string `json:"category"`
}

func (d *ToolDispatcher) financeCategoryHistory(ctx context.Context, raw json.RawMessage) (string, error) {
	var a financeCategoryArgs
	if err := json.Unmarshal(raw, &a); err != nil || a.Category == "" {
		return "", fmt.Errorf("category required")
	}
	rows, err := d.DB.QueryContext(ctx,
		`SELECT substr(date, 1, 7) AS month,
		        -SUM(amount_cents) AS spent
		   FROM fin_transactions
		  WHERE category = ? AND amount_cents < 0
		    AND date >= date('now', '-12 months')
		  GROUP BY month ORDER BY month ASC`, a.Category)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type m struct {
		Month      string `json:"month"`
		SpentCents int64  `json:"spent_cents"`
	}
	type out struct {
		Category string `json:"category"`
		Months   []m    `json:"months"`
	}
	o := out{Category: a.Category}
	for rows.Next() {
		var mm m
		if err := rows.Scan(&mm.Month, &mm.SpentCents); err != nil {
			return "", err
		}
		o.Months = append(o.Months, mm)
	}
	return jsonString(o)
}

func (d *ToolDispatcher) financeSubscriptions(ctx context.Context) (string, error) {
	// Heuristic: same (payee, abs amount rounded to nearest dollar) appearing
	// ≥2 distinct months in the last 6.
	rows, err := d.DB.QueryContext(ctx, `
		SELECT payee, ROUND(ABS(amount_cents) / 100.0) AS dollars,
		       COUNT(DISTINCT substr(date, 1, 7)) AS months,
		       MAX(date) AS last_seen
		  FROM fin_transactions
		 WHERE amount_cents < 0
		   AND date >= date('now', '-6 months')
		   AND COALESCE(payee,'') != ''
		 GROUP BY payee, dollars
		HAVING months >= 2
		 ORDER BY dollars DESC LIMIT 50`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type s struct {
		Payee   string  `json:"payee"`
		Dollars float64 `json:"dollars"`
		Months  int     `json:"months_seen"`
		Last    string  `json:"last_seen"`
	}
	type out struct {
		Subs []s `json:"subscriptions"`
	}
	var o out
	for rows.Next() {
		var x s
		if err := rows.Scan(&x.Payee, &x.Dollars, &x.Months, &x.Last); err != nil {
			return "", err
		}
		o.Subs = append(o.Subs, x)
	}
	return jsonString(o)
}

// ─── Goals tools ──────────────────────────────────────────────────────────

func (d *ToolDispatcher) goalsBrief(ctx context.Context) (string, error) {
	now := d.Now()
	today := now.UTC().Format("2006-01-02")
	type item struct {
		Text   string `json:"text"`
		Done   bool   `json:"done"`
		Source string `json:"source"`
	}
	type milestone struct {
		Title    string `json:"title"`
		DueDate  string `json:"due_date"`
		Status   string `json:"status"`
		Flagship bool   `json:"flagship"`
	}
	type out struct {
		Date         string      `json:"date"`
		Streak       int         `json:"streak"`
		Items        []item      `json:"items"`
		DueIn7Days   []milestone `json:"milestones_due_in_7_days"`
		ReminderCount int        `json:"active_reminder_count"`
	}
	o := out{Date: today, Items: []item{}, DueIn7Days: []milestone{}}

	// today items + streak
	var raw string
	err := d.DB.QueryRowContext(ctx,
		`SELECT COALESCE(items_json,'[]'), streak_count FROM goal_daily_log WHERE date = ?`,
		today).Scan(&raw, &o.Streak)
	if err == nil {
		var arr []map[string]any
		if json.Unmarshal([]byte(raw), &arr) == nil {
			for _, x := range arr {
				o.Items = append(o.Items, item{
					Text:   stringFromMap(x, "text"),
					Done:   boolFromMap(x, "done"),
					Source: stringFromMap(x, "source"),
				})
			}
		}
	}

	// milestones due in 7d
	weekOut := now.AddDate(0, 0, 7).Format("2006-01-02")
	rows, err := d.DB.QueryContext(ctx,
		`SELECT title, COALESCE(due_date,''), status, flagship
		   FROM goal_milestones
		  WHERE status NOT IN ('done','archived')
		    AND due_date IS NOT NULL AND due_date != ''
		    AND due_date <= ?
		  ORDER BY due_date ASC`, weekOut)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m milestone
			var flagship int
			if err := rows.Scan(&m.Title, &m.DueDate, &m.Status, &flagship); err == nil {
				m.Flagship = flagship == 1
				o.DueIn7Days = append(o.DueIn7Days, m)
			}
		}
	}

	// active reminder count
	_ = d.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM goal_reminders WHERE active = 1`).Scan(&o.ReminderCount)

	return jsonString(o)
}

type goalsMilestoneArgs struct {
	StatusIn []string `json:"status_in,omitempty"`
}

func (d *ToolDispatcher) goalsMilestones(ctx context.Context, raw json.RawMessage) (string, error) {
	var a goalsMilestoneArgs
	_ = json.Unmarshal(raw, &a)
	q := `SELECT id, title, COALESCE(description_md,''), COALESCE(due_date,''),
	             status, flagship FROM goal_milestones`
	args := []any{}
	if len(a.StatusIn) > 0 {
		placeholders := strings.Repeat("?,", len(a.StatusIn))
		placeholders = placeholders[:len(placeholders)-1]
		q += " WHERE status IN (" + placeholders + ")"
		for _, s := range a.StatusIn {
			args = append(args, s)
		}
	} else {
		q += " WHERE status != 'archived'"
	}
	q += " ORDER BY flagship DESC, COALESCE(due_date, '9999-12-31') ASC"
	rows, err := d.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type m struct {
		ID            string `json:"id"`
		Title         string `json:"title"`
		Description   string `json:"description"`
		DueDate       string `json:"due_date"`
		Status        string `json:"status"`
		Flagship      bool   `json:"flagship"`
	}
	type out struct {
		Milestones []m `json:"milestones"`
	}
	var o out
	for rows.Next() {
		var x m
		var flagship int
		if err := rows.Scan(&x.ID, &x.Title, &x.Description, &x.DueDate, &x.Status, &flagship); err != nil {
			return "", err
		}
		x.Flagship = flagship == 1
		o.Milestones = append(o.Milestones, x)
	}
	return jsonString(o)
}

type goalsOutputArgs struct {
	Days int `json:"days,omitempty"`
}

func (d *ToolDispatcher) goalsRecentOutput(ctx context.Context, raw json.RawMessage) (string, error) {
	var a goalsOutputArgs
	_ = json.Unmarshal(raw, &a)
	if a.Days <= 0 {
		a.Days = 90
	}
	since := d.Now().AddDate(0, 0, -a.Days).Format("2006-01-02")
	rows, err := d.DB.QueryContext(ctx,
		`SELECT date, category, title, COALESCE(url,'') FROM goal_output_log
		  WHERE date >= ? ORDER BY date DESC LIMIT 200`, since)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type e struct {
		Date     string `json:"date"`
		Category string `json:"category"`
		Title    string `json:"title"`
		URL      string `json:"url"`
	}
	type out struct {
		Days    int `json:"days"`
		Count   int `json:"count"`
		Entries []e `json:"entries"`
	}
	o := out{Days: a.Days}
	for rows.Next() {
		var x e
		if err := rows.Scan(&x.Date, &x.Category, &x.Title, &x.URL); err != nil {
			return "", err
		}
		o.Entries = append(o.Entries, x)
	}
	o.Count = len(o.Entries)
	return jsonString(o)
}

// ─── Health tools ─────────────────────────────────────────────────────────

func (d *ToolDispatcher) healthToday(ctx context.Context) (string, error) {
	today := d.Now().UTC().Format("2006-01-02")
	type out struct {
		Date          string   `json:"date"`
		RecoveryScore *int     `json:"recovery_score"`
		HRVms         *float64 `json:"hrv_ms"`
		RHR           *int     `json:"rhr"`
		SleepMin      *int     `json:"sleep_duration_min"`
		SleepScore    *int     `json:"sleep_score"`
		StrainScore   *float64 `json:"strain_score"`
		Verdict       string   `json:"verdict"`
	}
	o := out{Date: today}

	var score, rhr, sleepMin, sleepScore sql.NullInt64
	var hrv, strain sql.NullFloat64
	_ = d.DB.QueryRowContext(ctx,
		`SELECT r.score, r.hrv_ms, r.rhr, s.duration_min, s.score, t.score
		   FROM (SELECT 1) x
		   LEFT JOIN health_recovery r ON r.date = ?
		   LEFT JOIN health_sleep s    ON s.date = ?
		   LEFT JOIN health_strain t   ON t.date = ?`,
		today, today, today).Scan(&score, &hrv, &rhr, &sleepMin, &sleepScore, &strain)

	if score.Valid {
		v := int(score.Int64)
		o.RecoveryScore = &v
		switch {
		case v >= 70:
			o.Verdict = "push"
		case v >= 40:
			o.Verdict = "maintain"
		default:
			o.Verdict = "recover"
		}
	}
	if hrv.Valid     { v := hrv.Float64;          o.HRVms = &v }
	if rhr.Valid     { v := int(rhr.Int64);        o.RHR = &v }
	if sleepMin.Valid{ v := int(sleepMin.Int64);   o.SleepMin = &v }
	if sleepScore.Valid { v := int(sleepScore.Int64); o.SleepScore = &v }
	if strain.Valid  { v := strain.Float64;        o.StrainScore = &v }
	return jsonString(o)
}

type healthHistoryArgs struct {
	Days int `json:"days,omitempty"`
}

func (d *ToolDispatcher) healthRecoveryHistory(ctx context.Context, raw json.RawMessage) (string, error) {
	var a healthHistoryArgs
	_ = json.Unmarshal(raw, &a)
	if a.Days <= 0 {
		a.Days = 14
	}
	if a.Days > 90 {
		a.Days = 90
	}
	rows, err := d.DB.QueryContext(ctx,
		`SELECT date, score, hrv_ms, rhr FROM health_recovery
		   ORDER BY date DESC LIMIT ?`, a.Days)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type row struct {
		Date  string   `json:"date"`
		Score *int     `json:"score"`
		HRV   *float64 `json:"hrv_ms"`
		RHR   *int     `json:"rhr"`
	}
	type out struct {
		Days int   `json:"days"`
		Rows []row `json:"rows"`
	}
	o := out{Days: a.Days}
	for rows.Next() {
		var r row
		var score, rhr sql.NullInt64
		var hrv sql.NullFloat64
		if err := rows.Scan(&r.Date, &score, &hrv, &rhr); err != nil {
			return "", err
		}
		if score.Valid { v := int(score.Int64); r.Score = &v }
		if hrv.Valid   { v := hrv.Float64;       r.HRV = &v }
		if rhr.Valid   { v := int(rhr.Int64);    r.RHR = &v }
		o.Rows = append(o.Rows, r)
	}
	return jsonString(o)
}

type healthSupplementArgs struct {
	Days int `json:"days,omitempty"`
}

func (d *ToolDispatcher) healthSupplements(ctx context.Context, raw json.RawMessage) (string, error) {
	var a healthSupplementArgs
	_ = json.Unmarshal(raw, &a)
	if a.Days <= 0 {
		a.Days = 7
	}
	since := d.Now().AddDate(0, 0, -a.Days).Unix()

	defRows, err := d.DB.QueryContext(ctx,
		`SELECT id, name, COALESCE(dose,''), COALESCE(category,'supplement'),
		        cycle_days_on, cycle_days_off
		   FROM health_supplement_defs WHERE active = 1`)
	if err != nil {
		return "", err
	}
	defer defRows.Close()
	type def struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Dose        string `json:"dose"`
		Category    string `json:"category"`
		CycleOn     *int   `json:"cycle_on"`
		CycleOff    *int   `json:"cycle_off"`
		Doses7d     int    `json:"doses_in_window"`
	}
	defs := map[string]*def{}
	var order []*def
	for defRows.Next() {
		var d def
		var on, off sql.NullInt64
		if err := defRows.Scan(&d.ID, &d.Name, &d.Dose, &d.Category, &on, &off); err != nil {
			return "", err
		}
		if on.Valid  { v := int(on.Int64);  d.CycleOn = &v }
		if off.Valid { v := int(off.Int64); d.CycleOff = &v }
		defs[d.ID] = &d
		order = append(order, &d)
	}

	doseRows, err := d.DB.QueryContext(ctx,
		`SELECT def_id, COUNT(*) FROM health_supplement_log
		  WHERE taken_at >= ? GROUP BY def_id`, since)
	if err != nil {
		return "", err
	}
	defer doseRows.Close()
	for doseRows.Next() {
		var id string
		var n int
		if err := doseRows.Scan(&id, &n); err != nil {
			return "", err
		}
		if d, ok := defs[id]; ok {
			d.Doses7d = n
		}
	}

	type out struct {
		WindowDays  int    `json:"window_days"`
		Definitions []*def `json:"definitions"`
	}
	return jsonString(out{WindowDays: a.Days, Definitions: order})
}

// ─── helpers ──────────────────────────────────────────────────────────────

func jsonString(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func schemaObj(props map[string]any, required []string) json.RawMessage {
	if props == nil {
		props = map[string]any{}
	}
	s := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	b, _ := json.Marshal(s)
	return b
}
func schemaStringDesc(d string) map[string]any { return map[string]any{"type": "string", "description": d} }
func schemaIntDesc(d string) map[string]any    { return map[string]any{"type": "integer", "description": d} }
func schemaStringArrayDesc(d string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": d}
}

func stringFromMap(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
func boolFromMap(m map[string]any, k string) bool {
	if v, ok := m[k]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
