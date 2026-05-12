# Northstar

A self-hosted iOS app that ties together your **finance** (Actual Budget), **goals** (native), and **biometrics** (WHOOP) — with Claude as a cross-pillar brain.

> Status: **Phases 0–5 + Phase 6 partial + ops-polish + iOS polish + pillar-scope** (2026-05-12). All pillars functional in mock mode end-to-end — boot it, pair a phone, you have a working dashboard. Apple Dev Program path (TestFlight, App Store, APNs production sender) explicitly deferred. Polish wave: admin-token gate, initial-sync purchase-spam suppression, supplement reminder auto-firing with schedule UI, per-category quiet hours UI, sleep + overreach health detectors, SSE live-notification fanout, Anthropic token-usage telemetry, chat stream cancel + conversation rename + scope picker + inline tool-error surfacing. **7 server packages now pass tests (added `db` + `scheduler` coverage); 38 total new tests across the wave (17 polish + 21 covering migrations/filter/normalize/scheduler).**

Full docs at `docs-site/` — `cd docs-site && npm install && npm run dev` for the local site.

## Workspace layout

```
northstar/
├── server/                 northstar-server      Go service (chi + SQLite + Litestream)
├── ios/                    northstar-ios         SwiftUI app, iOS 17+, xcodegen-generated
├── infra/                                        docker compose, Litestream config
├── tools/
│   └── actual-sidecar/                           Node.js Actual Budget gateway (mock + real)
├── docs/                                         Plan + design docs
│   ├── ULTRAPLAN.md                              Full plan
│   └── WINDOWS-IOS-DEV.md                        Develop the iOS app primarily from Windows
├── mockups/                                      Static HTML UI concepts (open in browser)
└── .github/workflows/
    ├── ios.yml                                   macOS runner build + (future) TestFlight
    ├── server.yml                                Go build + Docker build
    └── sidecar.yml                               Node boot probe
```

Each top-level directory is its own public repo at launch — they're development-coupled here.

## Locked decisions (2026-05-12)

| | |
|---|---|
| Name | **Northstar** |
| Server language | **Go** single binary + Docker compose |
| Database | **SQLite + Litestream** → B2 |
| License | **MIT** on all repos |
| Day-1 distribution | **TestFlight** + AltStore docs as fallback |

## Quick start (server, Windows)

```powershell
cd server
copy .env.example .env
go run ./cmd/northstar-server
# In another shell:
curl http://localhost:8080/api/health
```

## Quick start (Docker)

```bash
cd infra
docker compose up -d server
curl http://localhost:8080/api/health
```

## Quick start (iOS, on a Mac)

```bash
brew install xcodegen
cd ios && xcodegen generate && open Northstar.xcodeproj
```

## End-to-end pairing test

```bash
# 1. Mint a pairing code:
PAIR=$(curl -s -X POST http://localhost:8080/api/pair/initiate -d '{}')
CODE=$(echo "$PAIR" | sed -n 's/.*"code":"\([0-9]*\)".*/\1/p')

# 2. From iOS: enter http://<server-ip>:8080 + the 6-digit code → "Pair".
# 3. The app receives a bearer token; the device row is persisted.
```

## View the UI concepts

```powershell
# Windows
Start-Process "C:\Users\Brayden\projects\northstar\mockups\index.html"
```
Or just double-click `mockups/index.html`.

## Phase 0 scope (shipped)

- ✅ Workspace layout + READMEs
- ✅ Go server scaffold (`chi`, `modernc.org/sqlite`, `goose`)
- ✅ SQLite schema migration covering all pillars
- ✅ Health endpoint + bearer-auth middleware
- ✅ Device pairing flow (initiate / redeem / register-APNs)
- ✅ Docker compose with Litestream backup sidecar (opt-in `--profile backup`)
- ✅ iOS scaffold (xcodegen, pairing + QR scanner, 5-tab shell, Keychain, theme tokens)
- ✅ Verified end-to-end: server pairs, consumed-code 410, bad-token 401

## Phase 1 scope (shipped)

- ✅ **Node Actual sidecar** at `tools/actual-sidecar/` — long-lived HTTP gateway. Mock mode (default) returns realistic data so the stack works without an Actual server. Real mode uses `@actual-app/api`.
- ✅ **Go finance sync worker** — periodic pull from the sidecar (15 min default), upserts into `fin_accounts` / `fin_transactions`, seeds `fin_budget_targets` on first run.
- ✅ **Finance REST endpoints** — `/api/finance/{accounts,transactions,summary}` with per-category spend/budget/pct/over and net-worth split on-budget vs off-budget.
- ✅ **Sidecar wired into Docker compose** with healthcheck; server `depends_on` sidecar.
- ✅ **iOS Finance tab** ported from stub → real SwiftUI: net-worth hero, 3 rings (Spent/Saved/Income), per-category bars with color thresholds, recent-transactions list, pull-to-refresh.
- ✅ **GitHub Actions CI** — `ios.yml` (macOS runner, xcodegen, xcodebuild), `server.yml` (Go vet + build + test + Docker build), `sidecar.yml` (Node boot probe).
- ✅ **Windows-iOS dev guide** at `docs/WINDOWS-IOS-DEV.md` — workflow for developing on Windows and using the Mac only for live simulator debugging.
- ✅ Verified Phase 1 end-to-end: sidecar serves mock data → server syncs → REST endpoints return real numbers (11 accounts, 31 txns, 9 budgeted categories with thresholds correctly flagging Mortgage at 99%).

## Phase 2 scope (shipped)

- ✅ **Migration 00002** — `notifications` (with `dedup_key UNIQUE`), `fin_merchant_stats`, `notif_daily_counts`
- ✅ **Notification engine** (`internal/notify/`) — Composer with rule eval + dedup + quiet-hours + daily cap + persistence; LogSender default; APNSSender wired but inert (no creds needed)
- ✅ **Threshold checker** in sync — fires only the highest unfired ladder step per category per month; `fin_threshold_state` remembers; priority 5/7/9 by step (50/90/100)
- ✅ **Purchase detection** — every new transaction fires `purchase` (rate-limited by daily cap)
- ✅ **Anomaly detection** — first-time merchant ≥$200 + 3× median spike with ≥4 prior samples
- ✅ **REST endpoints** — `/api/notifications/{feed,unread-count,/:id/read,rules,rules/:category}`, `/api/finance/budget-targets`, `PATCH /api/finance/budget-targets/:category`
- ✅ **iOS Settings** — real screen (replaces stub) with Connections, Notification rules toggles, Pillars, sign-out; backed by the new APIs
- ✅ **iOS Edit Budget sheet** — tap any category bar in Finance → bottom sheet with cap editor, threshold chips, push toggle; PATCH on save
- ✅ **iOS Notifications feed** — bell + badge in Home toolbar, full list view with category-colored icons + relative timestamps + auto-mark-read
- ✅ **iOS Home upgrade** — verdict ("Pull back on X" / "Pace is healthy"), pillar grid showing spend MTD, settings gear + notifications bell with unread badge
- ✅ **First Go tests** — composer dedup, quiet-hours wrap-around, critical-bypass, daily-cap; threshold parser + dollars formatter + body transitions
- ✅ **Verified live**: 33 notifications fired on initial sync (28 purchase + 4 threshold + 1 anomaly); restart = 0 refires; budget edit $500→$300 fired exactly one new p9 `Restaurants at 105%`

## Phase 3 scope (shipped)

- ✅ **`internal/goals/`** — Handlers, types, brief composer, cron parser (5-field, via `robfig/cron/v3`)
- ✅ **CRUD for all 7 native goal tables**:
  - `goal_milestones` — POST / PATCH / archive (soft-delete) / list with `archived=1` opt-in
  - `goal_daily_log` — GET/PUT keyed by date; PUT triggers streak recomputation
  - `goal_weekly_tracker` — GET/PUT keyed by Monday-of-week
  - `goal_monthly` — GET/PUT keyed by YYYY-MM
  - `goal_output_log` — list + POST (CVE, blog, talk, tool, cert, PR, report)
  - `goal_networking_log` — list + POST
  - `goal_reminders` — list / POST / PATCH / DELETE; recurrence is 5-field cron
- ✅ **Daily brief composer** (`/api/goals/brief`) — pulls today's items + reminders firing today + yesterday's incomplete rolled over + milestones due within 7d
- ✅ **Streak counter** — back-walks day-by-day from anchor until first empty/missing day
- ✅ **User settings endpoints** (`/api/me/settings`) — daily_brief_time, evening_retro_time, timezone; backed by `users.settings_json`
- ✅ **`internal/scheduler/`** — minute-ticking cron: fires daily_brief at user's morning time, evening_retro at evening time, any `goal_reminders` with `next_fires_at <= now` (and rolls `next_fires_at` forward to the next cron occurrence)
- ✅ **iOS Goals tab** — flagship hero (gradient indigo card), today's brief (checkable items + source labels), full milestones list with status/due/flagship badges; pull-to-refresh; streak pill in toolbar; add-milestone (+) button
- ✅ **iOS MilestoneEditSheet** — title, description, due-date picker, status segmented control, flagship toggle, archive button on existing rows
- ✅ **`tools/notion-importer/`** — Node CLI; one-shot pull from Notion workspace into Northstar via REST. Loose property matching ("Status"/"Due"/"Category" variants), `--dry-run` mode
- ✅ **Goals tests** — cron parsing (3 patterns + invalid), streak edge cases (consecutive / gap / empty items), brief composer (today + rollovers, milestones-in-7d window, reminders-injected-as-items + inactive excluded), mondayOf helper
- ✅ **Verified live**: created 3 milestones via API, listed back correctly. Wrote daily log → streak counter returned 1. Created a `0 7 * * *` reminder → cron parsed correctly. Set `daily_brief_time` to T+1min, waited 80s, daily_brief notification fired exactly once at scheduler tick

## Phase 3.5 scope (shipped)

- ✅ **iOS WeeklyMonthlyView** — single screen with segmented Week/Month scope, theme + goals + retro editor; backed by /api/goals/weekly + monthly
- ✅ **iOS OutputLogView + AddOutputSheet** — list with category-colored chips (CVE/blog/talk/tool/cert/pr/report); + button → form
- ✅ **iOS NetworkingLogView + AddNetworkingSheet** — list with person/context/next-action; + button → form with optional due-date
- ✅ **iOS RemindersView + ReminderEditSheet** — cron preset chips (every-morning-7am / weekdays-9am / etc.) + raw cron entry; active toggle; delete
- ✅ **Goals tab "More" section** — NavigationLinks to all four planner/log surfaces

## Phase 4 scope (shipped)

- ✅ **`tools/whoop-sidecar/`** — Node ESM HTTP gateway exposing /recovery /sleep /strain /profile. Mock mode default with 14-day window (today 84%, day 3 dip to 28%). Real mode uses WHOOP v2 OAuth2 with refresh-token handling
- ✅ **`server/internal/health/`** — sidecar.go (HTTP client), sync.go (Syncer with 15-min default, upserts recovery/sleep/strain), detect.go (CheckRecoveryDrop fires health_insight when today < yesterday by ≥30 pts), handlers.go (full REST surface)
- ✅ **REST endpoints**:
  - `GET /api/health/today` — composes recovery + sleep + strain into one payload with computed verdict (push/maintain/recover) and strain_goal range
  - `GET /api/health/recovery|sleep|strain?days=N` — last N days
  - `GET/POST/PATCH/DELETE /api/health/supplements/defs[/:id]` — supplement/peptide/medication CRUD with cycle on/off days, reminder toggle, prescribing doc
  - `POST/GET /api/health/supplements/log` — log doses with optional notes
  - `PUT /api/health/mood/:date` — daily mood/energy/focus log
- ✅ **iOS Health tab** — replaces stub. WHOOP-style recovery dial (180×180, color-coded green/yellow/red), HRV/RHR/sleep metric grid with deltas, 7-day recovery spark, supplement list with tap-to-edit + tap-circle-to-log-dose
- ✅ **iOS SupplementEditSheet** — name, dose, category (supplement/peptide/medication), cycle on/off, reminder toggle, prescribing-doc + notes, archive button
- ✅ **Health tests** — 4 detector cases: fire on big drop, no fire on small drop, no fire on insufficient history, dedup on second call
- ✅ **Verified end-to-end**: 14 days of recovery/sleep/strain synced from mock sidecar. `/api/health/today` returned exactly the mockup values: 84% recovery, HRV 78ms, RHR 51, sleep 7h42m / score 88, strain 16.0, verdict "push", strain goal "16.0–18.5". Supplement CRUD + dose log all working

## Phase 5 scope (shipped)

- ✅ **`server/internal/ai/`** — types.go (Anthropic wire types), client.go (8-turn tool-use loop with SSE parsing + `anthropic-beta: prompt-caching-2024-07-31`), tools.go (dispatcher + 10 tool defs), prompt.go (cached system prompt with "Not medical advice" safety rail), mock.go (keyword-routed MockEngine for dev without API key), handlers.go (REST + SSE)
- ✅ **10 cross-pillar tools** — `finance_{summary,search_transactions,category_history,subscriptions}`, `goals_{brief,milestones,recent_output}`, `health_{today,recovery_history,supplements}`. Last tool def carries `cache_control: ephemeral` so the whole tool block stays in Anthropic's prompt cache
- ✅ **System prompt** with two cached blocks (persona + safety rail). `SystemBlocks(now)` injects today's date so caching survives day rollover
- ✅ **REST + SSE endpoints**:
  - `GET/POST/DELETE /api/ai/conversations[/:id]` — list / create / soft-delete (sets `archived = 1`)
  - `GET /api/ai/conversations/:id/messages` — full message history
  - `POST /api/ai/conversations/:id/messages` (SSE) — streams `{type: text|tool_call|done|error}` events, persists user + assistant turns, auto-titles from the first user message (≤60 chars)
- ✅ **Mock mode by default** — `NORTHSTAR_AI_MODE=mock` is the default so the pipeline runs end-to-end without an Anthropic key. Keyword routing: "push/recover/today" → `health_today`, "budget/spend/spent" → `finance_summary`, "goal/milestone/streak" → `goals_brief`. Flip to real Claude with `NORTHSTAR_AI_MODE=anthropic` + `NORTHSTAR_CLAUDE_API_KEY=...`; model defaults to `claude-sonnet-4-6`
- ✅ **iOS Ask tab** wired into the SSE stream via `APIClient.aiSendMessageStream(...)`; tool-call markers appear inline alongside streamed text
- ✅ **AI tests** — 8 cases covering `finance_summary` aggregation, `finance_search_transactions` payee filter, `goals_milestones` archived-exclusion + status-in filter, `health_today` push/maintain/recover verdict bands, unknown-tool error, last-tool + last-system-block cache_control invariants
- ✅ **Verified live in mock mode**: paired a device → created a conversation → sent "Should I push hard today?" → SSE emitted `tool_call:health_today` then 21 text chunks embedding real DB data (recovery 84, verdict "push") then `done`; conversation auto-titled; both messages persisted

## Phase 6 partial (shipped 2026-05-12)

- ✅ **`docs-site/`** — Astro + Starlight documentation site. 22 pages spanning getting-started / pillars / configuration / api / operating / contributing. Runs locally via `npm install && npm run dev`. Build is static HTML — host on Cloudflare Pages / Netlify / GitHub Pages.
- ✅ **One-shot installers** — `scripts/install.sh` (Linux/macOS) + `scripts/install.ps1` (Windows + Docker Desktop). Idempotent: verifies Docker, provisions `~/.northstar/`, generates master passphrase + admin token, boots compose, prints pairing code. Matching `uninstall.{sh,ps1}`.
- ✅ **CONTRIBUTING.md** at the workspace root — opinionated about what's in/out of scope (no telemetry ever; no new pillars; no third-party iOS SDKs).
- ⏸ **CODE_OF_CONDUCT + SECURITY + ROADMAP + GH templates + Show HN draft** — paused; pick up when launch is closer.

## Server / infra polish (shipped 2026-05-12)

- ✅ **Admin-token gate on `/api/pair/initiate`** — `NORTHSTAR_ADMIN_TOKEN` env. When empty (fresh installs) the endpoint is open and the server logs a one-time boot warning; when set, requires `Authorization: Bearer <token>` (constant-time compare). The one-shot installer generates and uses the token automatically so first-time pairing isn't blocked.
- ✅ **Initial-sync purchase-spam suppression** — first finance sync detects an empty `fin_transactions` table and suppresses per-txn `purchase` + `anomaly` notifications for the backfill pass. Threshold notifications still fire (one-time over-budget signal is useful). Merchant stats keep updating so subsequent syncs can detect anomalies normally.
- ✅ **Supplement reminder auto-firing** — `internal/health/reminders.go` parses `schedule_json` (`{"times":["07:00","19:00"],"days":["mon",...]}` or bare `["HH:MM"]` array), gates on cycle on/off relative to `created_at`, fires `supplement` notifications at the configured minute in the user's timezone. iOS `SupplementEditSheet` now writes time pickers into the JSON.
- ✅ **Per-category quiet hours UI** — `NotificationRuleSheet.swift` for each rule: toggle enabled, set quiet start/end with wrap-around support, bypass-quiet for criticals, daily cap stepper. Reachable from Settings → Notification rules.
- ✅ **Sleep + overreach health detectors** — `CheckSleepIssues` (debt > 90 min, score < 50) and `CheckOverreach` (strain ≥ 18 with recovery < 50). Wired into `health.Syncer.SyncOnce` alongside the existing recovery-drop detector. Each has its own dedup key so multiple flags on the same day don't collide.
- ✅ **SSE live-notification fanout** — `notify.Hub` publishes every successful `Composer.Fire` to subscribed iOS clients via `GET /api/notifications/stream`. Heartbeat every 25s. Home and Notifications views auto-connect on appear, reconnect with 3s backoff on stream close. Lets the bell badge tick up in real time while the app is foregrounded.
- ✅ **Anthropic token-usage telemetry** — `ai.Usage` struct captures input + output + cache-read + cache-creation tokens from `message_start` / `message_delta` stream frames. `StreamConversation` aggregates across all 8 turn slots. Migration `00003_ai_usage.sql` adds `usage_json` column to `ai_messages`; handler persists it alongside the assistant content blocks.
- ✅ **iOS Ask polish** — chat streaming-cancel button (red `stop.fill` while streaming; `URLSession.bytes` task cancellation propagates to the server through `r.Context()`), conversation rename via swipe + context menu (new `PATCH /api/ai/conversations/:id`), inline `tool_error` markers so failed tool calls surface as `tool-name ✗` chips instead of silent retries.

## Still deferred

- **Apple Developer Program-blocked:** TestFlight pipeline, App Store submission, APNs production sender (`sideshow/apns2`), Live Activity for budget thresholds, lock-screen Widget for net worth.
- **Credential-blocked smoke tests:** `NORTHSTAR_AI_MODE=anthropic` against a real Claude key; Actual sidecar in `real` mode; WHOOP sidecar in `real` mode (WHOOP dev account approval lag).
- **Pillar-scope filtering** — `pillar_scope` column exists on `ai_conversations` and defaults to `[]`, but neither the server filter nor a UI to set it has been wired. Cosmetic until both sides land.
- **Phase 6 launch finish** — CODE_OF_CONDUCT + SECURITY + ROADMAP + GitHub issue/PR templates + Show HN draft.
- **Phase 7** — HealthKit ingest, Apple Watch glance, Android (Compose) community port, multi-user / family accounts, investment portfolio view, habit tracker beyond `goal_milestones`, cash-flow forecast view, receipt OCR.

## License

MIT — see [`LICENSE`](LICENSE).
