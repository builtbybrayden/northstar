# syntax=docker/dockerfile:1.7

# WHOOP sidecar has no third-party dependencies (Node stdlib + fetch only),
# so we skip the deps stage that the actual-sidecar uses. If a dep is ever
# added, switch this back to a multi-stage build mirroring actual-sidecar.

FROM node:20-alpine
WORKDIR /app
ENV NODE_ENV=production \
    NORTHSTAR_HEALTH_MODE=mock \
    SIDECAR_HOST=0.0.0.0 \
    SIDECAR_PORT=9091
COPY package.json ./
COPY src ./src
EXPOSE 9091
RUN addgroup -S app && adduser -S -G app app && \
    mkdir -p /home/app/.northstar && chown -R app:app /app /home/app
USER app
CMD ["node", "src/server.js"]
