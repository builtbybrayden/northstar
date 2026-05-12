-- +goose Up
-- Northstar — initial schema. Mirrors plan §5.

-- ─── Core ─────────────────────────────────────────────────────────────────
CREATE TABLE users (
  id                  TEXT PRIMARY KEY,
  email               TEXT,
  claude_api_key_enc  BLOB,
  settings_json       TEXT NOT NULL DEFAULT '{}',
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL
);

CREATE TABLE devices (
  id          TEXT PRIMARY KEY,
  user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT,
  token_hash  TEXT NOT NULL UNIQUE,    -- SHA-256 of the bearer token (raw token never stored)
  apns_token  TEXT,
  paired_at   INTEGER NOT NULL,
  last_seen   INTEGER,
  revoked_at  INTEGER
);
CREATE INDEX idx_devices_user ON devices(user_id);
CREATE INDEX idx_devices_token ON devices(token_hash);

-- Pairing codes — short-lived one-time codes a device exchanges for a token
CREATE TABLE pairing_codes (
  code        TEXT PRIMARY KEY,
  user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token       TEXT NOT NULL,         -- bearer token the device receives
  expires_at  INTEGER NOT NULL,
  consumed_at INTEGER
);
CREATE INDEX idx_pairing_codes_exp ON pairing_codes(expires_at);

-- ─── Finance (mirror — Actual is source of truth) ────────────────────────
CREATE TABLE fin_accounts (
  actual_id     TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  type          TEXT,
  balance_cents INTEGER NOT NULL DEFAULT 0,
  on_budget     INTEGER NOT NULL DEFAULT 1,
  closed        INTEGER NOT NULL DEFAULT 0,
  updated_at    INTEGER NOT NULL
);

CREATE TABLE fin_transactions (
  actual_id    TEXT PRIMARY KEY,
  account_id   TEXT NOT NULL REFERENCES fin_accounts(actual_id),
  date         TEXT NOT NULL,
  payee        TEXT,
  category     TEXT,
  amount_cents INTEGER NOT NULL,
  notes        TEXT,
  imported_at  INTEGER NOT NULL
);
CREATE INDEX idx_fin_txn_date ON fin_transactions(date);
CREATE INDEX idx_fin_txn_category_date ON fin_transactions(category, date);
CREATE INDEX idx_fin_txn_account_date ON fin_transactions(account_id, date);

CREATE TABLE fin_budget_targets (
  category         TEXT PRIMARY KEY,
  monthly_cents    INTEGER NOT NULL,
  rationale        TEXT,
  threshold_pcts   TEXT NOT NULL DEFAULT '[50,75,90,100]',
  push_enabled     INTEGER NOT NULL DEFAULT 1,
  updated_at       INTEGER NOT NULL
);

CREATE TABLE fin_threshold_state (
  category        TEXT NOT NULL,
  month           TEXT NOT NULL,           -- 'YYYY-MM'
  last_fired_pct  INTEGER NOT NULL,
  fired_at        INTEGER NOT NULL,
  PRIMARY KEY (category, month)
);

-- ─── Goals (NATIVE — Northstar owns this) ────────────────────────────────
CREATE TABLE goal_milestones (
  id              TEXT PRIMARY KEY,
  title           TEXT NOT NULL,
  description_md  TEXT,
  due_date        TEXT,
  status          TEXT NOT NULL DEFAULT 'pending',   -- pending/in_progress/done/archived
  flagship        INTEGER NOT NULL DEFAULT 0,
  display_order   INTEGER NOT NULL DEFAULT 0,
  created_at      INTEGER NOT NULL,
  updated_at      INTEGER NOT NULL
);
CREATE INDEX idx_goal_milestones_status ON goal_milestones(status);
CREATE INDEX idx_goal_milestones_due ON goal_milestones(due_date);

CREATE TABLE goal_daily_log (
  date            TEXT PRIMARY KEY,                  -- YYYY-MM-DD
  items_json      TEXT NOT NULL DEFAULT '[]',
  reflection_md   TEXT,
  streak_count    INTEGER NOT NULL DEFAULT 0,
  updated_at      INTEGER NOT NULL
);

CREATE TABLE goal_weekly_tracker (
  week_of           TEXT PRIMARY KEY,                -- Monday YYYY-MM-DD
  theme             TEXT,
  weekly_goals_json TEXT NOT NULL DEFAULT '[]',
  retro_md          TEXT,
  updated_at        INTEGER NOT NULL
);

CREATE TABLE goal_monthly (
  month               TEXT PRIMARY KEY,              -- YYYY-MM
  monthly_goals_json  TEXT NOT NULL DEFAULT '[]',
  retro_md            TEXT,
  updated_at          INTEGER NOT NULL
);

CREATE TABLE goal_output_log (
  id        TEXT PRIMARY KEY,
  date      TEXT NOT NULL,
  category  TEXT NOT NULL,           -- cve/blog/talk/tool/cert/pr/report
  title     TEXT NOT NULL,
  body_md   TEXT,
  url       TEXT,
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_goal_output_date ON goal_output_log(date);
CREATE INDEX idx_goal_output_category ON goal_output_log(category);

CREATE TABLE goal_networking_log (
  id              TEXT PRIMARY KEY,
  date            TEXT NOT NULL,
  person          TEXT NOT NULL,
  context         TEXT,
  next_action     TEXT,
  next_action_due TEXT,
  created_at      INTEGER NOT NULL
);
CREATE INDEX idx_goal_net_date ON goal_networking_log(date);

CREATE TABLE goal_reminders (
  id             TEXT PRIMARY KEY,
  title          TEXT NOT NULL,
  body           TEXT,
  recurrence     TEXT NOT NULL,        -- cron-like: 'every monday 09:00'
  next_fires_at  INTEGER,
  active         INTEGER NOT NULL DEFAULT 1,
  created_at     INTEGER NOT NULL
);
CREATE INDEX idx_goal_rem_next ON goal_reminders(active, next_fires_at);

-- ─── Health ──────────────────────────────────────────────────────────────
CREATE TABLE health_recovery (
  date     TEXT PRIMARY KEY,         -- YYYY-MM-DD
  score    INTEGER,                  -- 0–100
  hrv_ms   REAL,
  rhr      INTEGER,
  source   TEXT NOT NULL DEFAULT 'whoop'
);

CREATE TABLE health_sleep (
  date          TEXT PRIMARY KEY,
  duration_min  INTEGER,
  score         INTEGER,
  debt_min      INTEGER,
  source        TEXT NOT NULL DEFAULT 'whoop'
);

CREATE TABLE health_strain (
  date     TEXT PRIMARY KEY,
  score    REAL,
  avg_hr   INTEGER,
  max_hr   INTEGER,
  source   TEXT NOT NULL DEFAULT 'whoop'
);

CREATE TABLE health_supplement_defs (
  id                TEXT PRIMARY KEY,
  name              TEXT NOT NULL,
  dose              TEXT,
  category          TEXT,                 -- supplement / peptide / medication
  schedule_json     TEXT,
  cycle_days_on     INTEGER,
  cycle_days_off    INTEGER,
  reminder_enabled  INTEGER NOT NULL DEFAULT 1,
  active            INTEGER NOT NULL DEFAULT 1,
  prescribing_doc   TEXT,
  notes             TEXT,
  created_at        INTEGER NOT NULL
);

CREATE TABLE health_supplement_log (
  id        TEXT PRIMARY KEY,
  def_id    TEXT NOT NULL REFERENCES health_supplement_defs(id) ON DELETE CASCADE,
  taken_at  INTEGER NOT NULL,
  notes     TEXT
);
CREATE INDEX idx_supp_log_taken ON health_supplement_log(taken_at);
CREATE INDEX idx_supp_log_def ON health_supplement_log(def_id, taken_at);

CREATE TABLE health_mood_log (
  date    TEXT PRIMARY KEY,
  mood    INTEGER,
  energy  INTEGER,
  focus   INTEGER,
  notes   TEXT
);

-- ─── Notifications (user-editable rules) ─────────────────────────────────
CREATE TABLE notif_rules (
  id                 TEXT PRIMARY KEY,
  category           TEXT NOT NULL UNIQUE,      -- purchase/budget_threshold/health_insight/...
  enabled            INTEGER NOT NULL DEFAULT 1,
  quiet_hours_start  TEXT,                       -- 'HH:MM'
  quiet_hours_end    TEXT,
  bypass_quiet       INTEGER NOT NULL DEFAULT 0,
  delivery           TEXT NOT NULL DEFAULT 'push',  -- push/live_activity/silent_badge
  max_per_day        INTEGER NOT NULL DEFAULT 99,
  updated_at         INTEGER NOT NULL
);

-- ─── AI conversations ────────────────────────────────────────────────────
CREATE TABLE ai_conversations (
  id            TEXT PRIMARY KEY,
  started_at    INTEGER NOT NULL,
  title         TEXT,
  pillar_scope  TEXT NOT NULL DEFAULT '[]',
  archived      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE ai_messages (
  id              TEXT PRIMARY KEY,
  conv_id         TEXT NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
  role            TEXT NOT NULL,                  -- user/assistant/tool
  content_json    TEXT NOT NULL,
  tool_calls_json TEXT,
  created_at      INTEGER NOT NULL
);
CREATE INDEX idx_ai_msg_conv ON ai_messages(conv_id, created_at);

-- ─── Seed: default notification rules ────────────────────────────────────
INSERT INTO notif_rules (id, category, enabled, quiet_hours_start, quiet_hours_end, bypass_quiet, delivery, max_per_day, updated_at) VALUES
  ('seed-purchase',         'purchase',         1, '22:00', '07:00', 0, 'push', 99, strftime('%s','now')),
  ('seed-budget-threshold', 'budget_threshold', 1, '22:00', '07:00', 1, 'push', 99, strftime('%s','now')),
  ('seed-anomaly',          'anomaly',          1, '22:00', '07:00', 0, 'push', 99, strftime('%s','now')),
  ('seed-daily-brief',      'daily_brief',      1, NULL,    NULL,    0, 'push', 99, strftime('%s','now')),
  ('seed-evening-retro',    'evening_retro',    1, NULL,    NULL,    0, 'push', 99, strftime('%s','now')),
  ('seed-supplement',       'supplement',       1, '22:00', '07:00', 0, 'push', 99, strftime('%s','now')),
  ('seed-health-insight',   'health_insight',   1, '22:00', '07:00', 0, 'push',  1, strftime('%s','now')),
  ('seed-goal-milestone',   'goal_milestone',   1, '22:00', '07:00', 0, 'push', 99, strftime('%s','now')),
  ('seed-subscription-new', 'subscription_new', 1, '22:00', '07:00', 0, 'push', 99, strftime('%s','now')),
  ('seed-weekly-retro',     'weekly_retro',     1, NULL,    NULL,    0, 'push',  1, strftime('%s','now'));

-- +goose Down
DROP TABLE IF EXISTS ai_messages;
DROP TABLE IF EXISTS ai_conversations;
DROP TABLE IF EXISTS notif_rules;
DROP TABLE IF EXISTS health_mood_log;
DROP TABLE IF EXISTS health_supplement_log;
DROP TABLE IF EXISTS health_supplement_defs;
DROP TABLE IF EXISTS health_strain;
DROP TABLE IF EXISTS health_sleep;
DROP TABLE IF EXISTS health_recovery;
DROP TABLE IF EXISTS goal_reminders;
DROP TABLE IF EXISTS goal_networking_log;
DROP TABLE IF EXISTS goal_output_log;
DROP TABLE IF EXISTS goal_monthly;
DROP TABLE IF EXISTS goal_weekly_tracker;
DROP TABLE IF EXISTS goal_daily_log;
DROP TABLE IF EXISTS goal_milestones;
DROP TABLE IF EXISTS fin_threshold_state;
DROP TABLE IF EXISTS fin_budget_targets;
DROP TABLE IF EXISTS fin_transactions;
DROP TABLE IF EXISTS fin_accounts;
DROP TABLE IF EXISTS pairing_codes;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS users;
