---
title: Authentication & pairing
description: How devices pair with the server and how the bearer token works.
---

Every API call except `GET /api/health` and the two `/api/pair/*` endpoints
requires a bearer token. Tokens are minted during the pairing flow and stored
in the iOS Keychain.

## The pairing flow

```
iOS app           Server
  │                  │
  │  GET /api/pair/initiate          (no auth)
  │ ───────────────▶ │  generates code, stores pairing_codes row
  │ ◀─────────────── │  {code: "479320", expires_at: ...}
  │                  │
  │  user types code on phone OR scans QR
  │                  │
  │  POST /api/pair/redeem   {code, device_name}
  │ ───────────────▶ │  validates + burns code, mints bearer
  │ ◀─────────────── │  {token: "...", device_id: "...", user_id: "..."}
  │                  │
  │  stores in Keychain
  │                  │
  │  GET /api/me     (Authorization: Bearer ...)
  │ ───────────────▶ │  middleware hashes token, looks up devices row
  │ ◀─────────────── │  {user_id, email, devices}
```

## Endpoints

### `POST /api/pair/initiate`

Mints a 6-digit code valid for `NORTHSTAR_PAIRING_TTL` seconds (default 600).
Gated by `NORTHSTAR_ADMIN_TOKEN` — when the env is set, requests must include
`Authorization: Bearer <token>`. When unset (fresh installs), the endpoint is
open and the server logs a one-time warning at boot; restrict at the network
layer (LAN-only or Tailscale-only) — see [Threat model](/operating/threat-model/).
The one-shot installer generates and uses the admin token automatically, so
this is transparent for the standard path.

```json
// response
{
  "code": "479320",
  "expires_at": 1747076400
}
```

### `POST /api/pair/redeem`

Trades the 6-digit code for a bearer token + device ID.

```json
// request
{ "code": "479320", "device_name": "Brayden's iPhone 15" }

// response
{
  "token": "ns_eyJ...",
  "device_id": "550e8400-...",
  "user_id": "...",
  "expires_at": 0
}
```

Tokens currently have no expiry; revoke by deleting the device row.

### `POST /api/devices/register-apns`

Authenticated. Stores the device's APNs push token.

```json
{ "apns_token": "abcdef0123..." }
```

### `GET /api/me`

Authenticated. Returns the user and their device list.

### `GET /api/pillars`

Authenticated. Returns which pillars are enabled on this server so the iOS app
can hide tabs accordingly.

## Token storage

The server stores `sha256(token)` in the `devices.token_hash` column. The
plaintext token is **only** returned at pairing time and only stored on the
device. There is no "show token" endpoint.
