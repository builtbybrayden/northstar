---
title: Upgrading
description: How to upgrade in place, and what's on the roadmap.
---

## Upgrading the server

```sh
cd ~/northstar/infra
git pull
docker compose pull
docker compose up -d
```

Migrations run automatically on boot — `goose` checks `schema_migrations` and
applies anything new. Down-migrations exist for every up but should only be run
manually from a maintenance window:

```sh
docker compose exec server northstar-server migrate down 1
```

## Upgrading the sidecars

```sh
cd ~/northstar/infra
docker compose pull actual-sidecar whoop-sidecar
docker compose up -d actual-sidecar whoop-sidecar
```

The sidecars are stateless — pulling a new image and restarting is the entire
upgrade.

## Upgrading the iOS app

For now the app is built from source. After `git pull` in `ios/`:

```sh
cd ios
xcodegen generate
open Northstar.xcodeproj
# Hit ▶
```

Once the TestFlight pipeline is live, upgrades will come over the air.

## Roadmap

| Phase | Status |
|---|---|
| 0 — Foundation | ✅ shipped |
| 1 — Finance ingest | ✅ shipped |
| 2 — Budget alerts + push (log sender only — APNs deferred) | ✅ shipped |
| 3 — Goals pillar | ✅ shipped |
| 3.5 — iOS planner / log / reminders UIs | ✅ shipped |
| 4 — Health pillar | ✅ shipped |
| 5 — Ask Claude | ✅ shipped |
| 6 — Docs + installer + launch prep | ✅ shipped |
| 7 — Apple Dev Program + TestFlight + App Store | deferred (needs $99/yr enrollment) |
| 7 — HealthKit ingest + Apple Watch glance | deferred |
| 7 — Android (Compose) port | deferred — community-led |
| 7 — Multi-user / family accounts | deferred |
| 7 — Receipt OCR / investment portfolio view | deferred |

Public GitHub Projects board: [`brayden/northstar` → Projects](https://github.com/brayden/northstar/projects).
