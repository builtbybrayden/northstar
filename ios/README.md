# northstar-ios

SwiftUI iPhone app for [Northstar](../). iOS 17+. Bundled to use `xcodegen` so the Xcode project file isn't committed (it's machine-generated).

## Open on a Mac

```bash
brew install xcodegen
cd northstar/ios
xcodegen generate
open Northstar.xcodeproj
```

That's it. Build & run on simulator or device.

## Pair with a local server

1. Run the server (see `../server/README.md` or `../infra/`).
2. On the server: `curl -X POST http://localhost:8080/api/pair/initiate -d '{}'` — you'll get a 6-digit code.
3. In the app, enter `http://<your-mac-ip>:8080` and paste the code → "Pair".
4. The app exchanges the code for a bearer token, stored in Keychain.

Once the server admin web page is in place (Phase 1), the code is shown as a `northstar://pair?server=…&code=…` QR — the app's "Scan QR" button will autofill both fields.

## Layout

```
Project.yml                   xcodegen config (deployment target, settings, sources)
Northstar/
├── NorthstarApp.swift         @main, app entry
├── App/
│   ├── AppState.swift         ObservableObject — paired/unpaired, server URL, device ID
│   ├── ContentView.swift      Switches between PairingView and RootTabView
│   └── RootTabView.swift      5-tab bar
├── Pairing/
│   ├── PairingView.swift      Form + Scan QR
│   └── QRScannerView.swift    AVFoundation wrapper
├── Networking/
│   ├── APIClient.swift        REST client for the server
│   └── Keychain.swift         Bearer-token storage (whenUnlockedThisDeviceOnly)
├── Tabs/                      Stub views for the 5 tabs — meat lands Phase 1–5
└── Theme/
    └── Theme.swift            Design tokens (matches mockups/styles.css)
```

## Phase 0 ships

- App entry, dark-mode theme, design-token color palette
- Pairing flow (manual code + QR scan path), bearer-token Keychain storage
- Tab bar (Home / Finance / Goals / Health / Ask) with stub content
- Home: calls `/api/pillars` over the bearer token, confirming the round-trip works

## Phase 1+ (deferred)

- WidgetKit (net-worth + recovery glance)
- Live Activities (budget-threshold progress)
- Per-pillar views (carried over from the mockups in `../mockups/`)
- APNs registration + handling
- fastlane + TestFlight pipeline

## License

MIT. See `../LICENSE`.
