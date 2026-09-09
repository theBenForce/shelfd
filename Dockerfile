# ==============================================================================
# Stage 1: Build daemon binary with Cgo and sqlite-vec
# ==============================================================================
FROM golang:1.25-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Leverage Docker layer caching for Go modules
COPY apps/server/go.mod apps/server/go.sum ./
RUN go mod download

# Copy server source code and compile with Cgo
COPY apps/server ./
RUN CGO_ENABLED=1 GOOS=linux go build \
    -tags sqlite_fts5 \
    -ldflags="-s -w -X main.Version=0.1.0" \
    -o /out/shelfd \
    ./cmd/server

# ==============================================================================
# Stage 2: Minimal runtime image
# ==============================================================================
FROM debian:bookworm-slim

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
