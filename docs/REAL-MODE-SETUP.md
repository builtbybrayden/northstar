# Real-mode smoke test setup

How to point Northstar at your existing Actual Budget server (and the rest
of the credential-gated integrations) instead of the baked-in mock data.

## Actual Budget (real transactions from your bank accounts)

### Prereqs you already have

From `~/projects/finance/compose/.env`:

- `ACTUAL_SERVER_URL=http://localhost:5006`
- `ACTUAL_SERVER_PASSWORD=…`
- `ACTUAL_SYNC_ID=9d1bafa5-…`
- `ACTUAL_ENCRYPTION_PASSWORD=` (empty — your budget isn't e2e encrypted)

Your finance project repo is Actual + SimpleFIN running in Docker on port
5006 with bank accounts already wired in.

### Tell Northstar to use it

Edit `~/projects/northstar/server/.env` (gitignored, never committed). Add:

```ini
NORTHSTAR_FINANCE_MODE=actual

# `localhost` from inside the Northstar Docker container = the container
# itself. host.docker.internal routes to the host machine, where your
# Actual server actually listens. On Docker Desktop (Windows/Mac) this
# hostname is built-in; on Linux infra/docker-compose.yml maps it for you.
NORTHSTAR_ACTUAL_SERVER_URL=http://host.docker.internal:5006
NORTHSTAR_ACTUAL_PASSWORD=y9XKKSLJ7M9D64TV*
NORTHSTAR_ACTUAL_SYNC_ID=9d1bafa5-2c02-4a64-8c00-d45f33564528
NORTHSTAR_ACTUAL_ENCRYPTION_PASSWORD=
```

Also create `~/projects/northstar/infra/.env` so docker compose substitution
picks up the mode for the sidecar:

```ini
NORTHSTAR_FINANCE_MODE=actual
```

### Run it

```powershell
cd ~\projects\northstar\infra
docker compose down                # if the mock-mode stack is running
docker compose up -d --build       # rebuild so the sidecar picks up real-mode deps
docker compose logs -f server      # watch for "sidecar initialized against Actual server"
```

### What good looks like

In the server logs (`docker compose logs -f server`):

```
finance: sidecar initialized against Actual server http://host.docker.internal:5006
finance.sync: starting (interval=15m0s, detector=true)
finance.sync: synced 12 accounts (216 new txns / 0 updated)
```

In a separate shell, verify the data is in the Northstar DB:

```powershell
curl http://localhost:8080/api/finance/summary
curl http://localhost:8080/api/finance/transactions?limit=10
curl http://localhost:8080/api/finance/investments
curl http://localhost:8080/api/finance/forecast?days=30
```

The numbers should match what Actual's reports show. If summary's
`net_worth_cents` doesn't match your Actual net worth within $1 (rounding),
something is wrong — open an issue with the diff.

### When it doesn't work

- `sidecar /init: status 500` → the sidecar can't reach
  `http://host.docker.internal:5006`. Verify Actual is running
  (`docker ps | grep actual-server`) and that 5006 is published.
- `sidecar /init: cannot find module '@actual-app/api'` → rebuild the
  sidecar image: `docker compose build sidecar`. The dep is in
  `tools/actual-sidecar/package.json` but only installed when the image
  is rebuilt.
- `finance.sync: tick failed: sidecar /transactions: status 500` →
  `docker compose logs sidecar` will show the underlying Actual error.
  Usually a stale download — restart the sidecar:
  `docker compose restart sidecar`.
- Transactions appear but categories are blank → Actual returns category
  *IDs*, not names; Northstar maps them via the categories endpoint. If
  you've deleted categories upstream, the IDs in old transactions will be
  unmappable. Recategorize in Actual (or use Northstar's per-transaction
  override sheet to fix individual rows locally).

### Switching back to mock

Either unset `NORTHSTAR_FINANCE_MODE` in both `.env` files (defaults to mock)
or set it explicitly to `mock`, then `docker compose up -d`.

## WHOOP real mode

### Once you have the credentials

After WHOOP developer registration completes (the user is pending approval)
you should have:

- A **client ID** + **client secret** from `developer.whoop.com` → My Apps
- A **redirect URI** registered on that same page

Add to `server/.env`:

```ini
WHOOP_CLIENT_ID=…
WHOOP_CLIENT_SECRET=…
WHOOP_REDIRECT_URI=http://localhost:8080/api/health/whoop/oauth/callback
```

Sidecar mode also needs to flip; in `infra/.env`:

```ini
NORTHSTAR_HEALTH_MODE=real
```

### One-time OAuth flow

The WHOOP sidecar already has `/oauth/start` and `/oauth/callback` wired
(commit `d640267`). Open in your browser:

```
http://localhost:8080/api/health/whoop/oauth/start
```

It bounces you to WHOOP for consent, then back. After that the sidecar
holds a refresh token and the health sync starts populating data.

### What good looks like

- `curl http://localhost:8080/api/health/today` returns your real recovery
  score, not the baked-in 84.
- The supplements + mood pillars keep working unchanged — those have no
  WHOOP dependency.

## Apple Developer Program

Not a code-only task. Once you enroll at
[developer.apple.com/programs/](https://developer.apple.com/programs/) ($99/yr),
let me know and I'll wire:

- TestFlight pipeline in `.github/workflows/ios.yml`
- APNs production sender (`sideshow/apns2`) — needs the `.p8` key file
- Widget extension target + Live Activity target in `Project.yml`
- Watch app target

Each is its own milestone; we can do them one at a time.

## What I still need from you

| Item | Status | What I need |
|---|---|---|
| Actual real mode | **Ready to run** — config is wired, just paste the four `NORTHSTAR_ACTUAL_*` values into `server/.env` and `NORTHSTAR_FINANCE_MODE=actual` into both `.env` files, then `docker compose up -d --build` |
| WHOOP real mode | Pending approval | Client ID + client secret from `developer.whoop.com` once approved |
| Apple Watch / Widget / Live Activity / TestFlight / APNs | Blocked | Apple Developer Program enrollment ($99/yr) |
| Claude API | Skipped | Not needed — we use the local `claude` CLI |
