// Northstar WHOOP sidecar — long-lived HTTP gateway.
//
// Set NORTHSTAR_HEALTH_MODE=mock (default) to skip the WHOOP API entirely.

import http from 'node:http';
import { URL } from 'node:url';

const MODE   = (process.env.NORTHSTAR_HEALTH_MODE || 'mock').toLowerCase();
const HOST   = process.env.SIDECAR_HOST || '127.0.0.1';
const PORT   = parseInt(process.env.SIDECAR_PORT || '9091', 10);
const SECRET = process.env.SIDECAR_SHARED_SECRET || '';

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
};

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
    send(res, 200, out);
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
