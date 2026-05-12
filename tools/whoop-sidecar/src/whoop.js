// Real WHOOP mode — wraps the v2 REST API with OAuth2 refresh-token handling.
// Inert until you set the env vars. The Go server stays the same either way.
//
// Required env:
//   WHOOP_CLIENT_ID
//   WHOOP_CLIENT_SECRET
//   WHOOP_REFRESH_TOKEN   (obtained out-of-band via the OAuth grant)
//
// WHOOP issues short-lived access tokens (~1h); this layer refreshes lazily.

let accessToken = null;
let accessTokenExpires = 0;

async function refreshAccessToken() {
  const id = process.env.WHOOP_CLIENT_ID;
  const secret = process.env.WHOOP_CLIENT_SECRET;
  const refresh = process.env.WHOOP_REFRESH_TOKEN;
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
      scope: 'read:recovery read:sleep read:cycles read:profile',
    }),
  });
  if (!r.ok) throw new Error(`whoop oauth refresh ${r.status}: ${await r.text()}`);
  const tok = await r.json();
  accessToken = tok.access_token;
  accessTokenExpires = Date.now() + (tok.expires_in * 1000) - 60_000; // 1m safety
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

export async function recovery({ days = 14 } = {}) {
  const since = new Date(Date.now() - days * 86400 * 1000).toISOString();
  const r = await api(`/v1/recovery?start=${encodeURIComponent(since)}`);
  return (r.records || []).map(x => ({
    date:   (x.created_at || '').slice(0, 10),
    score:  x.score?.recovery_score ?? null,
    hrv_ms: x.score?.hrv_rmssd_milli != null ? Math.round(x.score.hrv_rmssd_milli) : null,
    rhr:    x.score?.resting_heart_rate ?? null,
  })).filter(r => r.date);
}

export async function sleep({ days = 14 } = {}) {
  const since = new Date(Date.now() - days * 86400 * 1000).toISOString();
  const r = await api(`/v1/activity/sleep?start=${encodeURIComponent(since)}`);
  return (r.records || []).map(x => {
    const ms = x.score?.stage_summary || {};
    const duration =
      (ms.total_in_bed_time_milli || 0) - (ms.total_awake_time_milli || 0);
    return {
      date:         (x.end || '').slice(0, 10),
      duration_min: Math.round(duration / 60000),
      score:        x.score?.sleep_performance_percentage ?? null,
      debt_min:     Math.round((x.score?.sleep_needed?.need_from_recent_debt_milli || 0) / 60000),
    };
  }).filter(r => r.date);
}

export async function strain({ days = 14 } = {}) {
  const since = new Date(Date.now() - days * 86400 * 1000).toISOString();
  const r = await api(`/v1/cycle?start=${encodeURIComponent(since)}`);
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
