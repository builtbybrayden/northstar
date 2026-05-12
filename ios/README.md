# northstar-ios

SwiftUI iPhone app for [Northstar](../). iOS 17+. Uses `xcodegen` so `Northstar.xcodeproj` is machine-generated — don't commit it (`.gitignore` excludes it).

## Open on a Mac

```bash
brew install xcodegen
cd northstar/ios
xcodegen generate
open Northstar.xcodeproj
```

Build & run on simulator (free) or a physical iPhone signed with your personal Apple ID (free, 7-day signing). You do **not** need to pay the Apple Developer Program ($99/yr) to put the app on your own phone for testing — that's only required for TestFlight, the App Store, and production APNs.

## Pair with a local server

1. Run the server (see `../server/README.md` or `../infra/`).
2. On the server: `curl -X POST http://localhost:8080/api/pair/initiate -d '{}'` — you'll get a 6-digit code.
3. In the app, enter `http://<your-mac-ip>:8080` and paste the code → "Pair".
4. The app exchanges the code for a bearer token, stored in Keychain.

Once the server admin web page is in place (Phase 1), the code is shown as a `northstar://pair?server=…&code=…` QR — the app's "Scan QR" button will autofill both fields.

## Layout

```
Project.yml                          xcodegen config (deployment target, settings, sources)
Northstar/
├── NorthstarApp.swift               @main, app entry
├── App/                             AppState, ContentView, RootTabView (tab shell)
├── Pairing/                         PairingView + QR scanner
├── Networking/                      APIClient + Keychain + per-pillar Codable models
├── Theme/Theme.swift                Design tokens (matches mockups/styles.css)
├── Tabs/                            Home, Finance, Goals, Health, Ask
├── Finance/EditBudgetSheet.swift
├── Goals/                           MilestoneEditSheet, WeeklyMonthly, OutputLog,
│                                    NetworkingLog, RemindersView
├── Health/SupplementEditSheet.swift
├── AI/ConversationScopeSheet.swift
├── Notifications/                   NotificationsView + NotificationRuleSheet
└── Settings/SettingsView.swift
```

## Build with the server in mock mode

The whole stack runs without credentials. From `northstar/`:

```bash
cd infra && docker compose up -d   # boots actual-sidecar + whoop-sidecar + server (mock mode)
curl -X POST http://localhost:8080/api/pair/initiate -d '{}'   # admin-token gated when env is set; mint the 6-digit code
```

In the iOS app, enter `http://<your-mac-ip>:8080` and paste the code → **Pair**.

## What's currently shipped

- Pairing (manual code + QR), bearer-token Keychain storage
- Home (verdict, pillar grid, bell + settings)
- Finance (rings, category bars, edit-budget sheet, recent transactions)
- Goals (flagship hero, daily brief, milestones, planners, output/networking/reminders)
- Health (recovery dial, metric grid, 7-day spark, supplement list + dose log, schedule UI)
- Ask (streaming chat, tool-call chips, cancel-mid-stream, rename, scope picker, tool-error chips)
- Notifications (feed + live SSE stream + per-category rule sheet w/ quiet hours)
- Settings (connections, notification rules, pillars, sign-out)

## What's deferred

- WidgetKit + Live Activities (needs Apple Developer Program for production push entitlements)
- APNs production registration (currently `LogSender` server-side)
- fastlane + TestFlight pipeline (paid program required)
- Apple Watch glance (Phase 7)
- HealthKit ingest as a fallback to WHOOP (Phase 7)

## License

MIT. See `../LICENSE`.
