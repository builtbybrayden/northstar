# Privacy Policy — Northstar

_Last updated: 2026-05-12_

Northstar is a self-hosted personal life-dashboard built and operated by a
single individual for their own use. There is no cloud service, no shared
backend, and no third-party data collection. This document exists so that
upstream providers (WHOOP, Actual, Notion, etc.) that require a published
privacy URL have one to link to.

## Who runs Northstar

Northstar is operated by **Brayden Arnold** as a personal project. The
codebase is open source at https://github.com/builtbybrayden/northstar. Each
person who installs Northstar runs their own private instance on their own
hardware — there is no central operator.

Questions: brayden.arnold@gmail.com

## What data Northstar handles

When you (the operator-user) connect a provider, Northstar pulls data from
that provider's API and stores it locally on the machine running Northstar.
The following categories may be handled:

- **WHOOP**: recovery scores, sleep duration and stages, strain, heart rate
  variability, resting heart rate, workout records, and basic profile fields
  (first name, height, weight) — pulled from the WHOOP v2 REST API.
- **Actual Budget**: accounts, account balances, transactions (date, payee,
  category, amount, notes), and budget targets — pulled from your own Actual
  server via the official Actual API.
- **Notion**: page titles, dates, and other top-level properties from
  databases you explicitly grant the integration access to.
- **User-entered data**: goals, daily reflections, supplement schedules, chat
  conversations with the built-in AI assistant, and notification preferences
  that you type into the app.

## What Northstar does not collect

- No analytics, telemetry, crash reports, or usage tracking is sent anywhere.
- No advertising identifiers are read or stored.
- No personal data is transmitted to the developer (Brayden Arnold) or to any
  third party other than the providers you explicitly connect, plus the
  Anthropic API if you choose to enable the AI assistant in real mode.

## Where data lives

All Northstar data is stored locally on the machine running the server
(SQLite database file). Provider OAuth tokens and API secrets are stored in a
gitignored `.env` file on that same machine. Nothing is uploaded to any
Northstar-operated server because there is no Northstar-operated server.

If you enable the AI assistant in real mode, the contents of your messages
plus any tool results the model fetches are sent to Anthropic's API in order
to generate a response. Anthropic's privacy policy applies to that
transmission: https://www.anthropic.com/legal/privacy.

## How data is used

Data is used exclusively to render dashboards, fire reminders, and answer
your questions inside the app you are running. It is not shared, sold,
licensed, or analyzed for any other purpose.

## How long data is kept

Data is retained on your device until you delete it. Northstar does not have
a retention timer.

## How to delete your data

- **All of it**: stop the Northstar server and delete the SQLite database
  file (`server/data/northstar.db`) and the `.env` file in `~/.northstar/`.
- **Just one provider**: revoke the relevant token in that provider's
  dashboard, remove its entries from `.env`, and remove the rows from
  SQLite using a SQL client.
- **AI conversations**: delete them from inside the app via the Ask tab.

## Children

Northstar is not intended for use by anyone under 13. The developer does not
knowingly collect data from children.

## Changes

If this policy materially changes, the updated date at the top of this file
will move forward and the change will be visible in the git history of the
repository.

## Provider compliance

This policy is published to satisfy provider requirements for OAuth app
registration (e.g., WHOOP's developer review, Notion integration setup).
Each provider's own terms and privacy policy continue to govern the
provider-side handling of your data.
