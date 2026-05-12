# northstar-actual-sidecar

A Node.js gateway that bridges the Go server to the Actual Budget JavaScript SDK (`@actual-app/api`). Long-lived HTTP server on `127.0.0.1:9090` by default.

The Go server is the only consumer; the sidecar is **not** internet-facing.

## Why a sidecar

`@actual-app/api` is the official, maintained Actual client. It's JavaScript-only. Rather than re-implement the Actual protocol in Go (and chase its updates forever), we run the official SDK as a sidecar and let the Go server call it over localhost JSON.

## Modes

| Mode | env | What it does |
|---|---|---|
| `mock` *(default)* | `NORTHSTAR_FINANCE_MODE=mock` | Returns canned data matching the user's category set. Lets the entire stack run end-to-end with no Actual server. |
| `actual` | `NORTHSTAR_FINANCE_MODE=actual` | Connects to a real Actual server via `/init`. Requires `@actual-app/api` installed (`npm install`). |

## Run

```bash
cd tools/actual-sidecar
# Mock mode (no install needed for the runtime — the import only happens in actual mode):
NORTHSTAR_FINANCE_MODE=mock node src/server.js

# Or via npm:
npm run mock

# Real mode (needs deps):
npm install
NORTHSTAR_FINANCE_MODE=actual node src/server.js
# … then POST to /init with serverURL/password/syncId.
```

## HTTP surface

| Method | Path | Body / Query | Returns |
|---|---|---|---|
| `GET`  | `/health`       | — | `{ ok, mode, version }` |
| `POST` | `/init`         | `{ serverURL, password, syncId, dataDir? }` | `{ ok, mode }` |
| `GET`  | `/accounts`     | — | `[{ id, name, offbudget, closed, balance }]` |
| `GET`  | `/categories`   | — | `[{ id, name, group_id, is_income }]` |
| `GET`  | `/transactions` | `?since=YYYY-MM-DD` | `[{ id, account, date, payee, category, amount, notes }]` |
| `GET`  | `/budgets`      | `?month=YYYY-MM` | `{ month, categories: [{ id, budgeted }] }` |
| `POST` | `/shutdown`     | — | `{ ok }` |

All amounts are **cents** (negative = outflow). Dates are `YYYY-MM-DD`.

## Optional shared secret

Set `SIDECAR_SHARED_SECRET=<random>` and the sidecar will require an `X-Sidecar-Secret: <random>` header on every request. The Go server sets this same env var so it's transparent. Recommended in production; not necessary on a single-user dev box.

## Why localhost-only

Bound to `127.0.0.1` by default. The sidecar holds the entire decrypted Actual budget in memory — exposing it on `0.0.0.0` would defeat Actual's E2E encryption. Override with `SIDECAR_HOST=0.0.0.0` only if you've put it behind a TLS-terminating proxy and know what you're doing.
