# Northstar — Personal Life OS

> A single iOS app that ties together your money, your goals, and your body — running on your own server, owned by you, and open to anyone who wants the same thing.

## Locked decisions (2026-05-12)

| | Choice | Why |
|---|---|---|
| **Name** | Northstar | Direction-toward-goals metaphor; ties to the career-roadmap framing |
| **Server language** | Go | Single-binary deploy, best APNs library, learning bet for the Year-2 plan |
| **License** | MIT (both repos) | Self-hosted personal app isn't really at SaaS-fork risk; MIT cuts contributor friction |
| **Day-1 distribution** | TestFlight + AltStore docs as fallback | "Paste link, install" UX; App Store at v1.0 |
| **Database** | SQLite + Litestream | Single-file ops, streaming backups to B2; fine for single-user scale for years |

---

## 1. Vision

**One sentence.** A self-hosted "life dashboard" that ingests finance data from your existing Actual Budget instance and biometrics from WHOOP, manages your career goals natively, surfaces all three in a single Robinhood-grade iOS app, and lets you ask Claude questions like *"Should I push hard today?"* or *"Am I on track for OSCP by April?"* against your actual data.

**Why now.** Two of three inputs are already in your stack, and the third has a clean import path:
- Finance data is already aggregated in Actual Budget at `~/projects/finance/`
- WHOOP wrist-data has a public developer API
- Career roadmap currently lives in Notion (Daily Log, Weekly Tracker, Monthly Goals, Milestones) — Northstar will manage goals natively, with a one-time Notion-import CLI to seed your personal data
- You have Claude API access already

Nothing else needs to be built to *acquire* the data — only to **unify, visualize, and reason over it** on your phone. The gap between "I have all this data" and "I make decisions with it daily" is the entire product.

**Why open-source.** Three reasons:
1. **Portability.** If you stop maintaining it, you can still use it.
2. **Career fit.** This is a substantial Swift/Go/iOS portfolio piece tied to the cybersec-engineer path you locked in for Year 5.
3. **There's no good OSS version of this.** Self-hosters string together Grafana + Home Assistant + Beancount + a notes app and it always looks like a science fair. There's room for a real one.

---

## 2. Design principles

| | |
|---|---|
| **Self-host by default** | Single Docker compose. No mandatory cloud. The Apple TestFlight build talks to *your* server, not ours. |
| **Own your data** | SQLite on your box, single file. Litestream streams encrypted backups to your B2 bucket. Export to JSON anytime. |
| **Mobile-first** | The phone is the primary surface. Web admin exists only for setup. |
| **Privacy-first** | No analytics. No telemetry. No third-party SDKs in the iOS app. Network calls are documented and auditable. |
| **Claude is opt-in** | Bring your own API key. Your data never leaves your server except for the specific prompt you sent. Conversations are stored locally. |
| **Pillars are modular** | Run finance-only. Or health-only. Each pillar can be disabled without breaking the others. |
| **Don't reinvent ingest where it works** | Mirror Actual and WHOOP — they're solving problems Northstar shouldn't redo. Goals are native because no good existing source covers the cross-cutting use case without external coupling. |

---

## 3. The three pillars

### Pillar 1 — Finance

**Source of truth:** your existing Actual Budget server (`docker compose -f compose/docker-compose.yml`).

**What Northstar adds on top of Actual:**
- Native iOS dashboard with the Robinhood-style "big number on black" aesthetic
- **Push notification on every transaction** (Actual itself has no push)
- **Threshold alerts** at 50% / 75% / 90% / 100% per category per month (your `daily-summary.js` already computes these — Northstar surfaces them on the lock screen, not in a once-a-day digest)
- **Spend pace** widget — "you're 17 days into the month and 23% under pace on Groceries"
- **Anomaly alerts** — first time you've spent $400 at a single merchant; this category is 3× last month's median
- **Subscription audit** view — feed from your `subscription-monitor.js`
- **Net-worth widget** on iOS lock screen (Live Activity / WidgetKit)

**Implementation note:** Northstar mirrors Actual into a local SQLite cache for fast reads, and routes any user edits (category override, budget cap change) back through Actual's API so Actual stays authoritative. This keeps your existing finance pipeline (`reclassify-transfers.js`, daily-summary push, rule engine) working even if Northstar is down.

**Categories that already exist** (from `setup-categories-and-rules.js` + `consolidate-categories.js`):
25 curated categories — Groceries, Restaurants, Gas, Utilities, Rent/Mortgage, Subscriptions, Travel, Medical, etc. Northstar picks these up via the Actual API and renders them as the budget rings.

### Pillar 2 — Goals

**Source of truth:** **Northstar's own SQLite database — native, first-class.** No external dependency. (Compare to Finance, which mirrors Actual, and Health, which mirrors WHOOP — Goals lives entirely inside Northstar.)

**Why native, not Notion-synced.** Two-way Notion sync was the original design, but it's the right call to drop:
1. **One less external dependency** for OSS users — no Notion workspace, no integration token, no API rate limits.
2. **No drift-class bugs** between local state and Notion state.
3. **The Notion structure ports cleanly** to SQLite tables — Daily Log, Weekly Tracker, Monthly Goals, Milestones, Output Log, Networking Log, Reminders all become native models (see §5).
4. **Edits are simpler** — tap → save → done. No optimistic-update-then-reconcile dance.

**Personal seed-import (you only, one-time).** Your existing Notion data — the entire "Cybersecurity Career Roadmap" workspace — gets pulled in once via a CLI in `northstar-tools`:

```
northstar-tools import-notion \
  --workspace=3560ea6ce4a681b484a2e6da60d8f7d6 \
  --notion-token=$NOTION_TOKEN \
  --tables=daily_log,weekly_tracker,monthly_goals,milestones,output_log,networking_log,reminders
```

This is a one-shot operation. After import you edit in Northstar, not Notion. Notion stays as the archival source; Northstar becomes the live one. The CLI ships in a separate repo and OSS users never see it.

**Native models** (full schema in §5): `goal_milestones`, `goal_daily_log`, `goal_weekly_tracker`, `goal_monthly`, `goal_output_log`, `goal_networking_log`, `goal_reminders`.

**What Northstar surfaces from these:**
- **Short-term goals view** — sorted by due date, with progress bars
- **Long-term goals view** — Year 5 decision gate (2031-05-04), OSCP (~Apr 2027), Senior offer (2027-05-04), CVE-first (2027-01-31)
- **Daily brief notification** at a chosen morning time — "Today: ship one Python tool commit, run 30 min PortSwigger, finish the Bykea writeup"
- **Evening retro nudge** — "Log today in the Daily Log database"
- **Streak tracking** — consecutive days you've logged
- **Quarterly retro reminder** — fires on the last Sunday of each quarter
- **Early warnings** baked into the home screen badge: 3 empty Weekly Tracker rows in a row, 60+ hr work weeks, 2 missed quarters, TC stuck below $180k by end of Year 3

**Implementation:** Pure local CRUD via the Northstar REST API. No external sync. iOS optimistic-updates the UI immediately; the server persists on the next round-trip. The complexity budget freed up here goes into making the edit affordances *good*.

### Pillar 3 — Health & Performance

**Source of truth:**
- WHOOP API (recovery, strain, sleep, HRV, RHR)
- Manual logs in-app: medications, supplements, peptides, mood, soreness

**What Northstar adds:**
- **Recovery dial** — WHOOP-style 0–100% gauge, color-coded green/yellow/red
- **Today's readiness card** — recovery + last night's sleep score + HRV trend + recommendation ("Push" / "Maintain" / "Recover")
- **Supplement / peptide logger** — schedule-driven (daily NAD+ at 7am, weekly BPC-157 cycle 4-on-2-off, etc.) with push reminders
- **Correlation explorer** — "show me recovery score on days I took X" or "sleep quality vs alcohol days"
- **Workout journal** — pulls strain + heart-rate-zone breakdown from WHOOP, plus optional manual notes
- **Cycle awareness for peptide stacks** — e.g. "BPC-157 cycle ends Sunday, restart Tuesday"

**Privacy note:** This is the most sensitive pillar. Stored only on your server. Never sent to Claude unless you explicitly ask a health question, and only the prompt-relevant subset is sent (not your full log). Encrypted at rest.

### Cross-cutting — Ask Claude

A chat tab in the app with read access to all three pillars. You bring your own Claude API key (stored in the server's `.env`, never sent to the device after pairing).

**Sample questions** the system should handle out of the box:
- *"Should I push hard today?"* → reads today's recovery, sleep score, yesterday's strain, recent supplement doses; recommends Push / Maintain / Recover.
- *"Why was my sleep terrible last week?"* → cross-references late dinners (after 9 pm), high strain days, alcohol logs, supplement timing.
- *"Am I on track for OSCP by April?"* → reads `goal_milestones`, `goal_weekly_tracker`, `goal_output_log`; tells you whether your study hours and lab completion rate hit the target.
- *"Where did my budget break this month?"* → diffs current month vs. the targets in `budget-targets.json`, with the top 3 over-categories and their drivers.
- *"What supplements correlate with my best HRV days?"* → joins the supplement log with WHOOP HRV data, returns a ranked list.

**Implementation details:**
- **Prompt caching** is mandatory — the cross-pillar context block (recent 30 days of finance/health/goals) is large and stable. Cached blocks make multi-turn chat affordable.
- **Tool use** — Claude gets tools to query specific pillars (`finance_search_transactions`, `health_get_recovery_window`, `goals_check_milestone`) rather than dumping all data into the context. Cuts cost ~10×.
- **Conversation history** lives on your server, not Anthropic's.
- **Safety rail** — health recommendations always end with "this is not medical advice; for peptide/medication decisions consult your physician."

---

## 4. Architecture

```
┌────────────────────────────────────────────────────────────────────┐
│  iOS app  (SwiftUI, iOS 17+, Swift 6)                              │
│  - Tab bar: Home / Finance / Goals / Health / Ask                  │
│  - WidgetKit: net worth, today's recovery, next goal due           │
│  - Live Activities: budget-threshold alerts, daily brief checklist │
│  - Local Core Data cache for offline                               │
│  - APNs for push                                                   │
│  - Pairs to server via QR + device passkey on first launch         │
└────────────────────────────────────────────────────────────────────┘
                              │ HTTPS (Tailscale by default)
                              ▼
┌────────────────────────────────────────────────────────────────────┐
│  Northstar server  (Go, single binary; also Docker Compose)          │
│                                                                    │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │  API layer:  REST + WebSocket (per-pillar subscriptions)   │    │
│  └────────────────────────────────────────────────────────────┘    │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │  Sync workers:  cron-driven pulls from each source         │    │
│  │     • Actual:   every 15 min via @actual-app/api proxy     │    │
│  │     • WHOOP:    every 15 min via WHOOP v2 API              │    │
│  │     (Goals are native — no external sync)                  │    │
│  └────────────────────────────────────────────────────────────┘    │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │  Notification engine:  rules → APNs push relay             │    │
│  │     • Budget-threshold (50/75/90/100)                      │    │
│  │     • New-transaction stream                               │    │
│  │     • Daily brief / evening retro                          │    │
│  │     • Supplement reminders                                 │    │
│  │     • Health insight (recovery dropped 30+ pts)            │    │
│  │     • Anomaly (statistical outlier)                        │    │
│  └────────────────────────────────────────────────────────────┘    │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │  Claude bridge:  prompt caching + per-pillar tool defs     │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                    │
│  SQLite (one file) + Litestream → B2  (fin_* / goal_* / health_*  │
│                                        / notif_rules / ai_* / users)│
└────────────────────────────────────────────────────────────────────┘
        │                                 │                 │
        ▼                                 ▼                 ▼
   Actual API                          WHOOP API        Claude API
   (LAN/127)                           (cloud)          (cloud, user key)
```

### Why these choices

- **Go for the server, not Node/Python.** Single static binary deploys easily, cross-compiles cleanly, has the best Apple Push library, and is the right language for the "lots of concurrent sync workers" workload. You also flagged Go as a Year-2 language to learn — this is a natural place to invest those hours.
- **SwiftUI 5+, not React Native or Flutter.** WidgetKit, Live Activities, App Intents, and Charts all require native. The aesthetic target (Robinhood / WHOOP) requires native too — cross-platform never quite gets there. The cost is no Android v1; the upside is Android can be a Compose port later by someone else (open source).
- **SQLite + Litestream, not Postgres.** Single-user self-host with maybe 50k transactions/yr and a few thousand health rows fits in SQLite comfortably for years. Single-file ops + streaming backup to B2 is dramatically simpler than running and backing up Postgres. WAL mode handles the concurrent-sync-worker case fine at this scale. Reassess only if a future multi-user phase materializes.
- **REST + WebSockets, not GraphQL.** Smaller surface. The pillars are well-bounded — there's no client-driven query flexibility worth the schema overhead.
- **Tailscale by default, not public exposure.** Same posture as the finance tracker. Cloudflare Tunnel + Access is a Phase-7 add when you want a remote viewer (fiancée, partner, accountant).
- **APNs via your own server, not a SaaS push relay.** No third party reads your notification bodies.

---

## 5. Data model (sketch)

SQLite, single file at `~/.northstar/data.db`, with Litestream replicating to B2. Tables are flat (no Postgres-style schemas) but prefixed by pillar.

```sql
-- Core
CREATE TABLE users (id text PK, email text, claude_api_key_enc blob, settings_json text);
CREATE TABLE devices (id text PK, user_id text FK, apns_token text, paired_at int);

-- Finance (mirror — Actual is source of truth)
CREATE TABLE fin_accounts (actual_id text PK, name text, type text, balance_cents int);
CREATE TABLE fin_transactions (
  actual_id text PK, account_id text FK, date text, payee text,
  category text, amount_cents int, notes text, imported_at int);
CREATE TABLE fin_budget_targets (
  category text PK, monthly_cents int, rationale text,
  threshold_pcts text DEFAULT '[50,75,90,100]',  -- editable per-category
  push_enabled int DEFAULT 1);
CREATE TABLE fin_threshold_state (category text, month text, last_fired_pct int);

-- Goals (NATIVE — Northstar is source of truth; Notion only used for one-time personal seed)
CREATE TABLE goal_milestones (
  id text PK, title text, description_md text,
  due_date text, status text,                       -- pending / in_progress / done / archived
  flagship int DEFAULT 0,                            -- show in big hero card on Goals tab?
  display_order int,
  created_at int, updated_at int);
CREATE TABLE goal_daily_log (
  date text PK, items_json text,                     -- [{text, done, source, milestone_id?}]
  reflection_md text, streak_count int);
CREATE TABLE goal_weekly_tracker (
  week_of text PK, theme text,
  weekly_goals_json text, retro_md text);
CREATE TABLE goal_monthly (
  month text PK,                                     -- '2026-05'
  monthly_goals_json text, retro_md text);
CREATE TABLE goal_output_log (
  id text PK, date text, category text,              -- cve / blog / talk / tool / cert / pr / report
  title text, body_md text, url text);
CREATE TABLE goal_networking_log (
  id text PK, date text, person text,
  context text, next_action text, next_action_due text);
CREATE TABLE goal_reminders (
  id text PK, title text, body text,
  recurrence text,                                   -- cron-like: 'every monday 09:00' / 'last sunday of quarter'
  next_fires_at int, active int DEFAULT 1);

-- Health
CREATE TABLE health_recovery (date text PK, score int, hrv_ms real, rhr int, source text);
CREATE TABLE health_sleep (date text PK, duration_min int, score int, debt_min int);
CREATE TABLE health_strain (date text PK, score real, avg_hr int, max_hr int);
CREATE TABLE health_supplement_defs (
  id text PK, name text, dose text, schedule_json text,
  cycle_days_on int, cycle_days_off int,
  reminder_enabled int DEFAULT 1);
CREATE TABLE health_supplement_log (id text PK, def_id text FK, taken_at int, notes text);
CREATE TABLE health_medications (id text PK, name text, dose text, schedule_json text, prescribing_doc text);
CREATE TABLE health_mood_log (date text PK, mood int, energy int, focus int, notes text);

-- Notifications (user-editable, all surfaces)
CREATE TABLE notif_rules (
  id text PK,
  category text,                    -- 'purchase' / 'budget_threshold' / 'health_insight' / etc.
  enabled int DEFAULT 1,
  quiet_hours_start text,            -- '22:00'
  quiet_hours_end text,              -- '07:00'
  bypass_quiet int DEFAULT 0,        -- critical alerts bypass
  delivery text DEFAULT 'push',      -- push / live_activity / silent_badge
  max_per_day int DEFAULT 99);

-- AI conversations
CREATE TABLE ai_conversations (id text PK, started_at int, title text, pillar_scope text);
CREATE TABLE ai_messages (id text PK, conv_id text FK, role text, content_json text, tool_calls_json text);
```

---

## 5b. In-app editability — what you can change from the phone

Every preference, rule, and goal that matters is editable in the app — you should never need to SSH into the server or open a config file to tune your own setup. The patterns:

**Inline edits** — long-press / swipe / tap-to-edit on the rendered surface itself, not buried in Settings.

| Surface | Edit affordance | Persisted to |
|---|---|---|
| Budget category (Finance overview) | Tap → sheet → change monthly cap, threshold pcts, push toggle | `fin_budget_targets` |
| Threshold pcts per category | Multi-select chips (50/75/90/100, custom) | `fin_budget_targets.threshold_pcts` |
| Recategorize a transaction | Swipe-left on row → pick category → auto-creates rule for future identical merchants | Actual API write-back |
| Add / edit / archive a milestone | Plus button on Goals tab → sheet; tap row to edit | `goal_milestones` |
| Mark milestone as flagship | Toggle in edit sheet — drives the Goals-tab hero card | `goal_milestones.flagship` |
| Reorder goals | Drag handles in long-press mode | `goal_milestones.display_order` |
| Daily brief task | Tap to check; long-press → snooze / remove for today | `goal_daily_log.items_json` |
| Weekly theme + retro | Tap → edit sheet | `goal_weekly_tracker` |
| Output log entry (CVE, blog, talk) | Plus button on Goals tab → category picker → form | `goal_output_log` |
| Networking log entry | Plus button → person, context, next action | `goal_networking_log` |
| Recurring reminder | Settings → Reminders → cron-like schedule | `goal_reminders` |
| Add supplement / peptide | Plus button on Health tab → name, dose, schedule (daily / interval / cycle on/off) | `health_supplement_defs` |
| Log a supplement dose now | Tap row → confirm | `health_supplement_log` |
| Notification per-category rules | Settings → Notifications → per-category quiet hours, delivery, daily caps | `notif_rules` |
| Mute a notification for 24h / 7d | Long-press the lock-screen notification → choose | `notif_rules` (temp override) |
| Live Activity vs push vs silent | Per-category toggle | `notif_rules.delivery` |
| Claude API key | Settings → Connections → paste; never shown after save | `users.claude_api_key_enc` |
| Actual / WHOOP connections | Settings → Connections → URL/token/OAuth; test button | `users.settings_json` |
| Quiet hours global default | Settings → Notifications → top section | `users.settings_json` |
| Daily brief delivery time | Settings → Notifications → "Morning brief at: 07:30" | `users.settings_json` |
| Pillar enable/disable | Settings → Pillars → toggle Finance / Goals / Health | `users.settings_json` |

**Sheets, not full screens** — iOS bottom sheets (`UISheetPresentationController` / SwiftUI `.sheet`) at the medium detent for most edits. Keeps you in context — you see your dashboard partially behind the edit surface.

**Settings tab** — accessed via the avatar in the Home top-right (not a 6th tab — keeps the tab bar lean). Three sections: **Connections** (Actual / WHOOP / Claude — Goals has no external connection, it's native), **Notifications** (per-category rules, global quiet hours, brief time), **Pillars** (enable/disable, customize home dashboard ordering).

**Write-back nuance** — Finance edits route through Actual's API *first* (so your existing automation — `reclassify-transfers.js`, daily-summary push — keeps working), then update the local cache. Goals edits write directly to local SQLite (no external sync). Health edits (supplement defs / doses / mood / medications) also write directly to local — WHOOP-sourced rows (recovery, sleep, strain) are read-only from the user's perspective.

## 6. Notification strategy

Push categories with per-category controls:

| Category | Trigger | Default frequency | Quiet hours respected |
|---|---|---|---|
| `purchase` | New transaction lands in Actual | Realtime | Yes |
| `budget_threshold` | Category hits 50/75/90/100% | Once per threshold per month | Yes (except 100%) |
| `anomaly` | 3× median for known payee, or new merchant >$200 | Realtime | Yes |
| `daily_brief` | Cron, morning | Once/day | N/A |
| `evening_retro` | Cron, evening | Once/day | N/A |
| `supplement` | Scheduled dose | Per schedule | Configurable |
| `health_insight` | Recovery dropped 30+ pts, or HRV outlier | Once/day max | Yes |
| `goal_milestone` | Milestone status change (any device) | Realtime | Yes |
| `subscription_new` | New recurring charge detected | Once per detection | Yes |
| `weekly_retro` | Friday 9pm | Weekly | N/A |

Settings UI lets you mute, change quiet hours, or change delivery (push / Live Activity / silent badge).

Critical alerts (anomaly, 100% budget, very low recovery) bypass quiet hours.

---

## 7. Privacy & security

| Concern | Mitigation |
|---|---|
| Bank credentials | Never touch Northstar. Stay in SimpleFIN / Actual. |
| Health data leaving the server | Only on explicit AI query, only the prompt-relevant subset, logged. |
| Claude API key | Stored encrypted at rest with a key derived from the server master passphrase. Never sent to the iOS device. |
| iOS device theft | Device is paired with a passkey; revoking from server invalidates immediately. App requires Face ID on cold launch. |
| Server compromise | Backups (restic to B2) encrypted with a separate key. Same posture you already use for the finance tracker. |
| Open source supply chain | All dependencies pinned. CI runs `govulncheck` + `npm audit` on PRs. No telemetry libraries; CI enforces the empty-allowlist. |
| Third-party SDKs | Zero in the iOS app. No Firebase, no analytics, no crash reporting that phones home. Crash logs are local + sendable as a file. |

---

## 8. Open-source plan

**License:** MIT for both the server and the iOS app. Northstar is a self-hosted personal app — the SaaS-fork-without-share-back risk that AGPL guards against doesn't really exist here (every user runs their own instance). MIT keeps contributor friction low, lets corporate contributors participate, and lets anyone ship a branded build for friends/family without legal review.

**Repo layout:**
```
github.com/<you>/northstar-server      MIT     Go server, SQLite + Litestream, Docker compose
github.com/<you>/northstar-ios         MIT     SwiftUI app, fastlane pipeline
github.com/<you>/northstar-docs        CC-BY   Astro/Starlight docs site
github.com/<you>/northstar-tools       MIT     CLI installer, migration tools, sample data
```

**Distribution:**
- **iOS:** TestFlight beta for early users (90-day cap). Public App Store at v1.0 ($99/yr Apple Developer Program — paid out of pocket).
- **Self-signed sideload path** documented (AltStore / Sidequest) for users who don't want to use TestFlight.
- **Server:** Docker Hub image + `docker compose up` one-liner. Also a one-shot install script for bare-metal Debian/Ubuntu.

**Community:**
- GitHub Discussions for support
- Public roadmap on GitHub Projects
- No paid tier. Optional GitHub Sponsors for those who want to give back.
- Code of Conduct + CONTRIBUTING.md from the jump

**Marketing once it's ready:**
- HN: "Show HN: Northstar — a self-hosted alternative to Mint + Notion + WHOOP, in one iOS app"
- /r/selfhosted, /r/personalfinance, /r/whoop, /r/quantifiedself
- A talk at a conference once you're 6+ months in (this folds into the [career roadmap Output Log](#))

---

## 9. Phased build plan

Total estimated calendar time: **~10 weeks at 8–10 hrs/week** alongside the day job. Compresses to ~4–5 weeks if you dedicate weekends.

### Phase 0 — Foundation (week 1)
**Outcome:** dev environment runs end-to-end with a hello-world request.
- Repo scaffolds: `northstar-server` (Go module, SQLite via `modernc.org/sqlite` pure-Go driver, migrations via `goose`, REST framework `chi`, Litestream sidecar in compose)
- `northstar-ios` (SwiftUI app, fastlane, TestFlight pipeline)
- Docker compose for local dev: `postgres + northstar-server + ngrok` (so the phone hits a real URL)
- Device pairing flow: QR code on server admin web page → iOS scans → device exchanges keys
- CI on GitHub Actions: lint + test + build + deploy nightly to your home server

### Phase 1 — Finance ingest (week 2)
**Outcome:** Northstar home screen shows live spend-by-category mirroring Actual.
- Sync worker pulls from Actual via `@actual-app/api` (run as a small Node sidecar called from Go via subprocess, since the API is JS-only)
- Schema mirrored into `fin_*` tables
- iOS Finance tab: spend-by-category rings, recent transactions list, account balances
- WidgetKit: net-worth widget
- WebSocket fanout: new transaction in Actual → push to iOS → cell appears at top of list

### Phase 2 — Budget alerts + push (week 3)
**Outcome:** APNs push notifications fire when budgets cross thresholds and on every new transaction.
- Threshold engine (port the logic from `daily-summary.js`)
- APNs key + cert wired
- Notification settings UI (mute / quiet hours / per-category)
- Live Activity: when you cross a threshold, dynamic-island shows progress until you tap

### Phase 3 — Goals pillar (weeks 4–5)
**Outcome:** Native goals work end-to-end; your personal Notion data is seeded in; daily brief notification fires.
- Native goal tables migrated (`goal_milestones`, `goal_daily_log`, `goal_weekly_tracker`, `goal_monthly`, `goal_output_log`, `goal_networking_log`, `goal_reminders`)
- REST endpoints + CRUD UI: add / edit / archive milestone, log output entry, add networking entry, set reminder
- Goals tab: flagship hero card, short-term list, long-term horizon, today's tasks
- Daily brief composer: pulls today's reminders + milestones due in next 7 days + items rolled over from yesterday's daily log
- Tap-to-check daily brief items; long-press to snooze
- Streak counter (consecutive days with a non-empty daily log)
- **`northstar-tools import-notion` CLI** — one-shot, parses Notion API into the native tables for personal seed. Ships in the tools repo, not the app.
- Push trigger: milestone status change, daily brief at morning, evening retro nudge

### Phase 4 — Health pillar (weeks 6–7)
**Outcome:** WHOOP data flows in; supplement logger works; recovery dial renders.
- WHOOP OAuth flow (one-time setup on server)
- Sync workers: recovery, sleep, strain pulled every 15 min
- Health tab: today's recovery dial, sleep ring, strain bar, weekly trend
- Supplement / peptide CRUD + scheduled doses
- Push reminders for scheduled doses
- Correlation explorer (basic v1: choose a supplement, see HRV before/after)

### Phase 5 — Ask Claude (week 8)
**Outcome:** Chat tab works against all 3 pillars with tool calls.
- Tool definitions: `finance_*`, `goals_*`, `health_*`
- Prompt caching for the system prompt + the rolling 30-day pillar summary
- Conversation persistence (server-side, encrypted)
- Streaming responses to the iOS chat UI
- Quick-prompts row: "Should I push today?" / "Am I on track?" / "Budget status"

### Phase 6 — Polish + open source (weeks 9–10)
**Outcome:** v1 launched.
- Docs site (Astro/Starlight): install guide, API docs, contributing guide
- One-shot installer (`curl -sSL get.northstar.dev | bash`)
- Public TestFlight link
- App Store submission
- Show HN draft + screenshots
- Public GitHub roadmap

### Phase 7+ — Roadmap (post-v1)
- **HealthKit ingest** — Apple Watch HR, steps, workouts as a non-WHOOP backup
- **Apple Watch app** — recovery glance, supplement quick-log
- **Investment portfolio view** — Robinhood + brokerage tax-lot tracking
- **Habit tracker** — Streaks-style daily habits beyond `goal_milestones` (cycle-tracking, etc.)
- **Android app** — Jetpack Compose port, community-led if possible
- **Multi-user** — fiancée gets her own pillar without seeing yours; sharable summary widgets
- **Cash-flow forecast** — port `cashflow-forecast.js` to a future-prediction view
- **Receipt OCR** — quick-add expense by photo

---

## 10. Cost estimate

| Line item | Monthly | Annual |
|---|---|---|
| Server hosting (re-uses existing home server) | $0 | $0 |
| Apple Developer Program (yourself) | — | $99 |
| Claude API (typical usage, ~50 chat queries / mo with caching) | $1–3 | $12–36 |
| APNs | $0 | $0 |
| Domain (optional, for the docs site) | ~$1 | $12 |
| **Total for you** | **~$3** | **~$135** |

For other users (open source consumers):
| | |
|---|---|
| Server hosting | $0 if they self-host on existing hardware, ~$5/mo VPS otherwise |
| Apple Developer Program | Not needed unless they want to ship their own App Store build |
| Claude API | Bring-your-own; usage depends on how much they chat |

This is intentionally cheap. The product is "your life dashboard," not a subscription business.

---

## 11. Risks & open questions

| Risk | Mitigation |
|---|---|
| **WHOOP API access tier limits.** WHOOP requires developer-account approval for the public API. | Apply on day 1 of Phase 4. Backup: HealthKit ingest, which works without external approval. |
| **TestFlight 90-day expiry** for OSS users without the App Store build. | Document the AltStore sideload path. Ship to App Store within 6 months. |
| **Personal Notion seed-import drift.** Until you finish migrating off Notion, your two systems will diverge. | Run the importer in `--dry-run` mode first, validate the diff, then a single cutover pass. After cutover, archive the Notion page (read-only) so muscle memory doesn't pull you back. |
| **Claude API cost creep** if power users chat a lot. | Prompt caching, tool calls (not full-context dumps), per-user monthly budget cap with a soft warning. |
| **App Store rejection** for "private use" apps. | Submit as a personal-finance/health app — there's precedent (Copilot, Lunch Money, RIZE). Worst case: ship via TestFlight + sideload only. |
| **HIPAA / health data legality.** | Northstar is personal-use; not a covered entity. The README states this clearly and ships under MIT with no warranty. Users handle their own peptide/medication legality. |
| **Maintainer burnout** (the OSS-project killer). | Public roadmap caps scope. Plain "I'm not accepting feature requests on these axes" list in CONTRIBUTING. MIT means you can walk away and people can fork — even ship their own branded build — without legal friction. |
| **Apple kills the API tier you depend on** (e.g. WidgetKit changes). | Pillars are independent — only the affected feature breaks, not the app. |
| **You stop using it.** This is the most likely failure mode for a personal project. | Build the daily-brief push FIRST so you get a daily nudge from your own product. If you don't use the daily brief for 2 weeks, kill the project. |

### Locked decisions (recap, 2026-05-12)

All five kickoff questions are answered — see the table at the top of this doc.

### Still-open mid-build calls

1. **Daily-brief composition logic.** Pull from `goal_reminders` only, or also include milestones due in next 7 days and yesterday's incomplete daily-log items? Tune after week 1 of usage.
2. **WHOOP API tier.** Apply day 1 of Phase 4; if the developer-account approval lags, fall back to HealthKit ingest as a temporary source.
3. **Supplement reminder timing strategy.** Fixed time-of-day vs. anchored-to-event (e.g. "with first meal")? Start with fixed; add event-anchored in Phase 7.

---

## 12. Out of scope for v1

| Excluded | Why |
|---|---|
| Android | SwiftUI required for the design target; Android can come later via OSS contribution. |
| Multi-user / family accounts | Single-tenant by design. Multi-user is Phase 7+. |
| Apple Watch app | Phase 7. Live Activity covers most of the watch surface for v1. |
| HealthKit ingest | WHOOP is the primary data source. HealthKit is Phase 7 redundancy. |
| Investment performance analytics | Actual handles balances; portfolio analytics deferred. |
| Receipt OCR | "Tap to add expense from photo" deferred to Phase 7. |
| Tax integration | Manual export of `tax-rollup.js` data covers v1 use case. |
| Web app | Phone is the surface. Web admin exists only for setup. |
| Encrypted E2E sync between multiple devices | One device per user, v1. Multi-device sync is Phase 7. |

---

## 13. Why this plan, in one sentence

You already have the data and the pipelines — Northstar is the **iOS app that turns those three disconnected pipelines into a single decision-support surface**, and it's a substantial Swift + Go portfolio piece that ties to your cybersec engineering trajectory while also being something you'll actually use every day.
