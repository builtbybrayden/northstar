---
title: Pair your iPhone
description: Connect the iOS app to your self-hosted server using a 6-digit code.
---

Pairing exchanges a short-lived 6-digit code for a long-lived bearer token that
lives in the iOS Keychain. The server never stores tokens in plaintext — only a
SHA-256 hash.

## 1. Build the iOS app

For now you'll need a Mac to build the app yourself:

```sh
brew install xcodegen
cd ios
xcodegen generate
open Northstar.xcodeproj
```

Hit ▶ in Xcode with an iPhone (physical or simulator) selected. Choose your team
under Signing & Capabilities first.

> A public TestFlight link is on the [roadmap](/operating/upgrades/#roadmap) for
> v1.0 once the Apple Developer Program enrollment lands.

## 2. Mint a pairing code on the server

```sh
curl -X POST http://<server-ip>:8080/api/pair/initiate -d '{}'
# {"code": "479320", "expires_at": 1747076400}
```

Codes are valid for 10 minutes and burned after one redeem.

## 3. Pair from the app

On first launch the app shows the Pairing screen. You have two options:

- **Manual:** enter the server URL (`http://192.168.1.42:8080`) and the 6-digit code.
- **QR scan:** the server admin web page (or `make qr` on the host) prints a QR
  containing both fields.

The app calls `POST /api/pair/redeem`, receives a bearer token, and stores it
in the iOS Keychain under `northstar.bearer`.

## 4. Confirm

After pairing the app drops into the Home tab. Tap **Settings → Connections** to
verify the server is reachable. From here you can wire up Actual, WHOOP, and
Claude credentials.

## Revoking a device

```sh
sqlite3 ~/.northstar/data.db \
  "DELETE FROM devices WHERE id = '<device-id>';"
```

The next API call from that device will get a 401; the app will return to the
Pairing screen.
