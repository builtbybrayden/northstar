---
title: Notifications
description: How push categories work, dedup, quiet hours, and daily caps.
---

The notification engine evaluates rules against incoming events (new
transaction, budget threshold crossed, recovery drop) and dispatches push
messages via the configured sender.

## Categories

| Category | Trigger | Default cap |
|---|---|---|
| `purchase` | New transaction lands | 99/day |
| `budget_threshold` | Category crosses 50/75/90/100% | One per threshold per category per month |
| `anomaly` | 3× median for known payee, or new merchant ≥ $200 | 5/day |
| `daily_brief` | Cron, configurable morning time | 1/day |
| `evening_retro` | Cron, configurable evening time | 1/day |
| `supplement` | Scheduled dose | Per definition |
| `health_insight` | Recovery dropped ≥ 30 points day-over-day | 1/day |
| `goal_milestone` | Milestone status change | Realtime |
| `subscription_new` | New recurring charge detected | Per detection |
| `weekly_retro` | Friday 9pm | Weekly |

## Quiet hours

Per-category and per-user. The default global window is 22:00–07:00 local.
Wrap-around windows are evaluated correctly (e.g. 23:00–06:00).

Critical alerts bypass quiet hours:

- `anomaly` (any)
- `budget_threshold` at 100%
- `health_insight` with `priority >= 9`

## Dedup

Every notification persists with a `dedup_key` UNIQUE constraint. A repeat
event with the same key is silently dropped. Examples:

- `purchase`: `txn:<actual_id>`
- `budget_threshold`: `bt:<category>:<month>:<pct>`
- `anomaly`: `anomaly:<merchant>:<date>`

## Daily caps

`notif_daily_counts(category, date, count)` tracks how many fired per category
per day. The composer reads + increments atomically. Over-cap events are
dropped silently.

## Sender backends

| Sender | When used | Notes |
|---|---|---|
| `LogSender` | Default | Writes a structured log line — useful for dev and the "I don't want push yet" path |
| `APNSSender` | When APNs creds are configured | Stubbed; needs Apple Developer Program enrollment + `sideshow/apns2` wired |

## REST endpoints

| Method + path | Notes |
|---|---|
| `GET /api/notifications/feed` | Recent notifications, paginated |
| `GET /api/notifications/unread-count` | Badge driver |
| `POST /api/notifications/:id/read` | Mark single |
| `GET /api/notifications/rules` | All per-category rules |
| `PATCH /api/notifications/rules/:category` | Update enabled / quiet hours / delivery / cap |

See [API reference: Notifications](/api/notifications/) for full payloads.
