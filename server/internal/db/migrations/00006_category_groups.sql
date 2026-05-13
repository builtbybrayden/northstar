-- +goose Up
-- Parent-group classification for budget categories. Lets the iOS Finance
-- tab render the 25+ leaf categories under a handful of headers (Living
-- Expenses, Transportation, Dining & Entertainment, Savings & Income,
-- Miscellaneous) without losing the per-category granularity.
--
-- The column is nullable so the server can backfill defaults via a name
-- heuristic on next sync; rows that came in before this migration applied
-- will pick up the default the first time the heuristic runs against them.
-- Users can override the default via
--   PATCH /api/finance/budget-targets/:category {"category_group": "..."}

ALTER TABLE fin_budget_targets ADD COLUMN category_group TEXT;

-- +goose Down
ALTER TABLE fin_budget_targets DROP COLUMN category_group;
