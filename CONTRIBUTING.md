# Contributing to Northstar

Thanks for your interest! Northstar is a personal-life-OS project — small,
opinionated, and explicitly **not** trying to be everything to everyone.
That means some contributions are very welcome and some aren't a fit. This
doc tries to be honest about both.

## What we want

- Bug fixes with a clear repro
- Compatibility with new versions of upstream sources (Actual, WHOOP, Anthropic)
- New pillar-internal features that don't expand the pillar count
- Docs improvements — typos, clarifications, missing env vars
- Test coverage on the Go server
- Better mock-mode fidelity

## What we probably don't want

- New pillars (the three-pillar shape is intentional)
- Telemetry, analytics, or "send anonymized usage" anything
- Cloud-hosted SaaS variants — the project is self-hosted by design
- Cross-platform mobile rewrites (Android can come as a separate community port)
- New external dependencies in the iOS app — we ship zero third-party SDKs and want to keep it that way

If you're not sure whether your idea fits, open a **Pillar question** issue
before writing code.

## Reporting a bug

1. Reproduce against `main`.
2. Open a **Bug** issue with: what you did, what you expected, what happened, server logs around the failure, and `go version` / `node --version` / iOS version if relevant.
3. If it's a security bug, **do not** open an issue — see [SECURITY.md](./SECURITY.md).

## Sending a PR

1. Fork + branch (`builtbybrayden/northstar`).
2. Run the tests (`cd server && go test ./...`).
3. For iOS changes: confirm Xcode builds clean and the affected pillar still functions in the simulator.
4. Open a PR against `main`. Include a screenshot if there's a UI change.
5. CI runs on every push. Green CI is required to merge.

## Style

Per-language conventions are in the [Style guide](https://github.com/builtbybrayden/northstar/blob/main/docs-site/src/content/docs/contributing/style.md).
Highlights:

- Go: `gofmt` + `goimports`, error wrapping, no global state
- Swift: `APIClient` for networking, models match server DTOs exactly
- JS sidecars: ESM only, mock mode is the default code path
- Commits: imperative subject, body explains *why*

## Commit hygiene

- One concern per commit
- Reference the issue you're fixing: `Fix BPC-157 cycle off-by-one (#42)`
- Squash review fixups before merge

## License

By contributing, you agree your contribution will be licensed under MIT.
There's no CLA — your `git` author is the record.

## Code of conduct

This project follows the [Contributor Covenant 2.1](./CODE_OF_CONDUCT.md).
The short version: be kind, assume good faith, leave the project a slightly
better place than you found it.
