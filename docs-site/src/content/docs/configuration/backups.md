---
title: Backups (Litestream)
description: Continuous SQLite replication to a B2 bucket.
---

Northstar's whole data set lives in one SQLite file at `${NORTHSTAR_DB_PATH}`
(default `/data/northstar.db` in Docker). Litestream streams the WAL in real
time to an S3-compatible bucket — we recommend Backblaze B2 because it's the
cheapest option and well-supported.

## Why streaming, not snapshots?

Snapshot backups lose anything between the last snapshot and a crash. Litestream
replicates every WAL frame, so the worst case is a few seconds of loss. The
streaming buffer is small and B2 charges by stored bytes, not by request count,
so the ongoing cost is dollars-per-year.

## Setup

### 1. Create a B2 application key

In the Backblaze console, create a bucket (private) and an application key
scoped to it. You'll get a key ID and a key secret.

### 2. Configure `infra/litestream.yml`

The repo ships with a template. The only fields you usually need to change:

```yaml
dbs:
  - path: /data/northstar.db
    replicas:
      - type: s3
        bucket: northstar-bak
        path: northstar
        endpoint: https://s3.us-west-002.backblazeb2.com
        region: us-west-002
        retention: 168h        # 7 days of point-in-time
        retention-check-interval: 1h
        snapshot-interval: 24h
```

### 3. Set the env on the host

```sh
export LITESTREAM_ACCESS_KEY_ID=…
export LITESTREAM_SECRET_ACCESS_KEY=…
```

### 4. Boot the backup profile

```sh
cd infra
docker compose --profile backup up -d litestream
```

The Litestream container shares the same volume as `northstar-server` and
replicates continuously.

## Restoring

To rebuild a fresh `/data/northstar.db` from B2:

```sh
docker compose run --rm litestream restore -o /data/northstar.db \
  s3://northstar-bak/northstar
```

Bring up the server afterwards as normal.

## Verifying the backup

Quarterly: do a dry-run restore to a throwaway path and `sqlite3 path 'pragma integrity_check;'`.
If it ever returns anything but `ok`, escalate to "rotate keys, redeploy, refile
B2's incident process" — at single-user scale a corrupted backup is the loudest
signal something is wrong with your storage stack.

## Threat model

Backup encryption-at-rest is provided by B2 server-side encryption with B2's
master key. If you want **end-to-end** encryption (so a compromised B2 account
cannot read your data), use `restic` with a separate passphrase as a
*secondary* backup path; Litestream alone does not encrypt with a key you hold.

> The user's existing finance tracker already runs `restic` to B2 with a
> separate passphrase. That same posture can be applied here.
