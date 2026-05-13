# syntax=docker/dockerfile:1.7

# Debian (slim) over Alpine — @actual-app/api pulls in better-sqlite3 which
# requires native compile tools (python + g++). Debian also has prebuilt
# better-sqlite3 binaries for the common combos so most builds skip the
# compile step entirely.

FROM node:20-bookworm-slim AS deps
WORKDIR /app
COPY package.json ./
RUN apt-get update && apt-get install -y --no-install-recommends \
        python3 make g++ ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && npm install --omit=dev --no-audit --no-fund

FROM node:20-bookworm-slim
WORKDIR /app
ENV NODE_ENV=production \
    NORTHSTAR_FINANCE_MODE=mock \
    SIDECAR_HOST=0.0.0.0 \
    SIDECAR_PORT=9090
COPY --from=deps /app/node_modules ./node_modules
COPY package.json ./
COPY src ./src
EXPOSE 9090
# @actual-app/api downloads the budget into ./actual-data (relative to the
# working dir). Pre-create it with the right ownership so init doesn't 500
# on first call.
RUN groupadd --system app && useradd --system --gid app app \
    && mkdir -p /app/actual-data \
    && chown -R app:app /app
USER app
CMD ["node", "src/server.js"]
