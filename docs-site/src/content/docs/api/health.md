---
title: Health API
description: REST surface for recovery / sleep / strain, supplements, dose log, and mood.
---

All endpoints require `Authorization: Bearer <token>`.

## `GET /api/health/today`

Returns the composed payload for the Health tab.

```json
{
  "date": "2026-05-12",
  "recovery": { "score": 84, "hrv_ms": 78, "rhr": 51 },
  "sleep": { "duration_min": 462, "score": 88, "debt_min": 0 },
  "strain": { "score": 16.0, "avg_hr": 112, "max_hr": 168 },
  "verdict": "push",
  "strain_goal": { "low": 16.0, "high": 18.5 }
}
```

`verdict` ∈ {`push` (≥ 70), `maintain` (≥ 40), `recover` (< 40)}.

## `GET /api/health/recovery?days=N`

`GET /api/health/sleep?days=N`

`GET /api/health/strain?days=N`

Last N rows (default 14, max 90). Each row matches its underlying table shape
in the [data model](/contributing/layout/).

## Supplements / peptides / medications

These share one definition table — `category` field discriminates.

| Method + path | Notes |
|---|---|
| `GET /api/health/supplements/defs?active=1` | All definitions |
| `POST /api/health/supplements/defs` | Create |
| `PATCH /api/health/supplements/defs/:id` | Edit |
| `DELETE /api/health/supplements/defs/:id` | Soft delete (active = 0) |
| `POST /api/health/supplements/log` | Log a dose `{def_id, taken_at?, notes?}` |
| `GET /api/health/supplements/log?days=N` | Last N days of doses |

Definition body:

```json
{
  "name": "BPC-157",
  "dose": "500 mcg",
  "category": "peptide",
  "schedule_json": "{...}",
  "cycle_days_on": 4,
  "cycle_days_off": 2,
  "reminder_enabled": true,
  "prescribing_doc": "Dr. Foo",
  "notes": "Subcutaneous, AM"
}
```

## Mood / energy / focus

```
PUT /api/health/mood/:date
```

Body: `{ mood: 1-5, energy: 1-5, focus: 1-5, notes: string }`.
