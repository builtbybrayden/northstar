# syntax=docker/dockerfile:1.7

FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json ./
# Only install deps in actual-mode; mock mode runs without any deps
RUN npm install --omit=dev --no-audit --no-fund || true

FROM node:20-alpine
WORKDIR /app
ENV NODE_ENV=production \
    NORTHSTAR_FINANCE_MODE=mock \
    SIDECAR_HOST=0.0.0.0 \
    SIDECAR_PORT=9090
COPY --from=deps /app/node_modules ./node_modules
COPY package.json ./
COPY src ./src
EXPOSE 9090
# Run as non-root
RUN addgroup -S app && adduser -S -G app app && chown -R app:app /app
USER app
CMD ["node", "src/server.js"]
