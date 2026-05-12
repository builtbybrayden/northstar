---
title: Ask Claude
description: A chat tab with tool-use access to all three pillars, prompt-cached system block, mock-mode default.
---

The Ask pillar gives Claude tool-use access to all three pillars: it can query
your finance, goals, and health data in well-bounded calls, then write a
grounded answer.

## What you get

- **10 tools** spanning the three pillars (see below)
- **SSE-streamed responses** to the iOS chat UI
- **Server-side conversation history** — never on Anthropic's servers
- **Mock mode default** so you can run the full pipeline without a key
- **Prompt caching** — the system block + tool definitions sit in Anthropic's
  ephemeral cache, so multi-turn chat is cheap

## Tools

| Name | Pillar | What it returns |
|---|---|---|
| `finance_summary` | Finance | Current-month net worth, income, spent, per-category spend vs. budget |
| `finance_search_transactions` | Finance | Up to 50 transactions matching payee / category / date filters |
| `finance_category_history` | Finance | 12 months of spend totals for one category |
| `finance_subscriptions` | Finance | Detected recurring charges (same payee × similar amount, ≥ 2 months) |
| `goals_brief` | Goals | Today's open items + active reminder count + milestones due in 7 days + streak |
| `goals_milestones` | Goals | Full milestone list with optional `status_in` filter |
| `goals_recent_output` | Goals | Recent entries from the output log within the last N days |
| `health_today` | Health | Today's recovery, HRV, RHR, sleep, strain, computed verdict |
| `health_recovery_history` | Health | Last N days of recovery + HRV + RHR |
| `health_supplements` | Health | Active supplement defs + dose counts in the last N days |

## Sample questions

- *"Should I push hard today?"* → `health_today` → verdict-grounded recommendation
- *"How much have I spent at Starbucks this month?"* → `finance_search_transactions`
- *"What subscriptions am I paying for?"* → `finance_subscriptions`
- *"Am I on track for OSCP?"* → `goals_milestones` + `goals_recent_output`
- *"Did my sleep get worse this week?"* → `health_recovery_history`

## Mock mode

The mock engine routes by keyword:

| Trigger word | Tool fired |
|---|---|
| push / recover / today | `health_today` |
| budget / spend / spent | `finance_summary` |
| goal / milestone / streak | `goals_brief` |

The actual SQLite data flows through, so the dashboard's numbers match the
chat's numbers even without an Anthropic key.

## Real mode

```ini
NORTHSTAR_AI_MODE=anthropic
NORTHSTAR_CLAUDE_API_KEY=sk-ant-…
NORTHSTAR_AI_MODEL=claude-sonnet-4-6
```

Claude streams over SSE; each event is one of `text`, `tool_call`, `done`, or
`error`. The iOS app renders tool-call markers inline so you can see which tool
fired before the text arrived.

## Safety rail

The cached system block ends with:

> Not medical advice — confirm with your prescriber before changing anything.

The rail is part of the cached system prompt so it's always in scope and free
to include.
