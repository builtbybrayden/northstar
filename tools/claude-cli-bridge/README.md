# northstar-claude-cli-bridge

Tiny HTTP shim around the local `claude` CLI so the dockerized Northstar
server can use `NORTHSTAR_AI_MODE=cli` without an Anthropic API key.

## Why

The Go server runs in a distroless Docker container; the `claude` CLI
lives on your host. A container can't reach the host's PATH, so an in-process
`exec.Command("claude", ...)` from inside the container fails with `not
found in $PATH`. This bridge runs ON THE HOST, shells out to `claude` there,
and exposes a small HTTP surface the dockerized server can call via
`host.docker.internal`.

## Run

```bash
cd tools/claude-cli-bridge
node src/server.js
# [claude-bridge ...] listening on http://127.0.0.1:9092 · claude_bin=claude · auth=open
```

Defaults:

| env | default | notes |
|---|---|---|
| `BRIDGE_HOST` | `127.0.0.1` | localhost only — Docker reaches it via host.docker.internal |
| `BRIDGE_PORT` | `9092` | |
| `BRIDGE_SHARED_SECRET` | _(empty)_ | when set, server must send `X-Bridge-Secret` |
| `BRIDGE_CLAUDE_BIN` | `claude` | path to the binary; override if it isn't on PATH |
| `BRIDGE_TIMEOUT_MS` | `120000` | per-request cap |

## HTTP

| Method | Path | Body | Returns |
|---|---|---|---|
| `GET` | `/health` | — | `{ok, mode, service, version}` |
| `POST` | `/prompt` | `{question, system?, model?}` | `{reply}` on 200; `{error}` on 4xx/5xx |

`question` is piped to `claude` on stdin so newlines, quotes, and `$`
characters are safe. `system` and `model` map to the CLI's
`--system-prompt` / `--model` flags.

## Wire-up from the Go server

In `server/.env`:

```ini
NORTHSTAR_AI_MODE=cli
NORTHSTAR_CLI_BRIDGE_URL=http://host.docker.internal:9092
# NORTHSTAR_CLI_BRIDGE_SECRET=...  # only if you set BRIDGE_SHARED_SECRET on the host
```

When `NORTHSTAR_CLI_BRIDGE_URL` is empty, the server falls back to direct
`exec` of `NORTHSTAR_CLI_BIN` (the right mode when the server runs on the
host, not in Docker).

## Keep it running

This isn't auto-started by `docker compose up` — it lives on the host. A
few options for keeping it alive across reboots:

- **macOS launchd**: `~/Library/LaunchAgents/com.northstar.claude-bridge.plist`
- **Linux systemd --user**: unit file in `~/.config/systemd/user/`
- **Windows Task Scheduler**: see `scripts/register-tasks.ps1` in the finance project for the pattern; run at logon

## Security

This bridge effectively grants whoever can reach it the ability to run
`claude` on your machine with your subscription. Two layers keep that safe:

1. Bind to `127.0.0.1`, not `0.0.0.0`. Only processes on the same host
   (including Docker via host-gateway) can reach it.
2. Set `BRIDGE_SHARED_SECRET` if you want belt-and-suspenders auth.

The request body is capped at 1 MiB and the spawned process is killed if
it exceeds `BRIDGE_TIMEOUT_MS`.
