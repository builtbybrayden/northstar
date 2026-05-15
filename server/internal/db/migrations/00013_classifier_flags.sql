-- +goose Up
-- Foolproof per-flow classification: user-overridable flags at both
-- the account level (include_in_income) and the transaction level
-- (flow_override) so the donut + drilldown match exactly what the
-- user wants, regardless of how Actual categorized the row.
--
-- include_in_income: nullable. NULL = use heuristic (account type +
-- name); 0/1 = explicit user override.
--
-- flow_override: nullable. NULL = auto-classify based on amount sign,
-- transfer linkage, and account flags. Otherwise one of:
--   'income'   force into income
--   'spent'    force into spent
--   'saved'    force into saved
--   'exclude'  drop from all flows (use for rogue rows the auto-
--              classifier got wrong and isn't worth re-categorizing)

ALTER TABLE fin_accounts     ADD COLUMN include_in_income INTEGER;
ALTER TABLE fin_transactions ADD COLUMN flow_override     TEXT;

CREATE INDEX idx_fin_transactions_flow_override
    ON fin_transactions(flow_override)
    WHERE flow_override IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_fin_transactions_flow_override;
ALTER TABLE fin_transactions DROP COLUMN flow_override;
ALTER TABLE fin_accounts     DROP COLUMN include_in_income;
