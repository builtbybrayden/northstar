---
title: Finance
description: Mirror your Actual Budget data into the Northstar dashboard with threshold + anomaly notifications.
---

The Finance pillar mirrors your existing
[Actual Budget](https://actualbudget.org/) instance into Northstar's local
SQLite. Actual stays the source of truth — user edits route back through the
Actual API so your existing automation (rules, recategorizations, reports)
keeps working.

## What you get

- **Spend rings** — current-month Spent / Saved / Income vs. targets
- **Per-category bars** with color thresholds (green / yellow / red)
- **Net-worth card** split on-budget vs. off-budget
- **Recent transactions list** with category-emoji glyphs
- **Push notifications** on every transaction, every 50/75/90/100% threshold,
  and every anomaly (3× median for known payee, or new merchant ≥ $200)
- **Subscription audit** — detected recurring charges

## How it works

```
Actual Budget instance
         │ @actual-app/api  (Node-only — that's why the sidecar exists)
         ▼
   actual-sidecar       (Node, mock-by-default)
         │ HTTP
         ▼
   northstar-server     Go sync worker, every 15 min
         │ UPSERT
         ▼
   SQLite  (fin_accounts / fin_transactions / fin_budget_targets / …)
```

## Mock mode (default)

The sidecar ships with realistic sample data:

- 11 accounts including checking, HYSA, credit cards, mortgage, and off-budget assets
- 31 transactions across the current and prior month
- 9 budgeted categories with realistic spend
- 1 category (Mortgage) intentionally at 99% so threshold UI is visible

This lets the entire stack run with zero external dependencies — useful for
trying Northstar before you decide to wire up Actual.

## Real mode

Flip `NORTHSTAR_FINANCE_MODE=actual` on the sidecar and add:

```ini
ACTUAL_SERVER_URL=https://actual.your-tailnet.ts.net
ACTUAL_PASSWORD=…
ACTUAL_SYNC_ID=<32-char-budget-id>
```

The sidecar lazy-loads `@actual-app/api` on first request and keeps a session
warm. See [Sidecars](/configuration/sidecars/) for the full env list.

## Editing

| Surface | Edit |
|---|---|
| Tap a category bar | Bottom sheet: cap, threshold pcts (50/75/90/100), push toggle |
| Swipe a transaction | Recategorize → writes back through Actual |
| Settings → Notifications | Per-category quiet hours + daily caps |

All edits write through the Actual API first, then update the local cache.
