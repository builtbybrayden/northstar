---
title: Style guide
description: How code, commits, and docs should look in the Northstar tree.
---

## Go (server)

- Run `gofmt` / `goimports` — CI enforces. No exceptions.
- Standard project layout: handlers + types in one package per pillar; tests next to the code they exercise.
- Errors: wrap with `fmt.Errorf("verb noun: %w", err)`. No bare returns.
- HTTP handlers: take `(w, r)`, return nothing; helpers `writeJSON` / `writeErr` are in each package's `handlers.go`.
- No global state. Every dependency (DB, sender, scheduler) is wired up in `cmd/northstar-server/main.go` and passed in.
- Empty slices serialize as `[]` not `null` — initialize `out := []Foo{}`, not `var out []Foo`.

## Swift (iOS)

- SwiftUI views are PascalCase and live in `Northstar/<Pillar>/`.
- Networking calls go through `APIClient` — no direct `URLSession` in views.
- Models matching server DTOs live in `Northstar/Networking/*Models.swift`. Field names match server JSON exactly (snake_case allowed in property names — `Codable` doesn't require a converter, fields like `started_at: Int64` are fine).
- No third-party SwiftPM packages in v1 — keep the dependency tree at zero.

## JavaScript (sidecars + tools)

- ESM (`"type": "module"`) — no CommonJS.
- Single-file servers — these are small enough that one `server.js` is the right answer.
- Mock mode is the default — every code path that touches a real third-party must be gated behind `SIDECAR_MODE === "real"`.

## Commits

- Imperative subject, ≤ 72 chars: "Add health pillar recovery dial".
- Body wraps at 80 and explains *why*. The diff explains *what*.
- One concern per commit. Squashing during PR review is fine.

## Docs

- These pages are Markdown / MDX inside `docs-site/src/content/docs/`.
- Frontmatter: title + description, both ≤ 110 chars.
- Code blocks must be runnable verbatim — fake-looking placeholders break the trust users put in your install instructions.
- Cross-link liberally: `[blah](/pillars/finance/)`. Starlight validates internal links at build time.

## Conventions specific to Northstar

- The word "pillar" is reserved for Finance / Goals / Health (+ the cross-cutting Ask).
- The word "sidecar" is reserved for the two Node processes (Actual + WHOOP).
- Money is always in cents, integers — never floats. The presentation layer divides by 100.
- Timestamps are unix seconds on the wire, `time.Time` in Go, `Int64` (then `Date(timeIntervalSince1970:)`) in Swift.
