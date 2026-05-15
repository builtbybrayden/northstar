-- +goose Up
-- One-row settings table for finance pillar preferences that don't fit
-- on user_settings (which is goal/health/AI-focused). Single row keyed
-- on id=1 so we don't accidentally fork into multiple rows.
--
-- savings_target_pct: percentage of monthly income the user is aiming
-- to save. The Finance tab's saved donut uses (income × pct / 100) as
-- its denominator. Default 25 — a reasonable starting point per the
-- 50/30/20 budget rule.

CREATE TABLE fin_settings (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    savings_target_pct INTEGER NOT NULL DEFAULT 25 CHECK (savings_target_pct BETWEEN 0 AND 100),
    updated_at         INTEGER NOT NULL
);

INSERT INTO fin_settings (id, savings_target_pct, updated_at)
VALUES (1, 25, strftime('%s','now'));

-- +goose Down
DROP TABLE IF EXISTS fin_settings;
