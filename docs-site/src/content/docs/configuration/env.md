---
title: Environment variables
description: Every server env var Northstar reads, with defaults and notes.
---

All knobs live in `server/.env` (or process env). The example file in the repo
is the canonical reference — this page is the same content with explanations.

## Core

| Var | Default | Notes |
|---|---|---|
| `NORTHSTAR_LISTEN_ADDR` | `:8080` | Address the HTTP server binds to |
| `NORTHSTAR_DB_PATH` | `./data/northstar.db` | SQLite file location; parent dir is created on boot |
| `NORTHSTAR_LOG_LEVEL` | `info` | One of `debug` / `info` / `warn` / `error` |
| `NORTHSTAR_PAIRING_TTL` | `600` | Pairing code lifetime in seconds (10 min default) |
| `NORTHSTAR_ADMIN_TOKEN` | *(empty)* | Gates `POST /api/pair/initiate`. When empty the endpoint is open and the server logs a one-time warning at boot. Generate with `openssl rand -hex 32`. The one-shot installer sets this automatically. |

## Pillar toggles

Disabling a pillar skips both its sync worker and its REST mount.

| Var | Default | Notes |
|---|---|---|
| `NORTHSTAR_PILLAR_FINANCE` | `1` | `1` enables, `0` disables |
| `NORTHSTAR_PILLAR_GOALS` | `1` | |
| `NORTHSTAR_PILLAR_HEALTH` | `1` | |
| `NORTHSTAR_PILLAR_AI` | `1` | |

## Finance sidecar

| Var | Default | Notes |
|---|---|---|
| `NORTHSTAR_FINANCE_SIDECAR_URL` | `http://127.0.0.1:9090` | Override to `http://actual-sidecar:9090` in Docker compose |
| `NORTHSTAR_FINANCE_SIDECAR_SECRET` | *(empty)* | Optional bearer the sidecar requires; matches `SIDECAR_SHARED_SECRET` on the sidecar |
| `NORTHSTAR_FINANCE_SYNC_SECONDS` | `900` | Sync interval (15 min default) |
| `NORTHSTAR_FINANCE_SYNC_ENABLED` | `1` | Disables the sync worker but keeps REST endpoints reading the cache |

## Health sidecar

| Var | Default | Notes |
|---|---|---|
| `NORTHSTAR_HEALTH_SIDECAR_URL` | `http://127.0.0.1:9091` | |
| `NORTHSTAR_HEALTH_SIDECAR_SECRET` | *(empty)* | |
| `NORTHSTAR_HEALTH_SYNC_SECONDS` | `900` | |
| `NORTHSTAR_HEALTH_SYNC_ENABLED` | `1` | |

## AI / Claude

| Var | Default | Notes |
|---|---|---|
| `NORTHSTAR_AI_MODE` | `mock` | `mock` runs the keyword-routed engine; `anthropic` calls Claude |
| `NORTHSTAR_CLAUDE_API_KEY` | *(empty)* | Required when mode is `anthropic` |
| `NORTHSTAR_AI_MODEL` | `claude-sonnet-4-6` | Override for newer / cheaper models |

## Sidecar-internal vars

These belong to the sidecar processes, not the Go server:

### actual-sidecar

| Var | Default | Notes |
|---|---|---|
| `SIDECAR_MODE` | `mock` | `real` switches to `@actual-app/api` |
| `SIDECAR_LISTEN` | `127.0.0.1:9090` | Override in compose |
| `SIDECAR_SHARED_SECRET` | *(empty)* | Optional bearer match with the server |
| `ACTUAL_SERVER_URL` | — | Required in real mode |
| `ACTUAL_PASSWORD` | — | Required in real mode |
| `ACTUAL_SYNC_ID` | — | The 32-char budget ID from Actual settings |

### whoop-sidecar

| Var | Default | Notes |
|---|---|---|
| `SIDECAR_MODE` | `mock` | `real` switches to WHOOP v2 API |
| `SIDECAR_LISTEN` | `127.0.0.1:9091` | |
| `SIDECAR_SHARED_SECRET` | *(empty)* | |
| `WHOOP_CLIENT_ID` | — | Required in real mode |
| `WHOOP_CLIENT_SECRET` | — | Required in real mode |
| `WHOOP_REFRESH_TOKEN` | — | Cached + rotated automatically once set |

## Litestream

When you enable the `backup` profile, Litestream reads its own config from
`infra/litestream.yml`. The B2 credentials live in env there:

| Var | Notes |
|---|---|
| `LITESTREAM_ACCESS_KEY_ID` | B2 application key ID |
| `LITESTREAM_SECRET_ACCESS_KEY` | B2 application key secret |

See [Backups](/configuration/backups/).
