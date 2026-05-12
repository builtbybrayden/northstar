---
title: Repo layout
description: Where everything lives.
---

```
northstar/
├── server/                 Go service (chi + SQLite + Litestream)
│   ├── cmd/northstar-server/    main.go
│   ├── internal/
│   │   ├── ai/                   Anthropic bridge + tool dispatch + mock engine
│   │   ├── api/                  chi router, mounts pillars conditionally
│   │   ├── auth/                 token hashing + bearer middleware
│   │   ├── config/               env-driven config
│   │   ├── db/                   *.sql migrations + goose runner
│   │   ├── finance/              Actual sync + handlers + detectors
│   │   ├── goals/                native CRUD + brief composer + cron parser
│   │   ├── health/               WHOOP sync + handlers + recovery-drop detector
│   │   ├── notify/               rule eval + dedup + quiet hours + log sender
│   │   └── scheduler/            minute ticker for daily_brief / reminders
│   ├── go.mod / go.sum
│   └── Dockerfile
│
├── ios/                    SwiftUI app, iOS 17+, xcodegen-generated
│   ├── Project.yml              xcodegen spec — generates Northstar.xcodeproj
│   ├── Northstar/
│   │   ├── App/                  AppState, RootTabView, ContentView
│   │   ├── Pairing/              PairingView + QRScannerView
│   │   ├── Networking/           APIClient + Keychain + per-pillar models
│   │   ├── Theme/                design tokens
│   │   ├── Tabs/                 Home, Finance, Goals, Health, Ask
│   │   ├── Finance/              EditBudgetSheet
│   │   ├── Goals/                MilestoneEditSheet, WeeklyMonthly, OutputLog, Networking, Reminders
│   │   ├── Health/               SupplementEditSheet
│   │   ├── Settings/             SettingsView
│   │   └── Notifications/        NotificationsView
│   └── README.md
│
├── infra/
│   ├── docker-compose.yml       server + sidecars + (profile) litestream
│   ├── litestream.yml           replicates SQLite to B2
│   └── README.md
│
├── tools/
│   ├── actual-sidecar/          Node ESM gateway, mock + real
│   ├── whoop-sidecar/           Node ESM gateway, mock + real
│   └── notion-importer/         One-shot CLI to seed goals from Notion
│
├── docs-site/              Astro/Starlight documentation site
├── docs/                   plan + design docs
│   ├── ULTRAPLAN.md             full plan
│   └── WINDOWS-IOS-DEV.md       develop iOS from Windows
├── mockups/                static HTML UI concepts
├── scripts/                installer scripts
└── .github/workflows/      CI
```

Each top-level directory under `northstar/` corresponds to a separate public
repo at launch — they're development-coupled here for now.
