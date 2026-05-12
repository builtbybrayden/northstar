# northstar-server

Go service that backs the [Northstar](../) iOS app. Single binary; SQLite + Litestream for persistence; chi for routing.

## Run locally

```bash
cp .env.example .env       # edit as needed
go run ./cmd/northstar-server
# server listens on :8080
```

Health check:
```bash
curl http://localhost:8080/api/health
```

## Run in Docker

See [`../infra/`](../infra/).

## Phase 0 endpoints

| Method | Path                          | Auth | Notes |
|--------|-------------------------------|------|-------|
| GET    | `/api/health`                 | none | DB ping + version |
| POST   | `/api/pair/initiate`          | none | Mints a 6-digit pairing code + bearer token (server admin will gate this in Phase 1) |
| POST   | `/api/pair/redeem`            | none | Device exchanges code for token; one-shot, 10-minute TTL |
| POST   | `/api/devices/register-apns`  | bearer | Persists the device's APNs push token |
| GET    | `/api/me`                     | bearer | Returns the calling device's metadata |
| GET    | `/api/pillars`                | bearer | Which pillars are enabled server-side |

## Layout

```
cmd/northstar-server/          main.go entry point
internal/
├── api/                       chi router + handlers
├── auth/                      bearer-token middleware, token + code generation
├── config/                    env-driven config
└── db/
    ├── db.go                  SQLite open + goose migration runner
    └── migrations/            *.sql migrations (embedded into the binary)
Dockerfile                     multi-stage distroless build
.env.example                   reference env vars
```

## Tests

```bash
go test ./...
```

(Phase 0 ships without tests in the source tree — they land in Phase 1 alongside the first sync worker.)

## License

MIT. See `../LICENSE`.
