-- +goose Up
-- Seed the rule row for forecast_warning notifications introduced in
-- Phase 7.5. Bypasses quiet hours because the user wants to know
-- *before* sleeping that their balance is about to dip below the
-- configured cash floor.

INSERT OR IGNORE INTO notif_rules
    (id, category, enabled, quiet_hours_start, quiet_hours_end,
     bypass_quiet, delivery, max_per_day, updated_at)
VALUES
    ('seed-forecast-warning', 'forecast_warning', 1, '22:00', '07:00',
     1, 'push', 2, strftime('%s','now'));

-- +goose Down
DELETE FROM notif_rules WHERE id = 'seed-forecast-warning';
