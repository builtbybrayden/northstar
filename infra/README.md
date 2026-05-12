# `infra/`

Docker compose + Litestream sidecar for running Northstar locally or on a home server / VPS.

## Run locally

```bash
cd ../server && cp .env.example .env    # edit if needed
cd ../infra && docker compose up -d server
docker compose logs -f server
curl http://localhost:8080/api/health
```

## With B2 backup

```bash
# Set LITESTREAM_B2_* in ../server/.env first
docker compose --profile backup up -d
docker compose logs -f litestream
```

## Restore from B2

```bash
docker run --rm -v northstar-data:/data \
  -e LITESTREAM_B2_BUCKET=$LITESTREAM_B2_BUCKET \
  -e LITESTREAM_ACCESS_KEY_ID=$LITESTREAM_B2_ACCESS_KEY_ID \
  -e LITESTREAM_SECRET_ACCESS_KEY=$LITESTREAM_B2_SECRET_ACCESS_KEY \
  litestream/litestream:0.3.13 \
  restore -o /data/northstar.db \
    s3://$LITESTREAM_B2_BUCKET/northstar.db
```

## Tear down (keeps volume)

```bash
docker compose down
```

## Tear down (DELETES volume — destructive)

```bash
docker compose down -v
```
