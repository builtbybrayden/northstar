// Real WHOOP mode — wraps the v2 REST API with OAuth2 refresh-token handling.
// Inert until you set the env vars. The Go server stays the same either way.
//
// Required env:
//   WHOOP_CLIENT_ID
//   WHOOP_CLIENT_SECRET
//   WHOOP_REFRESH_TOKEN   (obtained out-of-band via the OAuth grant)
//
// WHOOP issues short-lived access tokens (~1h) AND ROTATES refresh tokens
// on every refresh — each successful refresh response carries a new
// refresh_token that must be used next time. We persist it to disk so a
// container restart still has a valid grant.

import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { homedir } from 'node:os';
import { join, dirname } from 'node:path';

const REFRESH_FILE = join(homedir(), '.northstar', 'whoop-refresh-token.txt');

let accessToken = null;
let accessTokenExpires = 0;
let currentRefresh = null; // in-memory current refresh token (rotates)

function loadCurrentRefresh() {
  if (currentRefresh) return currentRefresh;
  // Prefer the on-disk rotated value (from prior refresh or OAuth callback);
  // fall back to env on first boot.
  try {
    const onDisk = readFileSync(REFRESH_FILE, 'utf-8').trim();
    if (onDisk) {
      currentRefresh = onDisk;
      return onDisk;
    }
  } catch (_) { /* file may not exist yet */ }
  currentRefresh = process.env.WHOOP_REFRESH_TOKEN || null;
  return currentRefresh;
}

function persistRefresh(next) {
  currentRefresh = next;
  try {
    mkdirSync(dirname(REFRESH_FILE), { recursive: true });
    writeFileSync(REFRESH_FILE, next + '\n', { mode: 0o600 });
  } catch (e) {
    console.error('[whoop-sidecar] failed to persist rotated refresh token:', e.message);
  }
}

async function refreshAccessToken() {
  const id = process.env.WHOOP_CLIENT_ID;
  const secret = process.env.WHOOP_CLIENT_SECRET;
  const refresh = loadCurrentRefresh();
  if (!id || !secret || !refresh) {
    throw new Error('WHOOP credentials missing — set NORTHSTAR_HEALTH_MODE=mock for dev');
  }
  const r = await fetch('https://api.prod.whoop.com/oauth/oauth2/token', {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'refresh_token',
      refresh_token: refresh,
      client_id: id,
      client_secret: secret,
      // `offline` is an authorize-time scope only; including it in the
      // refresh request makes WHOOP return invalid_request.
      scope: 'read:recovery read:sleep read:cycles read:profile',
    }),
  });
  if (!r.ok) throw new Error(`whoop oauth refresh ${r.status}: ${await r.text()}`);
  const tok = await r.json();
  accessToken = tok.access_token;
  accessTokenExpires = Date.now() + (tok.expires_in * 1000) - 60_000; // 1m safety
  // WHOOP rotates refresh tokens — persist the new one or the next refresh
  // will fail with 401 and the user will need to re-consent.
  if (tok.refresh_token && tok.refresh_token !== refresh) {
    persistRefresh(tok.refresh_token);
  }
  return accessToken;
}

async function authHeader() {
  if (!accessToken || Date.now() >= accessTokenExpires) {
    await refreshAccessToken();
  }
  return { 'Authorization': `Bearer ${accessToken}`, 'Content-Type': 'application/json' };
}

async function api(path) {
  const h = await authHeader();
  const r = await fetch(`https://api.prod.whoop.com/developer${path}`, { headers: h });
  if (!r.ok) throw new Error(`whoop ${path} ${r.status}: ${await r.text()}`);
  return r.json();
}

// ─── Public surface ───────────────────────────────────────────────────────

// WHOOP retired the /developer/v1/{recovery,activity/sleep} paths; these
// now live at /developer/v2/* with subtly renamed fields (sleep_needed.
// need_from_sleep_debt_milli, formerly need_from_recent_debt_milli). The
// profile endpoint has no v2 equivalent yet, so it stays on v1.

export async function recovery({ days = 14 } = {}) {
  const since = new Date(Date.now() - days * 86400 * 1000).toISOString();
  const r = await api(`/v2/recovery?start=${encodeURIComponent(since)}`);
  return (r.records || []).map(x => ({
    date:   (x.created_at || '').slice(0, 10),
    score:  x.score?.recovery_score ?? null,
    hrv_ms: x.score?.hrv_rmssd_milli != null ? Math.round(x.score.hrv_rmssd_milli) : null,
    rhr:    x.score?.resting_heart_rate ?? null,
  })).filter(r => r.date);
}

export async function sleep({ days = 14 } = {}) {
  const since = new Date(Date.now() - days * 86400 * 1000).toISOString();
  const r = await api(`/v2/activity/sleep?start=${encodeURIComponent(since)}`);
  return (r.records || []).map(x => {
    const ms = x.score?.stage_summary || {};
    const duration =
      (ms.total_in_bed_time_milli || 0) - (ms.total_awake_time_milli || 0);
    const needed = x.score?.sleep_needed || {};
    return {
      date:         (x.end || '').slice(0, 10),
      duration_min: Math.round(duration / 60000),
      score:        x.score?.sleep_performance_percentage ?? null,
      debt_min:     Math.round(
        (needed.need_from_sleep_debt_milli ?? needed.need_from_recent_debt_milli ?? 0) / 60000),
    };
  }).filter(r => r.date);
}

export async function strain({ days = 14 } = {}) {
  const since = new Date(Date.now() - days * 86400 * 1000).toISOString();
  const r = await api(`/v2/cycle?start=${encodeURIComponent(since)}`);
  return (r.records || []).map(x => ({
    date:    (x.start || '').slice(0, 10),
    score:   x.score?.strain ?? null,
    avg_hr:  x.score?.average_heart_rate ?? null,
    max_hr:  x.score?.max_heart_rate ?? null,
  })).filter(r => r.date);
}

export async function profile() {
  return api('/v1/user/profile/basic');
}
