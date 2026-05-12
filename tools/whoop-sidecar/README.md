# northstar-whoop-sidecar

HTTP gateway that bridges the Go server to the WHOOP v2 REST API. Mock-by-default so dev works without a real WHOOP account.

## Run

```bash
cd tools/whoop-sidecar
# Mock mode (default — no deps to install)
node src/server.js

# Real mode (needs OAuth2 credentials)
export WHOOP_CLIENT_ID=...
export WHOOP_CLIENT_SECRET=...
export WHOOP_REFRESH_TOKEN=...
NORTHSTAR_HEALTH_MODE=whoop node src/server.js
```

## HTTP surface

| Method | Path | Query | Returns |
|---|---|---|---|
| `GET` | `/health`    | — | `{ ok, mode, version }` |
| `GET` | `/recovery`  | `?days=N` (default 14) | `[{ date, score, hrv_ms, rhr }]` |
| `GET` | `/sleep`     | `?days=N` | `[{ date, duration_min, score, debt_min }]` |
| `GET` | `/strain`    | `?days=N` | `[{ date, score, avg_hr, max_hr }]` |
| `GET` | `/profile`   | — | `{ user_id, first_name, height_m, weight_kg }` |

Bound to `127.0.0.1:9091` by default. Holds no secrets in mock mode.

## Mock data

14-day rolling window. Today is anchored at 84% recovery (matches the mockup). Day 3 dips into 28% red so the recovery-drop detector has something to flag.
