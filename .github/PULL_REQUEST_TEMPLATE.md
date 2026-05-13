<!--
Thanks for sending a PR. A few quick notes:

- For new features, please open a "Pillar question" issue first if you haven't.
- One concern per PR is better than a grab-bag.
- CI must be green to merge.
-->

## What this changes

<!-- One or two sentences. -->

## Why

<!-- Link the issue if there is one (`Fixes #123`), or explain the motivation. -->

## How to verify

<!--
Reviewer-facing steps. Examples:
- `cd server && go test ./...`
- `curl -X PATCH http://localhost:8080/api/finance/budget-targets/Restaurants -d '{"cap":300}'`
- Open the iOS simulator, tap Finance → category bar, change cap, save.
-->

## Scope check

- [ ] One concern per PR
- [ ] Tests pass locally (`cd server && go test ./...`)
- [ ] If iOS changed: Xcode builds clean and the affected pillar renders
- [ ] If a new external dependency: justified in the PR description
- [ ] No new telemetry, analytics, or "phone-home" code
- [ ] No new third-party SDK in `ios/`

## Screenshots / output

<!-- For UI changes, include a before/after screenshot. For server changes, paste the relevant test output or `curl` response. -->
