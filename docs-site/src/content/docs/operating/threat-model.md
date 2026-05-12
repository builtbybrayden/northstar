---
title: Threat model
description: What Northstar protects against, what it explicitly doesn't, and where the user is on the hook.
---

Northstar is a personal-use, self-hosted application. The threat model is
written for that context — single user, single trust boundary, no multi-tenant
isolation requirements.

## Scope

Northstar is responsible for:

- Not exposing data over the network without a bearer token.
- Hashing bearer tokens before storing them.
- Encrypting third-party API keys (Claude, WHOOP refresh) at rest.
- Logging which prompts/data went to Claude when AI mode is `anthropic`.
- Not phoning home — no telemetry, no analytics SDKs, no crash reporting that leaves your machine.

Out of scope (the user is responsible):

- Server hardening (firewall, SSH, OS updates).
- Encrypted backups — Litestream uses B2 server-side encryption; if you need user-key encryption, layer `restic` underneath.
- Physical access to the device the server runs on.
- Compromise of the iOS device (Face ID + Keychain are the only mitigations).

## Trust boundaries

```
Tailscale / Cloudflare Access / LAN  ◄── you choose
                  │
                  ▼
        northstar-server  (bearer-token gate)
                  │
                  ▼
        SQLite + Litestream
```

The pairing endpoint is the **one** unauthenticated path. The recommended
posture: only expose port 8080 inside your Tailnet. The pairing initiate call
is intended to be made on a trusted LAN.

## Data classes and where they live

| Class | Stored at rest | In transit | Sent to a third party |
|---|---|---|---|
| Bank account numbers | Never on Northstar — stays in SimpleFIN / Actual | — | No |
| Transaction history | SQLite | Server ↔ iOS (HTTPS) | Only via Claude on explicit query |
| WHOOP biometrics | SQLite | Server ↔ iOS (HTTPS) | Only via Claude on explicit query |
| Goals + reminders | SQLite | Server ↔ iOS (HTTPS) | Only via Claude on explicit query |
| Supplement / medication log | SQLite | Server ↔ iOS (HTTPS) | Only via Claude on explicit query |
| Claude API key | SQLite (encrypted) | Server-side only | To Anthropic on every call |
| WHOOP refresh token | Sidecar process memory | Server-side only | To WHOOP on refresh |
| Bearer tokens | SHA-256 hash in SQLite | Returned once at pairing | No |

## What the iOS app does **not** do

- It does not link any Apple/Google analytics SDK.
- It does not include any crash reporter that uploads off-device.
- It does not store data in iCloud — only the local Keychain (one bearer token, one server URL).
- It does not request HealthKit permissions (v1 — Phase 7 will, opt-in).

## Disclosure

See [SECURITY.md](https://github.com/brayden/northstar/blob/main/SECURITY.md) for
the vulnerability disclosure process.
