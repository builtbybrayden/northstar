-- +goose Up
-- Per-account override for "is this a savings destination". Nullable —
-- when NULL, the name heuristic (isSavingsDestination) runs; when set,
-- the user has explicitly toggled it on/off and the override wins.
--
-- This unblocks accounts the name heuristic doesn't catch (e.g.
-- "Vanguard Brokerage" misses if literally named "VTSAX Fund 99821") and
-- lets the user opt out of accounts the heuristic over-matches.

ALTER TABLE fin_accounts ADD COLUMN is_savings_destination INTEGER;

-- +goose Down
ALTER TABLE fin_accounts DROP COLUMN is_savings_destination;
