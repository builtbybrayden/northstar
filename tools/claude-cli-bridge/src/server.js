// Northstar Claude CLI bridge.
//
// Why this exists: the Go server runs in a distroless Docker container,
// which can't see the host's `claude` CLI binary. Rather than baking
// Claude CLI + Node into the server image (heavy) or moving the server
// off Docker (loses Litestream), we run this tiny bridge on the HOST.
// The dockerized server reaches it via host.docker.internal:9092.
//
// HTTP surface:
//   GET  /health           → {ok, mode, version}
//   POST /prompt           → {reply}
//     body: { question, system?, model? }
//
// Security model:
//   - Bind to 127.0.0.1 by default (host-only). Docker reaches it via
//     host.docker.internal which routes through the host gateway.
//   - Optional shared-secret header (`X-Bridge-Secret`) when the bridge
//     env has BRIDGE_SHARED_SECRET set.
//   - User text is passed to `claude` via stdin — never spliced into a
//     shell string — so no injection risk on that path.

import http from 'node:http';
import { spawn } from 'node:child_process';
import { URL } from 'node:url';

const HOST   = process.env.BRIDGE_HOST || '127.0.0.1';
const PORT   = parseInt(process.env.BRIDGE_PORT || '9092', 10);
const SECRET = process.env.BRIDGE_SHARED_SECRET || '';
const CLAUDE = process.env.BRIDGE_CLAUDE_BIN || 'claude';
// Cap how long the bridge waits for a single Claude run. 120s is plenty
// for the prompt-snapshot answer shapes the server uses.
const TIMEOUT_MS = parseInt(process.env.BRIDGE_TIMEOUT_MS || '120000', 10);

function log(...args) {
  console.log(`[claude-bridge ${new Date().toISOString()}]`, ...args);
}

function send(res, status, body) {
  const json = Buffer.from(JSON.stringify(body));
  res.writeHead(status, {
    'Content-Type': 'application/json',
    'Content-Length': json.length,
  });
  res.end(json);
}

async function readJSON(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let bytes = 0;
    req.on('data', c => {
      bytes += c.length;
      // Refuse anything larger than 1 MiB — the server only sends a few KB.
      if (bytes > 1_048_576) {
        req.destroy(new Error('request too large'));
        return;
      }
      chunks.push(c);
    });
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
  return req.headers['x-bridge-secret'] === SECRET;
}

function runClaude({ question, system, model }) {
  return new Promise((resolve, reject) => {
    const args = ['--print', '--output-format', 'text'];
    if (system) args.push('--system-prompt', system);
    if (model)  args.push('--model', model);

    const child = spawn(CLAUDE, args, {
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    const stdoutChunks = [];
    const stderrChunks = [];
    let settled = false;

    const timer = setTimeout(() => {
      if (settled) return;
      settled = true;
      try { child.kill('SIGKILL'); } catch (_) {}
      reject(new Error(`claude CLI timed out after ${TIMEOUT_MS}ms`));
    }, TIMEOUT_MS);

    child.stdout.on('data', c => stdoutChunks.push(c));
    child.stderr.on('data', c => stderrChunks.push(c));
    child.on('error', err => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      reject(err);
    });
    child.on('close', code => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      const stdout = Buffer.concat(stdoutChunks).toString('utf-8');
      const stderr = Buffer.concat(stderrChunks).toString('utf-8');
      if (code !== 0) {
        const msg = (stderr || stdout || `exit ${code}`).trim();
        reject(new Error(`claude CLI exit ${code}: ${msg.slice(0, 500)}`));
        return;
      }
      resolve(stdout.trim());
    });

    child.stdin.end(question);
  });
}

const routes = {
  'GET /health': async () => ({
    ok: true,
    mode: 'cli',
    service: 'northstar-claude-cli-bridge',
    version: '0.0.1',
  }),
  'POST /prompt': async (req) => {
    const body = await readJSON(req);
    const question = (body.question || '').toString();
    if (!question.trim()) {
      return { __status: 400, error: 'question required' };
    }
    const reply = await runClaude({
      question,
      system: body.system ? body.system.toString() : '',
      model:  body.model  ? body.model.toString()  : '',
    });
    return { reply };
  },
};

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://${req.headers.host}`);
  const key = `${req.method} ${url.pathname}`;

  if (!authorized(req)) return send(res, 401, { error: 'unauthorized' });

  const handler = routes[key];
  if (!handler) return send(res, 404, { error: 'not_found', path: key });

  try {
    const out = await handler(req, url);
    const status = (out && out.__status) || 200;
    if (out && typeof out === 'object') delete out.__status;
    send(res, status, out);
  } catch (e) {
    log('handler error', key, e.message);
    send(res, 500, { error: 'handler_failed', message: e.message });
  }
});

server.listen(PORT, HOST, () => {
  log(`listening on http://${HOST}:${PORT} · claude_bin=${CLAUDE} · auth=${SECRET ? 'shared-secret' : 'open'}`);
});

const shutdown = sig => {
  log(`received ${sig}, shutting down`);
  server.close(() => process.exit(0));
};
process.on('SIGINT',  () => shutdown('SIGINT'));
process.on('SIGTERM', () => shutdown('SIGTERM'));
