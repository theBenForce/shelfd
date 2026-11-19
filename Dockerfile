# ==============================================================================
# Stage 1: Build daemon binary with Cgo and sqlite-vec
# ==============================================================================
FROM golang:1.25-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Leverage Docker layer caching for Go modules and heavy Cgo amalgamations
COPY apps/server/go.mod apps/server/go.sum ./
RUN go mod download
RUN CGO_ENABLED=1 GOOS=linux go build \
    -tags sqlite_fts5 \
    github.com/mattn/go-sqlite3 \
    github.com/asg017/sqlite-vec-go-bindings/cgo \
    github.com/uptrace/bun \
    github.com/uptrace/bun/dialect/pgdialect \
    github.com/uptrace/bun/dialect/sqlitedialect \
    github.com/uptrace/bun/driver/pgdriver \
    github.com/golang-jwt/jwt/v5 \
    github.com/google/uuid \
    golang.org/x/crypto/bcrypt \
    golang.org/x/net/html \
    gopkg.in/yaml.v3

# Copy server source code and compile with Cgo
COPY apps/server ./
ARG VERSION=0.1.0
RUN CGO_ENABLED=1 GOOS=linux go build \
    -tags sqlite_fts5 \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/shelfd \
    ./cmd/server

# ==============================================================================
# Stage 2: Build reference TypeScript plugins
# ==============================================================================
FROM node:22-bookworm-slim AS plugin-builder

WORKDIR /plugins-src
COPY packages/plugin-sdk ./packages/plugin-sdk
COPY plugins/project-gutenberg ./plugins/project-gutenberg

RUN node ./packages/plugin-sdk/scripts/build-sdk.mjs \
    && node ./packages/plugin-sdk/cli/shelfd-plugin.mjs build ./plugins/project-gutenberg \
    && mkdir -p /out/builtin-plugins/project-gutenberg/dist \
    && cp ./plugins/project-gutenberg/plugin.json /out/builtin-plugins/project-gutenberg/plugin.json \
    && cp ./plugins/project-gutenberg/dist/index.mjs /out/builtin-plugins/project-gutenberg/dist/index.mjs

# ==============================================================================
# Opt-in Minimal Stage: Pure-Go runtime image without Node/built-in plugins
# Build via: docker build --target runtime-minimal -t shelfd:minimal .
# ==============================================================================
FROM debian:bookworm-slim AS runtime-minimal

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    gosu \
    tini \
    curl \
    libsqlite3-0 \
    && rm -rf /var/lib/apt/lists/*

# Create standard volume mount points and web static asset directory
RUN mkdir -p /data /library /config /usr/share/shelfd/web

# Copy binary from builder stage
COPY --from=builder /out/shelfd /usr/local/bin/shelfd

# Copy bundled Flutter web static assets
COPY apps/app/build/web /usr/share/shelfd/web

# Copy entrypoint script
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Default environment configuration
ENV PUID=1000 \
    PGID=1000 \
    UMASK=002 \
    SHELFD_STORAGE_DATA_DIR=/data \
    SHELFD_STORAGE_LIBRARY_DIR=/library \
    SHELFD_SERVER_WEB_DIR=/usr/share/shelfd/web

EXPOSE 8080

VOLUME ["/data", "/library"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/usr/bin/tini", "--", "/entrypoint.sh"]
CMD ["shelfd"]

# ==============================================================================
# Default Final Stage: Node-enabled runtime image with built-in plugins (shelfd:latest)
# ==============================================================================
FROM debian:bookworm-slim AS runtime-plugins

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    gosu \
    tini \
    curl \
    libsqlite3-0 \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /data /data/plugins /library /config /usr/share/shelfd/web /usr/share/shelfd/builtin-plugins

COPY --from=node:22-bookworm-slim /usr/local/bin/node /usr/local/bin/node
COPY --from=builder /out/shelfd /usr/local/bin/shelfd
COPY --from=plugin-builder /out/builtin-plugins /usr/share/shelfd/builtin-plugins
COPY apps/app/build/web /usr/share/shelfd/web
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV PUID=1000 \
    PGID=1000 \
    UMASK=002 \
    SHELFD_STORAGE_DATA_DIR=/data \
    SHELFD_STORAGE_LIBRARY_DIR=/library \
    SHELFD_SERVER_WEB_DIR=/usr/share/shelfd/web

EXPOSE 8080
VOLUME ["/data", "/library"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/usr/bin/tini", "--", "/entrypoint.sh"]
CMD ["shelfd"]

FROM runtime-plugins AS runtime
