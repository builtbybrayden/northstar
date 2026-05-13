package goals

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	_ "modernc.org/sqlite"
)

func habitsDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	stmts := []string{
		`CREATE TABLE goal_habits (
		   id TEXT PRIMARY KEY, name TEXT NOT NULL, description_md TEXT,
		   color TEXT NOT NULL DEFAULT 'goals', target_per_week INTEGER NOT NULL DEFAULT 7,
		   active INTEGER NOT NULL DEFAULT 1, display_order INTEGER NOT NULL DEFAULT 0,
		   created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE goal_habit_log (
		   habit_id TEXT NOT NULL, date TEXT NOT NULL,
		   count INTEGER NOT NULL DEFAULT 0, notes TEXT,
		   updated_at INTEGER NOT NULL,
		   PRIMARY KEY (habit_id, date))`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return d
}

// pinnedHandlers returns a Handlers whose Now() always returns `now`.
func pinnedHandlers(db *sql.DB, now time.Time) *Handlers {
	return &Handlers{DB: db, Now: func() time.Time { return now }}
}

// routeRequest wraps a chi-routed call so URL params get populated correctly.
func routeRequest(method, path, body string, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestCreateHabit_RequiresName(t *testing.T) {
	db := habitsDB(t)
	h := pinnedHandlers(db, time.Now())
	r := routeRequest("POST", "/api/goals/habits", `{"name":""}`, nil)
	w := httptest.NewRecorder()
	h.CreateHabit(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty name, got %d", w.Code)
	}
}

func TestCreateHabit_DefaultsAndCreated(t *testing.T) {
	db := habitsDB(t)
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	h := pinnedHandlers(db, now)
	r := routeRequest("POST", "/api/goals/habits",
		`{"name":"Lift","target_per_week":4}`, nil)
	w := httptest.NewRecorder()
	h.CreateHabit(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d  body=%s", w.Code, w.Body.String())
	}
	var got Habit
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Name != "Lift" || got.TargetPerWeek != 4 || !got.Active || got.Color != "goals" {
		t.Errorf("unexpected habit: %+v", got)
	}
}

func TestPutHabitLog_ToggleOnEmptyBody(t *testing.T) {
	db := habitsDB(t)
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	h := pinnedHandlers(db, now)
	if _, err := db.Exec(`INSERT INTO goal_habits
		(id, name, description_md, color, target_per_week, active, display_order, created_at, updated_at)
		VALUES ('h1', 'Test', '', 'goals', 7, 1, 0, 0, 0)`); err != nil {
		t.Fatal(err)
	}

	// First toggle → done (count=1)
	r := routeRequest("PUT", "/api/goals/habits/h1/log/2026-05-13", "",
		map[string]string{"id": "h1", "date": "2026-05-13"})
	r.ContentLength = 0
	w := httptest.NewRecorder()
	h.PutHabitLog(w, r)
	if w.Code != 200 {
		t.Fatalf("first toggle status %d  body=%s", w.Code, w.Body.String())
	}
	var first HabitEntry
	_ = json.Unmarshal(w.Body.Bytes(), &first)
	if first.Count != 1 {
		t.Errorf("first toggle count = %d, want 1", first.Count)
	}

	// Second toggle → skipped (count=0)
	r = routeRequest("PUT", "/api/goals/habits/h1/log/2026-05-13", "",
		map[string]string{"id": "h1", "date": "2026-05-13"})
	r.ContentLength = 0
	w = httptest.NewRecorder()
	h.PutHabitLog(w, r)
	if w.Code != 200 {
		t.Fatalf("second toggle status %d", w.Code)
	}
	var second HabitEntry
	_ = json.Unmarshal(w.Body.Bytes(), &second)
	if second.Count != 0 {
		t.Errorf("second toggle count = %d, want 0", second.Count)
	}
}

func TestPutHabitLog_ExplicitCount(t *testing.T) {
	db := habitsDB(t)
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	h := pinnedHandlers(db, now)
	if _, err := db.Exec(`INSERT INTO goal_habits VALUES
		('h1', 'Push-ups', '', 'goals', 7, 1, 0, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	r := routeRequest("PUT", "/api/goals/habits/h1/log/2026-05-12",
		`{"count":42,"notes":"set 1 / 30 + set 2 / 12"}`,
		map[string]string{"id": "h1", "date": "2026-05-12"})
	w := httptest.NewRecorder()
	h.PutHabitLog(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d  body=%s", w.Code, w.Body.String())
	}
	var got HabitEntry
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Count != 42 {
		t.Errorf("count = %d, want 42", got.Count)
	}
}

func TestPutHabitLog_404WhenHabitMissing(t *testing.T) {
	db := habitsDB(t)
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	h := pinnedHandlers(db, now)
	r := routeRequest("PUT", "/api/goals/habits/ghost/log/2026-05-13",
		`{"count":1}`,
		map[string]string{"id": "ghost", "date": "2026-05-13"})
	w := httptest.NewRecorder()
	h.PutHabitLog(w, r)
	if w.Code != 404 {
		t.Errorf("expected 404 for missing habit, got %d", w.Code)
	}
}

func TestHabitStats_StreakAndLast30(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	mk := func(date string, count int) HabitEntry {
		return HabitEntry{Date: date, Count: count}
	}
	// 3-day current streak (today, yesterday, day-before), then a gap.
	entries := []HabitEntry{
		mk("2026-04-15", 1),
		mk("2026-04-16", 1),
		mk("2026-04-17", 0), // skip
		mk("2026-05-11", 1),
		mk("2026-05-12", 1),
		mk("2026-05-13", 1),
	}
	streak, last30 := habitStats(entries, now)
	if streak != 3 {
		t.Errorf("streak = %d, want 3", streak)
	}
	// Within last 30 days: 2026-04-15 onward, only the done-counts.
	// 04-15, 04-16, 05-11, 05-12, 05-13 = 5 done in last 30
	if last30 != 5 {
		t.Errorf("last30 = %d, want 5", last30)
	}
}

func TestHabitStats_NoEntries(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	streak, last30 := habitStats(nil, now)
	if streak != 0 || last30 != 0 {
		t.Errorf("expected zero stats for empty entries, got streak=%d last30=%d", streak, last30)
	}
}

func TestListHabits_RoundTripWithEntries(t *testing.T) {
	db := habitsDB(t)
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	h := pinnedHandlers(db, now)
	if _, err := db.Exec(`INSERT INTO goal_habits VALUES
		('h1', 'Run', '', 'goals', 5, 1, 0, 0, 0),
		('h2', 'Bedtime', '', 'health', 7, 0, 0, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO goal_habit_log (habit_id, date, count, notes, updated_at) VALUES
		('h1', '2026-05-13', 1, '', 0),
		('h1', '2026-05-12', 1, '', 0),
		('h1', '2026-05-11', 0, '', 0)`); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/goals/habits", nil)
	w := httptest.NewRecorder()
	h.ListHabits(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d  body=%s", w.Code, w.Body.String())
	}
	var got []HabitWithStats
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	// h2 is inactive, so default list returns only h1.
	if len(got) != 1 {
		t.Fatalf("len(got)=%d, want 1 (inactive habit should be hidden by default): %+v", len(got), got)
	}
	if got[0].ID != "h1" {
		t.Errorf("expected h1, got %s", got[0].ID)
	}
	if got[0].StreakDays != 2 {
		t.Errorf("streak = %d, want 2", got[0].StreakDays)
	}
	if len(got[0].Entries) != 3 {
		t.Errorf("entries = %d, want 3", len(got[0].Entries))
	}
}

func TestListHabits_IncludeInactive(t *testing.T) {
	db := habitsDB(t)
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	h := pinnedHandlers(db, now)
	if _, err := db.Exec(`INSERT INTO goal_habits VALUES
		('h1', 'Active',   '', 'goals',  7, 1, 0, 0, 0),
		('h2', 'Archived', '', 'health', 7, 0, 0, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/goals/habits?inactive=1", nil)
	w := httptest.NewRecorder()
	h.ListHabits(w, r)
	var got []HabitWithStats
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Errorf("inactive=1 should return both, got %d", len(got))
	}
}
