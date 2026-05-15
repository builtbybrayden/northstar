// Northstar Actual sidecar — long-lived HTTP gateway.
//
// The Go server polls this sidecar over localhost HTTP. The sidecar holds the
// downloaded Actual budget in memory + on disk and exposes JSON endpoints for
// accounts / categories / transactions / budgets.
//
// Set NORTHSTAR_FINANCE_MODE=mock to skip Actual entirely for dev.

import http from 'node:http';
import { URL } from 'node:url';

const MODE   = (process.env.NORTHSTAR_FINANCE_MODE || 'mock').toLowerCase();
const HOST   = process.env.SIDECAR_HOST || '127.0.0.1';
const PORT   = parseInt(process.env.SIDECAR_PORT || '9090', 10);
const SECRET = process.env.SIDECAR_SHARED_SECRET || '';   // optional belt-and-suspenders

let provider;
if (MODE === 'mock') {
  const m = await import('./mock.js');
  provider = {
    init: async () => ({ ok: true, mode: 'mock' }),
    accounts: async () => m.mockAccounts(),
    categories: async () => m.mockCategories(),
    transactions: async () => m.mockTransactions(),
    budgets: async () => m.mockBudgets(),
    shutdown: async () => ({ ok: true }),
  };
  log(`mode=mock — no Actual server required`);
} else {
  provider = await import('./actual.js');
  log(`mode=actual — will connect on /init`);
}

function log(...args) {
  console.log(`[sidecar ${new Date().toISOString()}]`, ...args);
}

function send(res, status, body) {
  // A handler returning undefined would crash JSON.stringify → Buffer.from
  // and the catch path would call send() again with a recursive failure.
  // Default to {ok:true} so the wire contract stays { Content-Type: json }.
  const payload = body === undefined ? { ok: true } : body;
  const json = Buffer.from(JSON.stringify(payload));
  res.writeHead(status, {
    'Content-Type': 'application/json',
    'Content-Length': json.length,
  });
  res.end(json);
}

async function readJSON(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on('data', c => chunks.push(c));
    req.on('end', () => {
      if (!chunks.length) return resolve({});
      try { resolve(JSON.parse(Buffer.concat(chunks).toString('utf-8'))); }
      catch (e) { reject(e); }
    });
    req.on('error', reject);
  });
}

function authorized(req) {
  if (!SECRET) return true;
  return req.headers['x-sidecar-secret'] === SECRET;
}

const routes = {
  'GET /health': async () => ({
    ok: true,
    mode: MODE,
    service: 'northstar-actual-sidecar',
    version: '0.0.1',
  }),
  'POST /init': async (req) => {
    const body = await readJSON(req);
    return provider.init(body);
  },
  'GET /accounts':     async () => provider.accounts(),
  'GET /categories':   async () => provider.categories(),
  'GET /transactions': async (req, url) => {
    const since = url.searchParams.get('since') || undefined;
    return provider.transactions({ since });
  },
  'GET /budgets': async (req, url) => {
    const month = url.searchParams.get('month') || undefined;
    return provider.budgets({ month });
  },
  'POST /shutdown': async () => provider.shutdown(),
};

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

const shutdown = async (sig) => {
  log(`received ${sig}, shutting down`);
  try { await provider.shutdown(); } catch (_) {}
  server.close(() => process.exit(0));
};
process.on('SIGINT',  () => shutdown('SIGINT'));
process.on('SIGTERM', () => shutdown('SIGTERM'));
