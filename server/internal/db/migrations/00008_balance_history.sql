-- +goose Up
-- Daily snapshot of every open account's balance. One row per (account, day)
-- in UTC. The finance syncer writes today's row on every tick using INSERT
-- OR REPLACE, so the latest sync of each day wins — and re-running on the
-- same day is idempotent.
--
-- Powers historical net-worth charts and forecast-vs-actual comparisons
-- without requiring Actual to retain the history itself.

CREATE TABLE fin_account_balance_history (
    account_id    TEXT    NOT NULL,
    date          TEXT    NOT NULL,
    balance_cents INTEGER NOT NULL,
    on_budget     INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (account_id, date)
);

CREATE INDEX idx_balance_history_date ON fin_account_balance_history(date);

-- +goose Down
DROP INDEX IF EXISTS idx_balance_history_date;
DROP TABLE IF EXISTS fin_account_balance_history;
