-- +goose Up
-- Phase 7.2 — seed two categories the user asked for so they appear in
-- the iOS category picker even before any transactions are recategorized
-- into them. Idempotent via INSERT OR IGNORE so re-running this migration
-- on an environment that already has these rows is a no-op.
--
-- Use unix epoch from SQLite for updated_at (no need for the goose driver
-- to inject a Go-side timestamp).

INSERT OR IGNORE INTO fin_budget_targets
  (category, monthly_cents, rationale, threshold_pcts, push_enabled, category_group, updated_at)
VALUES
  ('Rent',             0, 'added by 00007 (user-requested)', '[50,75,90,100]', 1, 'Living Expenses', CAST(strftime('%s','now') AS INTEGER)),
  ('Vehicle Payments', 0, 'added by 00007 (user-requested)', '[50,75,90,100]', 1, 'Transportation',  CAST(strftime('%s','now') AS INTEGER));

-- +goose Down
-- Only delete the rows if they still have zero spend rationale tag, so we
-- don't accidentally remove a category the user re-purposed.
DELETE FROM fin_budget_targets WHERE category IN ('Rent','Vehicle Payments')
  AND rationale LIKE '%user-requested%';
