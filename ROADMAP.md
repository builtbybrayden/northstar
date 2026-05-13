# Northstar Roadmap

_Last updated: 2026-05-13_

This roadmap reflects what I'm actually planning to ship. It is not a
promise, a SLA, or a feature vote. Northstar is a personal project; if
something on the "won't do" list matters to you, fork it.

## Shipped

**Phases 0–5 (server foundation + three pillars + AI brain)** — pairing,
finance pillar (Actual sidecar + budgets + notifications), goals pillar
(milestones + brief + planner + reminders), health pillar (WHOOP sidecar
+ supplements + detectors), Ask pillar (Claude tool-use over the
cross-pillar surface).

**Ops polish wave (2026-05-12)** — admin-token gate, initial-sync purchase
suppression, supplement reminder UI, per-category quiet hours, sleep +
overreach detectors, SSE live notifications, Anthropic token telemetry,
chat cancel + rename + scope picker + tool-error UI.

**Phase 6 partial (2026-05-12 → 2026-05-13)** — Astro/Starlight docs site
(22 pages), one-shot installers (`scripts/install.{sh,ps1}`),
CONTRIBUTING.md, PRIVACY.md, SECURITY.md, ROADMAP.md, GitHub
issue/PR templates, Show HN draft.

## Next up

In rough priority order. Items higher on the list are likely to land sooner.

1. **Apple Developer Program enrollment** — unblocks TestFlight, App Store
   submission, the APNs production sender, Live Activity for budget
   thresholds, and a lock-screen Widget for net worth.
2. **Real-mode smoke tests** — verify the three sidecars against real
   credentials. WHOOP is gated on dev-account review; Anthropic and Actual
   are gated on me just sitting down and running them.
3. **End-to-end install dogfood on a fresh VM** — confirm the one-shot
   installer works on Ubuntu LTS and Windows 11 without me already having
   half the tooling installed.

## Phase 7 — partially shipped

Shipped 2026-05-13:

- ✅ **Cash-flow forecast** — `/api/finance/forecast`; trailing-6-month
  recurring detection (cohort needs ≥3 distinct months at ~same payee/amount),
  daily discretionary spend rate, 30/60/90/180-day simulation, Charts line
  view with recurring inflows/outflows annotated as milestones
- ✅ **Habit tracker** — `goal_habits` + `goal_habit_log` tables, full CRUD,
  toggle-on-empty-PUT semantics, 90-day GitHub-style heatmap on iOS, streak +
  last-30 stats per habit
- ✅ **Investment portfolio view** — `/api/finance/investments` groups
  off-budget accounts (retirement / crypto / real estate / equity / cash /
  other) by name heuristic, per-account percentage, asset-class breakdown bar.
  No new sidecar — uses what Actual already syncs.

Backburnered / deferred / dropped:

- ⏸ **HealthKit ingest** — explicitly backburnered. WHOOP is the only health
  source for now.
- ⏸ **Apple Watch glance** — blocked on Apple Developer Program enrollment
- ❌ **Receipt OCR** — dropped from scope; user already gets transactions via
  Actual's bank connection

## Phase 7.5 — maybe

- **Daily balance snapshots** — `fin_account_balance_history` table so
  Investments + Forecast can chart historical drift, not just current state
- **Forecast-driven notifications** — fire a `forecast_warning` when the
  projection crosses zero or a configurable cash floor
- **HealthKit ingest** if WHOOP gets retired or you want a non-WHOOP path

## Phase 8 — maybe

- **Android (Compose) port** — community-driven, separate repo
- **Multi-user / family accounts** — opens enough complexity that I'd want
  someone else to drive it

## Won't do

- **Cloud-hosted SaaS variant.** The whole point is local-first. If you want
  a hosted variant, you can run Northstar on a VPS yourself.
- **Telemetry, analytics, crash reporting.** Ever.
- **New pillars.** Finance + Goals + Health + Ask is the shape. Anything
  domain-specific (fitness coaching, journaling, recipe planning) lives
  inside Goals or Health, not as its own tab.
- **Third-party SDKs in the iOS app.** Networking goes through `APIClient`,
  everything else is stdlib + SwiftUI.
- **Web frontend.** The mockups are illustrative only. iOS is the client.
- **Plugin system.** The cost of a stable plugin API for a single-operator
  app is too high. Fork instead.

## How dates work here

I avoid putting calendar dates on roadmap items because I've watched too
many of my own estimates slip. The order is real; the timing is not.

## Why this exists

So I have a single place to point people who ask "are you going to add X?"
and so I have a single place to point myself when scope creep tries to
pull a Phase 8 idea into a Phase 7 slot.
