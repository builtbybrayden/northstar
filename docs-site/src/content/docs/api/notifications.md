---
title: Notifications API
description: REST surface for the feed, unread count, mark-read, and per-category rules.
---

All endpoints require `Authorization: Bearer <token>`.

## Feed

`GET /api/notifications/feed?limit=N`

```json
[
  {
    "id": "...",
    "category": "budget_threshold",
    "title": "Restaurants at 105%",
    "body": "$525 of $500 spent",
    "payload": { "category": "Restaurants", "pct": 105 },
    "priority": 9,
    "delivered_at": 1747000000,
    "read_at": null
  }
]
```

## `GET /api/notifications/unread-count`

```json
{ "count": 3 }
```

Drives the bell badge in the iOS Home toolbar.

## `POST /api/notifications/:id/read`

Marks a single notification read. No body.

## Rules

`GET /api/notifications/rules`

Returns every per-category rule:

```json
[
  {
    "category": "purchase",
    "enabled": true,
    "quiet_hours_start": "22:00",
    "quiet_hours_end": "07:00",
    "bypass_quiet": false,
    "delivery": "push",
    "max_per_day": 99
  }
]
```

`PATCH /api/notifications/rules/:category`

Patch any subset. Useful for the Settings → Notifications screen.

```json
{
  "enabled": false,
  "max_per_day": 5
}
```

## Live stream

`GET /api/notifications/stream` opens a Server-Sent Events connection. The
server publishes one `data:` line per successful notification fire, plus a
heartbeat comment (`: ping`) every 25 seconds so idle connections survive NAT
and proxy timeouts.

```
data: {"type":"notification","id":"...","category":"budget_threshold","title":"Restaurants at 105%","body":"$525 of $500 spent","priority":9,"created_at":1747000000}

```

The iOS app subscribes whenever the Home or Notifications tab is on screen and
auto-reconnects with a 3-second backoff if the server closes the stream.
Multiple subscribers (e.g. Home + Notifications open at once) all receive each
event independently.

## Categories

See [Notifications configuration](/configuration/notifications/) for the
canonical list of categories and triggers.
