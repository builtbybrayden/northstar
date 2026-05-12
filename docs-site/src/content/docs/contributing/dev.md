---
title: Dev environment
description: Set up a local dev loop for the server, the sidecars, and the iOS app.
---

## Prerequisites

- Go 1.22+
- Node.js 20+
- Docker (for compose-based testing + sidecars in mock mode)
- Optional: Xcode 15+ on a Mac for iOS development
- Optional: `xcodegen` (`brew install xcodegen`)

## Server

```sh
cd server
cp .env.example .env
go run ./cmd/northstar-server
```

The server applies migrations on boot and starts listening on `:8080`. Health
check:

```sh
curl http://localhost:8080/api/health
```

### Tests

```sh
cd server
go test ./...
```

Five packages have tests:

- `internal/ai`         — tool dispatch + cache_control invariants
- `internal/finance`    — threshold parser + dollar formatter + transition copy
- `internal/goals`      — cron / streak / brief composer / mondayOf
- `internal/health`     — recovery-drop detector (fire / no-fire / dedup)
- `internal/notify`     — composer dedup / quiet hours / critical bypass / daily cap

### Running with sidecars

Easiest path is compose:

```sh
cd infra
docker compose up -d
```

To iterate on the server while sidecars run in compose:

```sh
docker compose up -d actual-sidecar whoop-sidecar
NORTHSTAR_FINANCE_SIDECAR_URL=http://localhost:9090 \
NORTHSTAR_HEALTH_SIDECAR_URL=http://localhost:9091 \
go run ./cmd/northstar-server
```

## Sidecars

```sh
cd tools/actual-sidecar
npm install
npm start   # SIDECAR_MODE=mock by default

cd tools/whoop-sidecar
npm install
npm start
```

Both are ESM Node services that hot-reload only on restart — they're small
enough that this isn't a hot loop.

## iOS app

```sh
cd ios
xcodegen generate
open Northstar.xcodeproj
```

Set the team under Signing & Capabilities. The app uses no APNs entitlement
in dev — push registration silently no-ops without one.

For Windows hosts, see [docs/WINDOWS-IOS-DEV.md](https://github.com/brayden/northstar/blob/main/docs/WINDOWS-IOS-DEV.md)
for a workflow that uses GitHub Actions macOS runners as the build cluster.

## End-to-end smoke

The README's "Quick start" walks through the full flow: boot compose, mint a
pairing code, scan from the app, see live (mock) data on Home. Worth running
once on any non-trivial change.
