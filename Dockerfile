# ==============================================================================
# Stage 1: Build daemon binary with Cgo and sqlite-vec
# ==============================================================================
FROM golang:1.24-alpine AS builder

# Install Cgo build dependencies for SQLite and sqlite-vec
RUN apk add --no-cache gcc musl-dev git

WORKDIR /src

# Leverage Docker layer caching for Go modules
COPY apps/server/go.mod apps/server/go.sum ./
RUN go mod download

# Copy server source code and compile statically with external cgo linking stripped
COPY apps/server ./
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=0.1.0" \
    -o /out/shelfd \
    ./cmd/server

# ==============================================================================
# Stage 2: Minimal runtime image (< 50MB)
# ==============================================================================
FROM alpine:3.21

# Install runtime utilities:
# - ca-certificates: TLS verification for remote AI endpoints (OpenAI)
# - tzdata: homelab timezone support
# - su-exec & shadow: PUID/PGID dynamic user mapping and privilege dropping
# - tini: zombie process reaping and graceful signal handling (SIGTERM/SIGINT)
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    su-exec \
    shadow \
    tini

# Create standard volume mount points
RUN mkdir -p /data /library /config

# Copy binary from builder stage
COPY --from=builder /out/shelfd /usr/local/bin/shelfd

# Copy entrypoint script
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Default environment configuration
ENV PUID=1000 \
    PGID=1000 \
    UMASK=002 \
    SHELFD_STORAGE_DATA_DIR=/data \
    SHELFD_STORAGE_LIBRARY_DIR=/library

EXPOSE 8080

VOLUME ["/data", "/library"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]
CMD ["shelfd"]
