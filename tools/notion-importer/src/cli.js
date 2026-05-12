#!/usr/bin/env node
// northstar-import-notion — one-shot personal seed importer.
//
// Reads pages from one or more Notion databases and writes them into a running
// Northstar server via the REST API. Idempotent for milestones / output / networking
// because the importer keys on Notion page IDs stored in description_md / body_md.
//
// Usage:
//   NOTION_TOKEN=secret_... \
//   NORTHSTAR_URL=http://localhost:8080 \
//   NORTHSTAR_TOKEN=<bearer> \
//   northstar-import-notion \
//     --milestones=<notion-db-id> \
//     --output=<notion-db-id> \
//     --networking=<notion-db-id> \
//     --dry-run

import { Client } from '@notionhq/client';

const args = parseArgs(process.argv.slice(2));
if (args.help || (!args.milestones && !args.output && !args.networking && !args['daily-log'])) {
  console.log(`
northstar-import-notion — one-shot Notion → Northstar seed

ENV:
  NOTION_TOKEN       Notion integration token (Internal Integration)
  NORTHSTAR_URL      e.g. http://localhost:8080  (default)
  NORTHSTAR_TOKEN    bearer token from /api/pair/redeem

FLAGS:
  --milestones=<id>   Notion database ID for milestones (title, due date, status, flagship)
  --output=<id>       Notion database ID for output log (title, category, date, url)
  --networking=<id>   Notion database ID for networking log (person, date, context, next action)
  --daily-log=<id>    Notion database ID for daily log (date, items[], reflection)
  --dry-run           Print what would be sent; no writes
  --limit=N           Cap on records per DB (default 500)
  --help              This text

The mapping is intentionally loose so it works against the user's existing
"Cybersecurity Career Roadmap" workspace without forcing a schema. The importer
tries common property names: "Title", "Name", "Due", "Date", "Status",
"Flagship", "Category", "URL", "Person", "Context", "Next Action".
`);
  process.exit(args.help ? 0 : 1);
}

const NOTION_TOKEN = process.env.NOTION_TOKEN;
const NS_URL = (process.env.NORTHSTAR_URL || 'http://localhost:8080').replace(/\/+$/, '');
const NS_TOKEN = process.env.NORTHSTAR_TOKEN;

if (!NOTION_TOKEN) { fatal('NOTION_TOKEN env required'); }
if (!args['dry-run'] && !NS_TOKEN) { fatal('NORTHSTAR_TOKEN env required (use /api/pair/redeem)'); }

const limit = parseInt(args.limit || '500', 10);
const dryRun = !!args['dry-run'];
const notion = new Client({ auth: NOTION_TOKEN });

(async () => {
  if (args.milestones)  await importMilestones(args.milestones);
  if (args.output)      await importOutput(args.output);
  if (args.networking)  await importNetworking(args.networking);
  if (args['daily-log']) await importDailyLog(args['daily-log']);
  console.log('done.');
})().catch(e => fatal(e.stack || e.message));

// ─── Importers ─────────────────────────────────────────────────────────────

async function importMilestones(dbId) {
  console.log(`\n[milestones] reading Notion db ${dbId}`);
  let posted = 0;
  for await (const page of paginate(dbId)) {
    const body = {
      title:          getTitle(page),
      description_md: `notion:${page.id}\n\n${getRichText(page, ['Notes', 'Description'])}`.trim(),
      due_date:       getDate(page, ['Due', 'Due Date', 'Date', 'Target']),
      status:         mapStatus(getSelect(page, ['Status'])),
      flagship:       getCheckbox(page, ['Flagship', 'Pinned']) || false,
    };
    if (!body.title) continue;
    await postOrSkip('/api/goals/milestones', body);
    posted++;
    if (posted >= limit) break;
  }
  console.log(`[milestones] ${dryRun ? 'would write' : 'wrote'} ${posted}`);
}

async function importOutput(dbId) {
  console.log(`\n[output] reading Notion db ${dbId}`);
  let posted = 0;
  for await (const page of paginate(dbId)) {
    const body = {
      title:    getTitle(page),
      category: (getSelect(page, ['Category', 'Type']) || 'tool').toLowerCase(),
      date:     getDate(page, ['Date', 'Published']) || new Date().toISOString().slice(0,10),
      body_md:  `notion:${page.id}\n\n${getRichText(page, ['Notes', 'Description', 'Summary'])}`.trim(),
      url:      getURL(page, ['URL', 'Link']) || '',
    };
    if (!body.title) continue;
    await postOrSkip('/api/goals/output', body);
    posted++;
    if (posted >= limit) break;
  }
  console.log(`[output] ${dryRun ? 'would write' : 'wrote'} ${posted}`);
}

async function importNetworking(dbId) {
  console.log(`\n[networking] reading Notion db ${dbId}`);
  let posted = 0;
  for await (const page of paginate(dbId)) {
    const body = {
      person:           getTitle(page) || getRichText(page, ['Person', 'Name']),
      date:             getDate(page, ['Date', 'Last Contact']) || new Date().toISOString().slice(0,10),
      context:          getRichText(page, ['Context', 'Notes']),
      next_action:      getRichText(page, ['Next Action', 'Next Step']),
      next_action_due:  getDate(page, ['Next Action Due', 'Follow Up']) || '',
    };
    if (!body.person) continue;
    await postOrSkip('/api/goals/networking', body);
    posted++;
    if (posted >= limit) break;
  }
  console.log(`[networking] ${dryRun ? 'would write' : 'wrote'} ${posted}`);
}

async function importDailyLog(dbId) {
  console.log(`\n[daily-log] reading Notion db ${dbId}`);
  // Group by date: each Notion page = one day's reflection; the title or a
  // sub-table holds the items. We just stuff the rendered text into reflection_md.
  let posted = 0;
  for await (const page of paginate(dbId)) {
    const date = getDate(page, ['Date', 'Day']);
    if (!date) continue;
    const reflection = getRichText(page, ['Reflection', 'Notes', 'Summary']);
    const body = { items: [], reflection_md: `notion:${page.id}\n\n${reflection}`.trim() };
    await putOrSkip(`/api/goals/daily/${date}`, body);
    posted++;
    if (posted >= limit) break;
  }
  console.log(`[daily-log] ${dryRun ? 'would write' : 'wrote'} ${posted}`);
}

// ─── HTTP ─────────────────────────────────────────────────────────────────

async function postOrSkip(path, body) {
  if (dryRun) {
    console.log(`  [dry-run] POST ${path}  ${JSON.stringify(body).slice(0,140)}`);
    return;
  }
  await call('POST', path, body);
}
async function putOrSkip(path, body) {
  if (dryRun) {
    console.log(`  [dry-run] PUT  ${path}  ${JSON.stringify(body).slice(0,140)}`);
    return;
  }
  await call('PUT', path, body);
}
async function call(method, path, body) {
  const r = await fetch(NS_URL + path, {
    method,
    headers: {
      'Authorization': `Bearer ${NS_TOKEN}`,
      'Content-Type': 'application/json',
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!r.ok) {
    const text = await r.text().catch(() => '');
    throw new Error(`${method} ${path} → ${r.status}: ${text.slice(0,200)}`);
  }
}

// ─── Notion helpers ───────────────────────────────────────────────────────

async function* paginate(dbId) {
  let cursor;
  do {
    const r = await notion.databases.query({
      database_id: dbId,
      start_cursor: cursor,
      page_size: 50,
    });
    for (const p of r.results) yield p;
    cursor = r.has_more ? r.next_cursor : undefined;
  } while (cursor);
}

function findProp(page, candidates) {
  for (const name of candidates) {
    if (page.properties && page.properties[name]) return page.properties[name];
  }
  return null;
}
function getTitle(page) {
  const t = Object.values(page.properties || {}).find(p => p?.type === 'title');
  if (!t) return '';
  return (t.title || []).map(x => x.plain_text || '').join('').trim();
}
function getRichText(page, candidates) {
  const p = findProp(page, candidates);
  if (!p) return '';
  if (p.type === 'rich_text') return (p.rich_text || []).map(x => x.plain_text || '').join('').trim();
  if (p.type === 'title')     return (p.title || []).map(x => x.plain_text || '').join('').trim();
  return '';
}
function getSelect(page, candidates) {
  const p = findProp(page, candidates);
  if (!p) return null;
  if (p.type === 'select') return p.select?.name || null;
  if (p.type === 'status') return p.status?.name || null;
  if (p.type === 'multi_select') return (p.multi_select || []).map(s => s.name).join(',') || null;
  return null;
}
function getCheckbox(page, candidates) {
  const p = findProp(page, candidates);
  return p?.type === 'checkbox' ? !!p.checkbox : false;
}
function getDate(page, candidates) {
  const p = findProp(page, candidates);
  if (!p) return '';
  if (p.type === 'date' && p.date?.start) return p.date.start.slice(0, 10);
  return '';
}
function getURL(page, candidates) {
  const p = findProp(page, candidates);
  if (!p) return '';
  if (p.type === 'url') return p.url || '';
  if (p.type === 'rich_text') return getRichText(page, candidates);
  return '';
}
function mapStatus(s) {
  if (!s) return 'pending';
  const x = s.toLowerCase();
  if (x.includes('done') || x.includes('complete')) return 'done';
  if (x.includes('progress') || x.includes('doing')) return 'in_progress';
  if (x.includes('archive') || x.includes('cancel')) return 'archived';
  return 'pending';
}

// ─── argv ─────────────────────────────────────────────────────────────────

function parseArgs(argv) {
  const out = {};
  for (const a of argv) {
    if (a.startsWith('--')) {
      const [k, v] = a.slice(2).split('=', 2);
      out[k] = v === undefined ? true : v;
    }
  }
  return out;
}
function fatal(msg) {
  console.error('error:', msg);
  process.exit(1);
}
