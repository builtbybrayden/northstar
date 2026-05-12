# northstar-notion-importer

One-shot CLI that reads Notion databases and writes them into a running Northstar server via the REST API. Personal-use seed importer — **not** part of the app, not shipped to users.

## Why this exists

Northstar's Goals pillar is intentionally native (SQLite-backed), so the user maintains goals inside the app rather than syncing with Notion at runtime. But the user's existing roadmap already lives in Notion. This CLI is the one-time cutover: read Notion → POST to Northstar → done. After the import you edit in Northstar.

## Install

```bash
cd tools/notion-importer
npm install
```

## Get credentials

1. **Notion internal integration token**:
   - Visit https://www.notion.so/my-integrations
   - "New integration" → name it "Northstar importer" → grant "Read content" capability
   - Copy the `secret_…` token → that's `NOTION_TOKEN`
   - For each database you want to import, open it in Notion → "Add connections" → select "Northstar importer"

2. **Northstar bearer token**:
   ```bash
   PAIR=$(curl -s -X POST http://localhost:8080/api/pair/initiate -d '{}')
   CODE=$(echo $PAIR | grep -oE '[0-9]{6}')
   TOKEN=$(curl -s -X POST http://localhost:8080/api/pair/redeem -d "{\"code\":\"$CODE\"}" | grep -oE '"bearer_token":"[^"]+' | cut -d\" -f4)
   echo $TOKEN
   ```

3. **Database IDs**: in Notion, open the database, look at the URL — the 32-char hex after the last `/` is the ID.

## Usage

```bash
export NOTION_TOKEN=secret_...
export NORTHSTAR_URL=http://localhost:8080
export NORTHSTAR_TOKEN=<bearer>

# Dry run first to validate
node src/cli.js \
  --milestones=3560ea6ce4a681b484a2e6da60d8f7d6 \
  --output=<output-db-id> \
  --networking=<networking-db-id> \
  --dry-run

# Real import
node src/cli.js \
  --milestones=3560ea6ce4a681b484a2e6da60d8f7d6 \
  --output=<output-db-id>
```

## Property mapping

The importer is loose about Notion property names — it tries common variants so it works against the existing "Cybersecurity Career Roadmap" workspace without renaming columns first.

| Northstar field | Notion property names it looks for |
|---|---|
| milestone title    | the title column (any name) |
| milestone due_date | `Due`, `Due Date`, `Date`, `Target` |
| milestone status   | `Status` (matched fuzzily — "Done" → done, "In Progress" → in_progress, etc.) |
| milestone flagship | `Flagship`, `Pinned` (checkbox) |
| output category    | `Category`, `Type` (select) |
| output url         | `URL`, `Link` |
| networking person  | title column |
| networking context | `Context`, `Notes` |

For anything that doesn't fit, edit the records in Northstar after import.

## Idempotency

Each imported row stamps `notion:<page_id>` into the body/description. Re-running the importer will create duplicates currently — that's a follow-up. For v1, run it once.

## Limitations

- Doesn't import Notion page children (sub-tables, embeds). Top-level properties only.
- Doesn't currently update existing records — only inserts.
- `--daily-log` import stores only the reflection text; items[] inside Notion pages are not parsed.
