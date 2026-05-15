-- +goose Up
-- Persist the transfer linkage + split flag that Actual exposes on every
-- transaction. Without these, every CC payment, savings transfer, and
-- broker funding looks like real income/spend to the aggregator and
-- inflates the donut totals by 2-3x.
--
-- transfer_id: peer transaction id when this row is one leg of an
--   inter-account move. Set on BOTH legs in Actual. We exclude any
--   non-NULL transfer_id from income/spend totals.
-- is_parent: 1 when this row is the gross of a split transaction. The
--   children sum to the same gross amount and carry the categories.
--   Counting both double-counts the line, so we skip parents.

ALTER TABLE fin_transactions ADD COLUMN transfer_id TEXT;
ALTER TABLE fin_transactions ADD COLUMN is_parent  INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_fin_transactions_transfer ON fin_transactions(transfer_id)
    WHERE transfer_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_fin_transactions_transfer;
ALTER TABLE fin_transactions DROP COLUMN is_parent;
ALTER TABLE fin_transactions DROP COLUMN transfer_id;
