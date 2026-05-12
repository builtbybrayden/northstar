---
title: Quickstart (Docker)
description: Boot the whole Northstar stack in under five minutes with mock data — no credentials required.
---

This walkthrough boots `northstar-server` plus the two sidecars in mock mode, so
you'll have a working dashboard with realistic sample data before connecting any
real services.

## Prerequisites

- Docker + Docker Compose (Docker Desktop on macOS/Windows, or the docker-ce
  packages on Linux)
- ~200 MB free disk for the images
- An iPhone running iOS 17+ (only needed once you want to look at the dashboard)

## 1. Clone the repo

```sh
git clone https://github.com/brayden/northstar.git
cd northstar
```

## 2. Copy the example env

```sh
cp server/.env.example server/.env
```

The defaults are enough to run the whole stack in mock mode:

```ini
NORTHSTAR_LISTEN_ADDR=:8080
NORTHSTAR_DB_PATH=/data/northstar.db
NORTHSTAR_AI_MODE=mock
# Sidecars default to localhost — compose overrides to service names.
NORTHSTAR_FINANCE_SIDECAR_URL=http://actual-sidecar:9090
NORTHSTAR_HEALTH_SIDECAR_URL=http://whoop-sidecar:9091
```

See [Environment variables](/configuration/env/) for the full list.

## 3. Boot the stack

```sh
cd infra
docker compose up -d
```

This brings up three services:

| Service | Port | What it does |
|---|---|---|
| `actual-sidecar` | `9090` | Mock or real Actual Budget gateway |
| `whoop-sidecar` | `9091` | Mock or real WHOOP API gateway |
| `server` | `8080` | The Go API server + sync workers + scheduler |

Healthcheck:

```sh
curl http://localhost:8080/api/health
# {"ok": true}
```

## 4. Mint a pairing code

```sh
curl -X POST http://localhost:8080/api/pair/initiate -d '{}'
# {"code": "479320", "expires_at": ...}
```

Codes are valid for 10 minutes. See [Pair your iPhone](/getting-started/pairing/)
for the iOS side.

## 5. (Optional) Backups

Litestream is wired as a separate compose profile so it doesn't run unless you
ask:

```sh
docker compose --profile backup up -d litestream
```

You'll need a B2 bucket and a key pair — see [Backups](/configuration/backups/).

## 6. (Optional) Flip on real data sources

When you're ready to swap mock for real:

- **Actual:** set `NORTHSTAR_FINANCE_SIDECAR_MODE=real` plus the Actual server URL and password.
- **WHOOP:** set `NORTHSTAR_HEALTH_SIDECAR_MODE=real` plus a WHOOP client ID / refresh token.
- **Claude:** set `NORTHSTAR_AI_MODE=anthropic` plus `NORTHSTAR_CLAUDE_API_KEY=sk-ant-…`.

See [Sidecars](/configuration/sidecars/).
