---
title: Sidecars (Actual + WHOOP)
description: How the two Node sidecars front third-party APIs and why they're a separate process.
---

The Northstar server is Go, but two of the upstream sources (Actual Budget,
WHOOP) only ship clients in JavaScript. Rather than shell out to Node from Go
or rewrite their auth flows, each source has a thin Node sidecar that exposes a
clean HTTP surface the Go server polls.

## Why a sidecar at all?

- **Actual's API is Node-only.** `@actual-app/api` is the official client and there's no clean Go alternative.
- **WHOOP's OAuth2 refresh dance** is annoying to keep in Go; the sidecar holds the in-process refresh-token cache.
- **Restart isolation** — if Actual's API decides to change a field, only the sidecar needs a fix, not the whole Go server.
- **Mock mode is cheap** — each sidecar can fake an entire upstream so the rest of the stack runs without credentials.

## Bound to localhost by default

Both sidecars bind to `127.0.0.1` by default. In Docker compose they sit on an
internal network and the Go server talks to them by service name
(`actual-sidecar` / `whoop-sidecar`). They should never be exposed to the
internet.

## Shared secret (optional)

If you want a defense-in-depth check inside the compose network, set
`SIDECAR_SHARED_SECRET` on the sidecar and `NORTHSTAR_FINANCE_SIDECAR_SECRET`
(or `_HEALTH_`) on the server to the same value. The sidecar checks
`Authorization: Bearer <secret>` and 401s mismatches.

## actual-sidecar

### Endpoints

| Method + path | Returns |
|---|---|
| `GET /health` | `{ok: true}` |
| `POST /init` | Loads the budget if real mode (idempotent) |
| `GET /accounts` | All accounts including balance + on/off-budget |
| `GET /categories` | All categories |
| `GET /transactions?since=YYYY-MM-DD` | Transactions, defaults to last 90 days |
| `GET /budgets?month=YYYY-MM` | Monthly category budgets |
| `POST /shutdown` | Graceful shutdown (used by tests) |

### Mock data shape

The mock returns 11 accounts and 31 transactions across the current + prior
month with realistic distributions. The Mortgage category is intentionally at
~99% of its budget so the threshold UI fires.

### Flipping to real

```ini
NORTHSTAR_FINANCE_MODE=actual
ACTUAL_SERVER_URL=https://actual.your-tailnet.ts.net
ACTUAL_PASSWORD=…
ACTUAL_SYNC_ID=<32-char-budget-id>
```

The sidecar lazy-loads `@actual-app/api` on the first request and keeps a
session warm. Once you switch modes, the sidecar's `/init` endpoint takes the
serverURL / password / syncId; the Go server calls it on boot.

## whoop-sidecar

### Endpoints

| Method + path | Returns |
|---|---|
| `GET /health` | `{ok: true}` |
| `GET /recovery?days=N` | Last N days of recovery scores + HRV + RHR |
| `GET /sleep?days=N` | Sleep duration + score + debt per day |
| `GET /strain?days=N` | Strain + avg/max HR per day |
| `GET /profile` | The connected WHOOP user's profile |

### Mock data shape

14-day rolling window with today at 84% recovery (verdict push), a 28%
recovery dip three days back (for the recovery-drop detector path), and all
metrics internally correlated.

### Flipping to real

```ini
NORTHSTAR_HEALTH_MODE=real
WHOOP_CLIENT_ID=…
WHOOP_CLIENT_SECRET=…
WHOOP_REFRESH_TOKEN=…
```

The sidecar uses WHOOP v2 OAuth2. Refresh tokens are rotated automatically and
cached in-process; if the sidecar restarts, you'll need to seed the env again
with the most recent refresh token.

> **WHOOP API access** requires a developer account and approval. Apply early.
