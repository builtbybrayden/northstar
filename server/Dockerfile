# syntax=docker/dockerfile:1.7

# ─── Build stage ──────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS build
WORKDIR /src

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/northstar-server ./cmd/northstar-server

# ─── Runtime stage ────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/northstar-server /app/northstar-server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/northstar-server"]
