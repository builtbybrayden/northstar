---
title: Finance API
description: REST surface for accounts, transactions, summary, and budget targets.
---

All endpoints require `Authorization: Bearer <token>`.

## `GET /api/finance/accounts`

```json
[
  {
    "actual_id": "a1",
    "name": "Chase Checking",
    "type": "checking",
    "balance_cents": 482312,
    "on_budget": true,
    "closed": false,
    "updated_at": 1747000000
  }
]
```

## `GET /api/finance/transactions`

Query params:

| Param | Notes |
|---|---|
| `limit` | Default 50, max 500 |
| `since` | `YYYY-MM-DD` inclusive lower bound |
| `until` | `YYYY-MM-DD` inclusive upper bound |
| `category` | Exact match |
| `account_id` | Exact match |

```json
[
  {
    "actual_id": "t1",
    "account_id": "a1",
    "date": "2026-05-12",
    "payee": "Starbucks #4",
    "category": "Restaurants",
    "amount_cents": -745,
    "notes": ""
  }
]
```

## `GET /api/finance/summary?month=YYYY-MM`

Returns the data driving the Finance tab's hero + rings + bars. `month`
defaults to the current month.

```json
{
  "month": "2026-05",
  "net_worth_cents": 12345678,
  "on_budget_cents": 482312,
  "off_budget_cents": 11863366,
  "income_cents": 612345,
  "spent_cents": 234567,
  "categories": [
    {
      "category": "Restaurants",
      "spent_cents": 51200,
      "budgeted_cents": 50000,
      "pct": 102,
      "over": true
    }
  ]
}
```

## `GET /api/finance/budget-targets`

Lists all budget targets.

```json
[
  {
    "category": "Restaurants",
    "monthly_cents": 50000,
    "rationale": "BLS for $70-99k Atlanta",
    "threshold_pcts": [50, 75, 90, 100],
    "push_enabled": true,
    "updated_at": 1747000000
  }
]
```

## `PATCH /api/finance/budget-targets/:category`

Mutate any combination of the editable fields. UPSERT on first call.

```json
// request
{
  "monthly_cents": 60000,
  "threshold_pcts": [75, 90, 100],
  "push_enabled": true
}
```
