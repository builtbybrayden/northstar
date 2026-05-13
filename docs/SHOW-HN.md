# Show HN draft

_Status: draft. Don't post until Apple Developer enrollment lands and TestFlight
link is live — the value of a Show HN drops sharply if "how do I actually run
this" requires AltStore sideloading._

---

## Title options

Pick one. HN titles are capped at 80 characters.

1. **Show HN: Northstar — self-hosted personal life-OS (finance + goals + health + AI)**  _(78)_
2. **Show HN: Northstar – a self-hosted dashboard tying Actual, WHOOP, and Claude**  _(74)_
3. **Show HN: A local-first iOS dashboard that lets Claude read across your data**  _(75)_

I'd default to #1 — most descriptive, gets the local-first angle in early.

## Body

```
Hi HN — Northstar is a personal-life dashboard I've been building for myself
that ties three "pillars" together — money, goals, and health — and puts a
Claude-backed assistant on top that can read across all of them via tool
use.

It is explicitly *not* a product. It's a self-hosted Go server + SwiftUI
iOS app + a couple of Node sidecars, runs on your hardware, ships zero
telemetry, and has no SaaS variant on the roadmap. I'm posting it because
I've gotten enough use out of it that the local-first shape feels like
something other people might want too.

Architecture in one paragraph:

- A Go server (chi + SQLite + Litestream) is the system of record. It runs
  on whatever you point it at — Pi, NAS, a VPS, doesn't matter — and the
  iOS app pairs to it with a 6-digit code over Tailscale or LAN.
- Two Node sidecars talk to providers I don't control: Actual Budget for
  finance, WHOOP for biometrics. Both have mock modes that return realistic
  data so the whole stack works without any external credentials.
- The "Ask" pillar gives Claude (via the Anthropic API in real mode, or a
  keyword-routed mock in dev mode) ten tools that read across the database:
  finance_summary, goals_brief, health_today, etc. Prompt caching is wired
  end-to-end so the system prompt + tool definitions stay cached.
- Each pillar has its own notification engine path: budget thresholds fire
  when you cross 50/90/100% of a category cap, recovery-drop fires when
  your WHOOP score drops 30+ points day-over-day, milestone reminders fire
  via a 5-field cron parser.

What it doesn't do:

- No cloud sync, no multi-user, no plugin system, no Android client
- No third-party SDKs in the iOS app (everything goes through one APIClient)
- No telemetry of any kind
- No new pillars beyond the three plus Ask — domain-specific stuff lives
  inside the existing pillars

What surprised me building it:

- Mock mode is a huge productivity win. I write 90% of features against the
  mock sidecar response and only flip to real APIs when I'm ready to ship.
  Recommended for any "we depend on a flaky third-party API" project.
- Anthropic's prompt caching is well worth the wire-up cost. Tool defs are
  about 2.5K tokens; once they're cached, every follow-up message in a
  conversation pays roughly nothing for them.
- SwiftUI + a strict APIClient layer means the iOS app stays small. The
  whole thing is under 6K lines of Swift.

Stack: Go 1.23, chi, modernc.org/sqlite, goose, SwiftUI iOS 17+, Node 22,
Astro + Starlight for the docs site, Docker compose for the deployment
unit. MIT licensed. Single-binary install via `scripts/install.sh` or
`scripts/install.ps1`.

Repo: https://github.com/builtbybrayden/northstar
Docs: https://northstar-docs.pages.dev  (update with real URL before posting)
TestFlight: <update with real link before posting>

Happy to answer questions about the prompt-caching setup, the
mock-mode-first dev workflow, or why I picked SQLite + Litestream over
Postgres for a single-operator system.
```

## Pre-post checklist

- [ ] Apple Developer Program enrolled and a TestFlight build is live
- [ ] Real-mode smoke tests pass for at least one of Actual / WHOOP (so the
      claim "works against real APIs" isn't aspirational)
- [ ] Docs site deployed to a stable URL (Cloudflare Pages / Netlify / GH Pages)
- [ ] `scripts/install.sh` tested on a fresh Ubuntu 24.04 VM end-to-end
- [ ] Repo README screenshot is current, not the original from 2026-05-12
- [ ] Verify SECURITY.md inbox is being monitored (responses within 5 days promised)
- [ ] Dry-read the post out loud — if any sentence sounds like a product
      page, rewrite it

## Post-post first hour

HN's algorithm rewards staying on top of replies during the first 60–90
minutes. Block the slot.

- Engage with every top-level comment, even short ones
- Don't argue about scope — if someone wants "feature X", point at ROADMAP.md
  and the "Won't do" list, no debate
- Edits are fine but keep them small; large edits look like crisis management

## After

If it stays on the front page > 2h, expect:

- A wave of GitHub stars (mostly noise, ignore)
- Some real issues filed against the installer on platforms I didn't test
- A small number of thoughtful pillar-question issues — those are the
  signal, prioritize replying to them

If it doesn't make the front page, that's also fine. Re-post is allowed
after a meaningful change. Don't burn a re-post on the same artifact.
