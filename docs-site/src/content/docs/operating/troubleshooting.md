---
title: Troubleshooting
description: Common problems and how to triage them.
---

## "Pairing code is invalid"

- Codes expire after `NORTHSTAR_PAIRING_TTL` (default 600s). Mint a fresh one with `POST /api/pair/initiate`.
- Codes are single-use — burning happens on `POST /api/pair/redeem`. If you mistyped, mint another.
- Confirm the iOS app is hitting the right URL: `http://<server-ip>:8080`, not `localhost`.

## "Sync isn't pulling data"

```sh
docker compose logs server --tail=50 | grep -E "(sync|finance|health)"
docker compose logs actual-sidecar --tail=30
docker compose logs whoop-sidecar --tail=30
```

Common causes:

- Sidecar in mock mode by accident — check `SIDECAR_MODE` env on the sidecar container.
- Real-mode credentials missing/wrong — sidecar logs the auth error.
- Sync worker disabled — `NORTHSTAR_FINANCE_SYNC_ENABLED=0` or `_HEALTH_SYNC_ENABLED=0`.
- Pillar disabled — `NORTHSTAR_PILLAR_FINANCE=0` skips both sync and REST.

Force a single sync run:

```sh
curl -X POST http://localhost:8080/api/finance/sync \
  -H "Authorization: Bearer $TOKEN"
```

## "Notifications aren't appearing on my phone"

V1 ships with `LogSender` — notifications land in the server log, not in APNs.
You'll see lines like:

```
notify  composed       category=budget_threshold title="Restaurants at 105%"
```

Once Apple Developer Program is wired in Phase 7, `APNSSender` will replace
this. For now, the in-app `Notifications` tab (`GET /api/notifications/feed`)
is the surface.

## "Claude calls fail"

Check:

```sh
echo $NORTHSTAR_AI_MODE          # should be 'anthropic' for real mode
echo ${NORTHSTAR_CLAUDE_API_KEY:0:6}…
```

If the env is set correctly but calls still fail:

- The server emits `error` SSE events with Anthropic's message verbatim — pipe them through the iOS app or curl directly.
- Rate-limit errors (`429`) flow through unchanged; back off.
- Tool dispatch errors (e.g. unknown tool name) come back as `tool_result` blocks with `is_error: true`; the conversation continues.

To fall back to mock mode without losing the conversation:

```sh
NORTHSTAR_AI_MODE=mock docker compose up -d server
```

## "SQLite locked / busy"

The server runs SQLite in WAL mode so reads + writes can coexist. If you see
`SQLITE_BUSY`:

- Confirm no other process is holding `northstar.db` open (Litestream is fine, it uses a snapshot).
- Check disk space — full disks can stall the WAL checkpoint.
- Worst case: stop the server, run `sqlite3 northstar.db 'PRAGMA wal_checkpoint(TRUNCATE);'`, restart.

## Resetting

Wipes data, keeps config:

```sh
docker compose down
docker volume rm northstar_data
docker compose up -d
```

Wipes everything:

```sh
docker compose down -v
rm -rf ~/.northstar/
```
