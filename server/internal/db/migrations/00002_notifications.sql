-- +goose Up
-- Phase 2 — notification persistence + helper tables for anomaly detection.

CREATE TABLE notifications (
  id              TEXT PRIMARY KEY,
  category        TEXT NOT NULL,     -- maps to notif_rules.category
  title           TEXT NOT NULL,
  body            TEXT,
  payload_json    TEXT,              -- {category, amount_cents, dedup_meta, ...}
  priority        INTEGER NOT NULL DEFAULT 5,    -- 1–10; >=8 = bypass quiet hours
  dedup_key       TEXT UNIQUE,                    -- e.g. 'budget_threshold:Restaurants:2026-05:90'
  created_at      INTEGER NOT NULL,
  read_at         INTEGER,
  delivered_at    INTEGER,                        -- when sent to APNs (or log-only acknowledged)
  delivery_status TEXT NOT NULL DEFAULT 'pending' -- pending/sent/failed/skipped
);
CREATE INDEX idx_notif_created   ON notifications(created_at DESC);
CREATE INDEX idx_notif_unread    ON notifications(read_at) WHERE read_at IS NULL;
CREATE INDEX idx_notif_category  ON notifications(category, created_at DESC);

-- ─── Anomaly support — rolling merchant stats ────────────────────────────
-- Updated incrementally during sync. Lets us detect new-merchant >threshold
-- and 3× spend spikes without scanning all transactions every time.
CREATE TABLE fin_merchant_stats (
  payee            TEXT PRIMARY KEY,
  txn_count        INTEGER NOT NULL DEFAULT 0,
  total_abs_cents  INTEGER NOT NULL DEFAULT 0,
  median_abs_cents INTEGER NOT NULL DEFAULT 0,    -- approximate; updated on sync
  max_abs_cents    INTEGER NOT NULL DEFAULT 0,
  first_seen_at    INTEGER NOT NULL,
  last_seen_at     INTEGER NOT NULL
);

-- ─── Daily counters for notification rate limiting ───────────────────────
CREATE TABLE notif_daily_counts (
  category TEXT NOT NULL,
  date     TEXT NOT NULL,                          -- YYYY-MM-DD
  count    INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (category, date)
);

-- +goose Down
DROP TABLE IF EXISTS notif_daily_counts;
DROP TABLE IF EXISTS fin_merchant_stats;
DROP TABLE IF EXISTS notifications;
