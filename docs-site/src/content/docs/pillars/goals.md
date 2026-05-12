---
title: Goals
description: Native milestone + daily-log + reminder system. No external sync.
---

Goals is the only pillar with no external source of truth — Northstar's SQLite
*is* the source of truth. The trade-off vs. a Notion-backed design: one less
external dependency for OSS users, no drift bugs, no two-way-sync complexity.

If you already have goal data in Notion, the `notion-importer` CLI does a
one-shot pull.

## The model

Seven native tables, all in the same SQLite file:

| Table | Holds |
|---|---|
| `goal_milestones` | Long-term milestones with status, due date, flagship flag |
| `goal_daily_log` | Per-day items + reflection + computed streak |
| `goal_weekly_tracker` | Theme + weekly goals + retro markdown, keyed by Monday |
| `goal_monthly` | Monthly theme + retro, keyed by `YYYY-MM` |
| `goal_output_log` | Shipped output (CVE / blog / talk / tool / cert / PR / report) |
| `goal_networking_log` | People + context + next action with optional due date |
| `goal_reminders` | Cron-scheduled reminders that fire as push |

## What you get

- **Flagship hero** — your one big goal, gradient indigo card on the Goals tab
- **Today's brief** — pulls open daily items + reminders firing today + yesterday's incomplete items rolled over + milestones due in the next 7 days
- **Streak counter** — consecutive days with a non-empty daily log
- **Weekly + monthly planners** — single screen with segmented scope toggle
- **Output log** — shipped work, color-coded by category
- **Networking log** — people + next actions
- **Reminders** — full cron expressions, with chip presets for common cadences

## Daily brief notification

The scheduler ticks every minute and fires a `daily_brief` push at your chosen
morning time (default 07:30 local). The brief composer:

1. Loads today's items from `goal_daily_log`.
2. Rolls over yesterday's incomplete items (tagged `source: rollover`).
3. Pulls reminders whose cron fires today (tagged `source: reminder`).
4. Surfaces milestones with `due_date <= today + 7d` and `status NOT IN ('done', 'archived')`.

You can tap each item to check it off, long-press to snooze.

## Seeding from Notion (optional, one-time)

```sh
cd tools/notion-importer
npm install
NOTION_TOKEN=secret_xxx \
NORTHSTAR_URL=http://localhost:8080 \
NORTHSTAR_TOKEN=<your-bearer> \
node src/cli.js --workspace=<workspace-id> \
                --tables=daily_log,weekly_tracker,monthly_goals,milestones,output_log,networking_log,reminders \
                --dry-run
```

The importer does loose property matching (`Status` / `State`, `Due` / `Date` / `Deadline`,
etc.) and stamps `notion:<id>` in the description so you can trace back. Drop
`--dry-run` to actually write. After cutover, archive the Notion page read-only
so muscle memory doesn't pull you back.
