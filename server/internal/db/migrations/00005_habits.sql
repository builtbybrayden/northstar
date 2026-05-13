-- +goose Up
-- Habit tracker — daily yes/no tracking distinct from goal_milestones.
-- A "habit" is something you want to do regularly (lift weights, no
-- alcohol, study cyber). A "milestone" is a one-time achievement (pass
-- OSCP, ship feature X).
--
-- Per-day entries live in goal_habit_log. count > 0 means done; 0 means
-- explicitly not done (vs. no row = unrecorded). This three-state shape
-- lets the heatmap distinguish "skipped" from "didn't track yet".

CREATE TABLE goal_habits (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description_md TEXT,
    color TEXT NOT NULL DEFAULT 'goals',
    target_per_week INTEGER NOT NULL DEFAULT 7,
    active INTEGER NOT NULL DEFAULT 1,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE goal_habit_log (
    habit_id TEXT NOT NULL,
    date TEXT NOT NULL,                     -- YYYY-MM-DD
    count INTEGER NOT NULL DEFAULT 0,       -- 0 = explicitly skipped; >0 = done
    notes TEXT,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (habit_id, date),
    FOREIGN KEY (habit_id) REFERENCES goal_habits(id) ON DELETE CASCADE
);

CREATE INDEX idx_habit_log_date ON goal_habit_log(date);

-- +goose Down
DROP INDEX IF EXISTS idx_habit_log_date;
DROP TABLE IF EXISTS goal_habit_log;
DROP TABLE IF EXISTS goal_habits;
