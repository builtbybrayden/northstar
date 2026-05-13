# Security Policy

Northstar is a self-hosted project: every instance is run by one person on
their own hardware, with their own data. There is no production service to
attack, no shared backend, and no user database to leak. That said, bugs in
the code can still expose individual operators to risk — e.g., a pairing flow
that lets an unauthenticated request mint a device token, or a sidecar that
leaks environment variables. Reports for issues like that are very welcome.

## Reporting a vulnerability

**Do not open a public GitHub issue.** Email instead:

**brayden.arnold@gmail.com** — subject line `Northstar security:`

PGP is not currently published; if you need encrypted transport, mention it
in your first message and I'll set up a key exchange.

Please include:

- The affected component (`server`, `actual-sidecar`, `whoop-sidecar`, `ios`, `infra`, `docs-site`)
- The commit SHA you tested against
- Reproduction steps or a proof-of-concept
- The impact you believe a successful exploit would have

I'll acknowledge within **5 business days**. For valid issues I'll aim to
have a fix or mitigation in `main` within **30 days**, and will credit the
reporter in the commit message and release notes unless you prefer anonymity.

## Scope

In scope:

- The Go server (`server/`) — auth, pairing, REST handlers, SSE streams, scheduler
- Both Node sidecars (`tools/actual-sidecar/`, `tools/whoop-sidecar/`) — OAuth flows, env-var handling, request forwarding
- The iOS app (`ios/`) — Keychain usage, pairing flow, deep links
- Docker compose and Litestream configuration (`infra/`) — exposed ports, volume permissions, backup encryption
- One-shot installers (`scripts/install.*`) — generated-secret strength, file permissions, idempotency edge cases

Out of scope:

- Vulnerabilities in upstream dependencies that the project tracks but does not maintain (Actual, WHOOP API, Anthropic SDK, chi, Starlight, etc.) — report those upstream
- Attacks that require local code-execution on the operator's machine (Northstar trusts the host)
- DoS against a self-hosted instance from a network attacker who already has LAN access (Northstar is designed for Tailscale-only or LAN-only deployment)
- Social engineering of the operator into pasting a malicious bearer token

## Hardening guidance

Recommended deployment posture, summarized below and discussed in more
detail in [`docs-site/src/content/docs/operating/threat-model.md`](./docs-site/src/content/docs/operating/threat-model.md):

- Bind the server to a Tailnet interface, not `0.0.0.0`
- Set `NORTHSTAR_ADMIN_TOKEN` before exposing `/api/pair/initiate`
- Keep `.env` in `~/.northstar/` with `chmod 600`
- Run Litestream against an encrypted B2 bucket if backups are enabled
- Update Docker images monthly

## No bounty

This is a personal-scope project with zero revenue, so there is no monetary
bounty. Credit and a published thank-you are the only compensation on offer.
If that's a dealbreaker, that's fair — feel free to take the finding elsewhere.

## Past advisories

None yet. Once we publish the first one it'll live under
[GitHub Security Advisories](https://github.com/builtbybrayden/northstar/security/advisories).
