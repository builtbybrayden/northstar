---
title: Health
description: Mirror WHOOP biometrics alongside a manual supplement / peptide / medication logger.
---

The Health pillar mirrors WHOOP recovery, sleep, and strain into local SQLite,
then sits a manual supplement / peptide / medication logger on top so you can
correlate stack with biometrics.

> **Not medical advice.** Northstar surfaces your own data and computes
> heuristics over it. Confirm any peptide / medication change with your
> prescriber.

## What you get

- **Recovery dial** — WHOOP-style 0–100% gauge, color-coded green ≥ 70 / yellow ≥ 40 / red < 40
- **Today's verdict** — push / maintain / recover, plus a strain goal range
- **Metric grid** — HRV (ms), RHR (bpm), sleep duration + score, strain — each with day-over-day delta
- **7-day recovery spark** — color per day
- **Supplement / peptide / medication list** — cycle awareness (e.g. 4-on / 2-off), dose log, tap-to-log-today
- **Recovery-drop notification** — fires once when today's recovery is ≥ 30 points below yesterday

## How it works

```
WHOOP cloud
         │ WHOOP v2 API (OAuth2 w/ refresh-token caching)
         ▼
   whoop-sidecar         (Node, mock-by-default)
         │ HTTP
         ▼
   northstar-server      Go sync worker, every 15 min
         │ UPSERT
         ▼
   SQLite  (health_recovery / health_sleep / health_strain / health_supplement_* / health_mood_log)
```

## Mock mode (default)

The WHOOP sidecar ships with a 14-day rolling window of synthesized data:

- Today: 84% recovery, 78ms HRV, 51 RHR, 7h42m sleep score 88, strain 16.0 → verdict **push**
- Day 3 dip to 28% recovery to exercise the recovery-drop detector path
- Sleep / strain values internally correlated with recovery (no obviously fake numbers)

## Real mode

```ini
NORTHSTAR_HEALTH_SIDECAR_MODE=real
WHOOP_CLIENT_ID=…
WHOOP_CLIENT_SECRET=…
WHOOP_REFRESH_TOKEN=…
```

You'll need a developer account approved on the WHOOP API. Apply early —
approval has historically lagged.

## Supplement logger

Add a supplement / peptide / medication from the Health tab's **+** button.
Each definition supports:

- Name + dose (e.g. `500 mcg`, `5 mg`)
- Category — supplement / peptide / medication
- Optional cycle: X days on / Y days off
- Reminder toggle
- Prescribing doctor (medication only)
- Notes

Tap a row's circle to log a dose at the current time. Tap the row body to edit.
Doses appear in the recovery query window so Claude can correlate.
