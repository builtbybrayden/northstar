# Developing the iOS app primarily from Windows

> tl;dr: write Swift on Windows in VS Code (SwiftFormat + SourceKit-LSP work fine), push to GitHub, the macOS runner builds + signs + ships to TestFlight. Use a Mac only when you need live simulator debugging.

## Why this works

Apple gates Xcode to macOS, but Apple does **not** gate the *outputs*. A binary built on a cloud Mac is identical to one built on your laptop. So:

| Task | Where you do it |
|---|---|
| Write Swift code | Windows (VS Code + Swift extension) |
| Validate it builds | `git push` → GitHub Actions macOS runner |
| Run in simulator | Mac (your MacBook or a cloud Mac) |
| TestFlight upload | GitHub Actions on a tagged release |
| Live UI debugging | Mac, briefly |

The Mac becomes "the simulator/debugger box" not "the dev box."

## Setup on Windows

1. Install **VS Code** + the **Swift extension** (sswg.swift-lang) + **SourceKit-LSP**.
2. Install **Swift for Windows** (https://swift.org/install/windows) — gets you syntax checking and parts of the Foundation API locally.
3. Use the iOS app source in `ios/` like any other project. Autocomplete works for Swift-language constructs; SwiftUI / UIKit symbols won't resolve (those are Apple SDK only) — that's OK, the cloud build catches errors.

## Setup the Mac (one-time)

1. `brew install xcodegen`
2. `cd northstar/ios && xcodegen generate`
3. `open Northstar.xcodeproj`
4. In Xcode, sign in with your Apple ID under *Settings → Accounts* (free tier is enough for simulator + on-device development).

## CI workflow

The repo includes `.github/workflows/ios.yml`:
- **On every push** to `ios/**` it generates the project, runs `xcodebuild` against the iPhone 15 simulator, and fails the build if anything regressed. Cost: ~6–8 macOS-runner minutes per push.
- **On a `v*` tag** it builds an archive + uploads to TestFlight (currently commented out — needs Apple Developer credentials).

## When you actually need the Mac

- The cloud build caught a Swift compile error and you need to iterate quickly — open the Mac, fix locally, push.
- You're working on a visual layout (rings, gauges, gradient cards) and want to see live changes — Xcode preview canvas is irreplaceable for this.
- You're debugging a runtime crash (LLDB on simulator).

For everything else — typing Swift, refactoring, writing models, editing test fixtures — Windows is fine.

## Cost expectations

- GitHub Actions macOS runners: **$0.08/min** for private repos (free for public). A typical iOS Debug build is 4–6 min. Call it **~$0.40/push** worst case, less if you batch commits.
- Private repo with 10 pushes/day → ~$120/month if you push constantly. In practice it's much less because most pushes don't touch `ios/`.
- **Open source the repo and it's all free.** That's the recommended path anyway.

## Setting up TestFlight delivery later

When you're ready to ship to TestFlight from CI:

1. Apple Developer Program enrollment ($99/yr).
2. Generate an **App Store Connect API key** (Users and Access → Keys). Download the `.p8`.
3. In the GitHub repo, set these secrets:
   - `APPLE_API_KEY_ID`
   - `APPLE_API_ISSUER_ID`
   - `APPLE_API_KEY_BASE64` (`base64 -i AuthKey_XXXX.p8`)
4. Uncomment the TestFlight upload step in `.github/workflows/ios.yml`.
5. Push a tag: `git tag v0.1.0 && git push origin v0.1.0`. Build + upload happens in CI.

## For OSS contributors who don't own a Mac

The CONTRIBUTING.md guidance:
- Fork the repo; GitHub Actions runs the iOS CI on your fork for free (public repos).
- If you need to interact with the simulator, sign up for a cloud Mac trial (MacInCloud has a free trial; AWS EC2 Mac is metered).
- Or — and this is the most realistic answer — focus contributions on the **server**, **sidecar**, **docs**, or **tools** repos. iOS contributions need *some* Mac access; there's no way around that.
