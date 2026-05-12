-- +goose Up
-- Phase 6 polish — persist Anthropic token usage per assistant message.
-- usage_json holds the four counts from the Messages API stream:
--   {"input_tokens", "output_tokens",
--    "cache_creation_input_tokens", "cache_read_input_tokens"}
-- Stored as JSON (not split columns) because Anthropic may add new fields
-- and we want to capture them without a migration churn.

ALTER TABLE ai_messages ADD COLUMN usage_json TEXT;

-- +goose Down
-- SQLite supports DROP COLUMN since 3.35 — modernc.org/sqlite has it.
ALTER TABLE ai_messages DROP COLUMN usage_json;
