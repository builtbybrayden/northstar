---
title: What is Northstar?
description: A one-page tour of the architecture, pillars, and design principles.
---

## In one sentence

Northstar is a self-hosted iOS app that ingests your finance data from Actual
Budget, your biometrics from WHOOP, manages your career goals natively, and lets
you ask Claude questions against the combined data.

## The architecture

```
┌──────────────────────────────────────────────────────────────┐
│  iOS app   (SwiftUI, iOS 17+)                                │
│  Home / Finance / Goals / Health / Ask                       │
└──────────────────────────────────────────────────────────────┘
                              │  HTTPS (Tailscale by default)
                              ▼
┌──────────────────────────────────────────────────────────────┐
│  northstar-server  (Go single binary, Docker compose)        │
│                                                              │
│  • REST + SSE API   (chi)                                    │
│  • Sync workers     (Actual / WHOOP every 15 min)            │
│  • Notification engine (rules → APNs)                        │
│  • Claude bridge    (tool use + prompt caching)              │
│  • Cron scheduler   (daily brief / reminders)                │
│                                                              │
│  SQLite (one file) + Litestream → B2                         │
└──────────────────────────────────────────────────────────────┘
        │                            │                  │
        ▼                            ▼                  ▼
   Actual sidecar              WHOOP sidecar         Claude API
   (local Node)                (local Node)          (cloud, your key)
```

The server is a single Go binary. The two sidecars are local Node processes that
gateway third-party APIs (Actual JavaScript-only API, WHOOP OAuth) into a clean
HTTP surface the Go server polls. Both sidecars ship with a mock mode so you can
run the whole stack with zero credentials.

## Design principles

| | |
|---|---|
| **Self-host by default** | Single Docker compose. No mandatory cloud. |
| **Own your data** | SQLite on your box. Litestream replicates to your B2 bucket. |
| **Mobile-first** | The phone is the primary surface. Web admin exists only for setup. |
| **Privacy-first** | No analytics. No telemetry. No third-party SDKs in the iOS app. |
| **Claude is opt-in** | Bring your own key. Mock mode is the default. |
| **Pillars are modular** | Run finance-only, or health-only — each pillar can be disabled. |

## The three pillars + chat

- **Finance** mirrors Actual. Writes route back through Actual's API so your existing automation keeps working.
- **Goals** is native (no external dependency). One-time CLI seeds from a Notion workspace if you have one.
- **Health** mirrors WHOOP. Manual supplement / peptide / medication logger sits on top.
- **Ask Claude** is a chat tab with tool-use access to all three pillars.

## What's next

- [Quickstart →](/getting-started/quickstart/)
- [Pair your iPhone →](/getting-started/pairing/)
