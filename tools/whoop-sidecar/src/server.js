// Northstar WHOOP sidecar — long-lived HTTP gateway.
//
// Set NORTHSTAR_HEALTH_MODE=mock (default) to skip the WHOOP API entirely.

import http from 'node:http';
import { URL } from 'node:url';
import { writeFileSync, mkdirSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

const MODE   = (process.env.NORTHSTAR_HEALTH_MODE || 'mock').toLowerCase();
const HOST   = process.env.SIDECAR_HOST || '127.0.0.1';
const PORT   = parseInt(process.env.SIDECAR_PORT || '9091', 10);
const SECRET = process.env.SIDECAR_SHARED_SECRET || '';

const WHOOP_AUTH_URL  = 'https://api.prod.whoop.com/oauth/oauth2/auth';
const WHOOP_TOKEN_URL = 'https://api.prod.whoop.com/oauth/oauth2/token';
const WHOOP_SCOPES    = 'read:recovery read:sleep read:cycles read:profile offline';
const REDIRECT_URI    = process.env.WHOOP_REDIRECT_URI || `http://localhost:${PORT}/oauth/callback`;
const TOKEN_OUT_FILE  = join(homedir(), '.northstar', 'whoop-refresh-token.txt');

let provider;
if (MODE === 'mock') {
  const m = await import('./mock.js');
  provider = {
    recovery: async () => m.mockRecovery(),
    sleep:    async () => m.mockSleep(),
    strain:   async () => m.mockStrain(),
    profile:  async () => m.mockProfile(),
  };
  log(`mode=mock — no WHOOP credentials needed`);
} else {
  provider = await import('./whoop.js');
  log(`mode=whoop — needs WHOOP_CLIENT_ID + WHOOP_CLIENT_SECRET + WHOOP_REFRESH_TOKEN`);
}

function log(...args) {
  console.log(`[whoop-sidecar ${new Date().toISOString()}]`, ...args);
}

function send(res, status, body) {
  const json = Buffer.from(JSON.stringify(body));
  res.writeHead(status, { 'Content-Type': 'application/json', 'Content-Length': json.length });
  res.end(json);
}

function authorized(req) {
  if (!SECRET) return true;
  return req.headers['x-sidecar-secret'] === SECRET;
}

const routes = {
  'GET /health': async () => ({
    ok: true, mode: MODE, service: 'northstar-whoop-sidecar', version: '0.0.1',
  }),
  'GET /recovery': async (_, url) => provider.recovery({ days: int(url, 'days', 14) }),
  'GET /sleep':    async (_, url) => provider.sleep({    days: int(url, 'days', 14) }),
  'GET /strain':   async (_, url) => provider.strain({   days: int(url, 'days', 14) }),
  'GET /profile':  async () => provider.profile(),
  'GET /oauth/start':    oauthStart,
  'GET /oauth/callback': oauthCallback,
};

// ─── OAuth bootstrap ──────────────────────────────────────────────────────
// One-time browser flow to obtain a WHOOP refresh token. Available regardless
// of mode so the user can get a refresh token while the sidecar still runs in
// mock mode.

let oauthState = '';

async function oauthStart(_, _url) {
  const id = process.env.WHOOP_CLIENT_ID;
  if (!id) {
    return {
      __status: 400,
      error: 'WHOOP_CLIENT_ID not set',
      hint: 'export WHOOP_CLIENT_ID + WHOOP_CLIENT_SECRET and restart the sidecar',
    };
  }
  oauthState = Math.random().toString(36).slice(2) + Date.now().toString(36);
  const params = new URLSearchParams({
    response_type: 'code',
    client_id:     id,
    redirect_uri:  REDIRECT_URI,
    scope:         WHOOP_SCOPES,
    state:         oauthState,
  });
  return {
    __redirect: `${WHOOP_AUTH_URL}?${params.toString()}`,
  };
}

async function oauthCallback(_, url) {
  const code  = url.searchParams.get('code');
  const state = url.searchParams.get('state');
  const err   = url.searchParams.get('error');
  if (err)  return { __status: 400, error: 'whoop_denied', detail: err };
  if (!code) return { __status: 400, error: 'missing_code' };
  if (state !== oauthState) return { __status: 400, error: 'state_mismatch' };

  const id     = process.env.WHOOP_CLIENT_ID;
  const secret = process.env.WHOOP_CLIENT_SECRET;
  if (!id || !secret) {
    return { __status: 400, error: 'WHOOP_CLIENT_ID or WHOOP_CLIENT_SECRET missing' };
  }

  const r = await fetch(WHOOP_TOKEN_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type:    'authorization_code',
      code,
      client_id:     id,
      client_secret: secret,
      redirect_uri:  REDIRECT_URI,
    }),
  });
  if (!r.ok) {
    return { __status: 502, error: 'token_exchange_failed', status: r.status, body: await r.text() };
  }
  const tok = await r.json();
  if (!tok.refresh_token) {
    return { __status: 502, error: 'no_refresh_token', body: tok };
  }

  try {
    mkdirSync(join(homedir(), '.northstar'), { recursive: true });
    writeFileSync(TOKEN_OUT_FILE, tok.refresh_token + '\n', { mode: 0o600 });
  } catch (e) {
    log('failed to write refresh token file', e.message);
  }
  log(`OAuth complete. refresh_token written to ${TOKEN_OUT_FILE}`);
  return {
    __html: renderSuccessPage(tok.refresh_token, TOKEN_OUT_FILE),
  };
}

function renderSuccessPage(refresh, path) {
  return `<!doctype html><meta charset="utf-8"><title>Northstar · WHOOP</title>
<style>body{font:14px/1.5 -apple-system,system-ui,sans-serif;background:#0a0a0a;color:#fff;
padding:48px;max-width:720px;margin:0 auto}h1{color:#00B6FF;font-size:22px}code{background:#141414;
padding:2px 6px;border-radius:6px;font-family:ui-monospace,Menlo,monospace;font-size:12px}
pre{background:#141414;padding:14px;border-radius:10px;overflow-x:auto;font-size:12px;
border:1px solid #2a2a2a}.ok{color:#16EC06}.dim{color:#6b6b6b}</style>
<h1>✓ WHOOP authorization complete</h1>
<p class="ok">Refresh token saved to <code>${path}</code> (mode 0600).</p>
<p class="dim">Next: copy the value into <code>~/.northstar/.env</code> as
<code>WHOOP_REFRESH_TOKEN</code>, then restart the sidecar with
<code>NORTHSTAR_HEALTH_MODE=whoop</code>.</p>
<pre>${refresh.replace(/[<>&]/g, c => ({ '<':'&lt;','>':'&gt;','&':'&amp;' }[c]))}</pre>
<p class="dim">You can close this tab.</p>`;
}

function int(url, key, def) {
  const v = url.searchParams.get(key);
  if (!v) return def;
  const n = parseInt(v, 10);
  return isNaN(n) ? def : n;
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://${req.headers.host}`);
  const key = `${req.method} ${url.pathname}`;
  if (!authorized(req)) return send(res, 401, { error: 'unauthorized' });
  const handler = routes[key];
  if (!handler) return send(res, 404, { error: 'not_found', path: key });
  try {
    const out = await handler(req, url);
    if (out && out.__redirect) {
      res.writeHead(302, { Location: out.__redirect });
      return res.end();
    }
    if (out && out.__html) {
      const body = Buffer.from(out.__html);
      res.writeHead(out.__status || 200, { 'Content-Type': 'text/html; charset=utf-8', 'Content-Length': body.length });
      return res.end(body);
    }
    const status = (out && out.__status) || 200;
    if (out && typeof out === 'object') delete out.__status;
    send(res, status, out);
  } catch (e) {
    log('handler error', key, e);
    send(res, 500, { error: 'handler_failed', message: e.message });
  }
});

server.listen(PORT, HOST, () => {
  log(`listening on http://${HOST}:${PORT} · mode=${MODE}`);
});

process.on('SIGINT',  () => { log('SIGINT'); server.close(() => process.exit(0)); });
process.on('SIGTERM', () => { log('SIGTERM'); server.close(() => process.exit(0)); });
