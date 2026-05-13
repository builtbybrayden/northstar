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

## Real mode — Anthropic API

```ini
NORTHSTAR_AI_MODE=anthropic
NORTHSTAR_CLAUDE_API_KEY=sk-ant-…
NORTHSTAR_AI_MODEL=claude-sonnet-4-6
```

Claude streams over SSE; each event is one of `text`, `tool_call`, `done`, or
`error`. The iOS app renders tool-call markers inline so you can see which tool
fired before the text arrived. Token usage (input / output / cache-read /
cache-creation) is persisted on every assistant message.

## Real mode — local Claude CLI

If you'd rather not give Northstar an Anthropic API key, point it at a locally
installed [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) instead:

```ini
NORTHSTAR_AI_MODE=cli
NORTHSTAR_CLI_BIN=claude               # exec path; only used when the
                                       # server runs outside Docker
NORTHSTAR_AI_MODEL=claude-sonnet-4-6   # optional --model passthrough
# When the server runs in Docker, exec can't see the host's PATH. Point
# at the host-side bridge instead:
NORTHSTAR_CLI_BRIDGE_URL=http://host.docker.internal:9092
NORTHSTAR_CLI_BRIDGE_SECRET=           # optional shared secret
```

The bridge is a tiny Node script at `tools/claude-cli-bridge/` that you run
on the host (`node src/server.js`). It accepts `POST /prompt` and shells out
to `claude -p` locally. The dockerized server reaches it via
`host.docker.internal:9092`. Without the bridge, set
`NORTHSTAR_CLI_BRIDGE_URL=` empty and the server will try to `exec claude`
directly — which works when the server itself runs on the host.

Trade-offs vs the API mode:

- No live tool-use loop. Before invoking the CLI, the server pre-fetches a
  snapshot of each pillar (the same data the API-mode tools would return) and
  inlines it into the system prompt. The model sees a frozen view of pillar
  state at request time.
- Reply streaming is faked client-side — the CLI returns the full reply at
  once, the server chunks it before forwarding. The iOS surface is identical.
- Token usage telemetry is unavailable on this path; `usage_json` is NULL for
  CLI-mode messages.

This is the right mode for a single-operator setup where you've already paid
for Claude Code and don't want a second API bill.

## Safety rail

The cached system block ends with:

> Not medical advice — confirm with your prescriber before changing anything.

The rail is part of the cached system prompt so it's always in scope and free
to include.
