---
title: Goals API
description: REST surface for milestones, daily log, weekly/monthly trackers, output log, networking, reminders, and the daily brief.
---

All endpoints require `Authorization: Bearer <token>`.

## Milestones

| Method + path | Notes |
|---|---|
| `GET /api/goals/milestones?include_archived=1` | Lists all (excluding archived by default) |
| `POST /api/goals/milestones` | Create |
| `GET /api/goals/milestones/:id` | Single |
| `PATCH /api/goals/milestones/:id` | Update title / description / due / status / flagship / display_order |
| `DELETE /api/goals/milestones/:id` | Soft delete (sets status = `archived`) |

Body fields: `title`, `description_md`, `due_date` (YYYY-MM-DD), `status`
(`pending` / `in_progress` / `done` / `archived`), `flagship` (bool),
`display_order` (int).

## Daily log

| Method + path | Notes |
|---|---|
| `GET /api/goals/daily/:date` | Returns `{items: [...], reflection_md, streak_count}` |
| `PUT /api/goals/daily/:date` | UPSERT; recomputes streak |

Items: `[{ text, done, source?, milestone_id? }]`. `source` is one of
`user` / `rollover` / `reminder` and drives the UI label.

## Weekly tracker

| Method + path | Notes |
|---|---|
| `GET /api/goals/weekly/:week_of` | `week_of` = the Monday's YYYY-MM-DD |
| `PUT /api/goals/weekly/:week_of` | UPSERT theme + weekly_goals + retro |

## Monthly tracker

| Method + path | Notes |
|---|---|
| `GET /api/goals/monthly/:month` | `month` = YYYY-MM |
| `PUT /api/goals/monthly/:month` | UPSERT |

## Output log

| Method + path | Notes |
|---|---|
| `GET /api/goals/output?days=N` | Recent entries |
| `POST /api/goals/output` | Add entry |

Categories: `cve` / `blog` / `talk` / `tool` / `cert` / `pr` / `report`.

## Networking log

| Method + path | Notes |
|---|---|
| `GET /api/goals/networking` | All |
| `POST /api/goals/networking` | Add entry |

## Reminders

| Method + path | Notes |
|---|---|
| `GET /api/goals/reminders` | All |
| `POST /api/goals/reminders` | Create with 5-field cron in `recurrence` |
| `PATCH /api/goals/reminders/:id` | Toggle / edit |
| `DELETE /api/goals/reminders/:id` | Hard delete |

## Brief

`GET /api/goals/brief` composes today's brief and returns:

```json
{
  "date": "2026-05-12",
  "streak": 14,
  "items": [
    { "text": "Ship Bykea report", "done": false, "source": "user" },
    { "text": "Daily review", "done": false, "source": "reminder" }
  ],
  "milestones_due_in_7_days": [
    { "title": "OSCP scheduled", "due_date": "2026-05-18", "status": "in_progress", "flagship": true }
  ],
  "active_reminder_count": 4
}
```

## User settings

| Method + path | Notes |
|---|---|
| `GET /api/me/settings` | Returns `daily_brief_time`, `evening_retro_time`, `timezone` |
| `PATCH /api/me/settings` | Update any of them |

The scheduler reads these and fires the daily brief + evening retro at the
configured times.
