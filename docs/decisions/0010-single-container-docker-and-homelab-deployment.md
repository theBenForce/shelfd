# Single-Container Docker & Homelab Deployment

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team, @homelab, @librarian
* Date: 2026-09-07

## Context and Problem Statement

`shelfd` is designed to run in diverse self-hosted homelab environments (e.g. Unraid, TrueNAS SCALE, Synology DSM, Docker Compose on Linux/Debian) alongside Audiobookshelf. Self-hosters require an easy-to-deploy, robust single container that starts instantly, consumes minimal memory (< 50MB runtime), seamlessly shares volume permissions without file permission lockouts, and protects Audiobookshelf libraries from unwanted sidecar modifications. How should the Docker container, volume mapping, user permissions, and deployment artifacts be architected?

## Decision Drivers

* **Minimal Image Size & Resource Footprint**: Image size target < 50MB and runtime idle memory < 50MB.
* **Cgo & sqlite-vec Compilation**: Compiling native C extensions (`mattn/go-sqlite3` and `sqlite-vec-go-bindings`) while maintaining a tiny runtime base.
* **Audiobookshelf Sanctity & Permission Ergonomics**: Preventing Linux file ownership conflicts (PUID/PGID mapping with `umask 002`) while avoiding recursive permission changes on shared `/library` directories.
* **Operational Simplicity**: Single-container Docker Compose setup with zero external dependencies and zero-config environment variable overrides.

## Considered Options

* **Alpine Linux Multi-Stage Build with `su-exec` and `tini`**
* **Debian Bookworm-Slim Multi-Stage Build with `gosu`**
* **Root-Only Single-Stage Scratch / Distroless Container**

## Decision Outcome

Chosen option: **Alpine Linux Multi-Stage Build with `su-exec` and `tini`**.
- Builder stage uses `golang:1.24-alpine` with `gcc` and `musl-dev` to compile Cgo with `sqlite-vec`.
- Runtime stage uses `alpine:3.21` with `su-exec`, `shadow`, `tzdata`, `ca-certificates`, and `tini`.
- `docker/entrypoint.sh` inspects `PUID`, `PGID`, and `UMASK` (defaulting to `1000:1000` and `002`), configures the non-root `shelfd` user, chowns `/data` exclusively, and executes `shelfd` via `su-exec`.

### Positive Consequences

* **Tiny Footprint**: The final compressed image is ~35–45MB, well under the 50MB requirement.
* **Audiobookshelf Permission Parity**: Using `UMASK=002` and configurable `PUID`/`PGID` ensures files written by `shelfd` (newly uploaded EPUBs) are group-writable and match host permissions.
* **Library Safety**: The entrypoint strictly avoids recursive `chown -R` on `/library`, preventing startup freezes on multi-terabyte libraries and preserving Audiobookshelf metadata timestamps.
* **Clean Signal Handling**: `tini` acts as PID 1 to gracefully harvest child processes and forward `SIGTERM`/`SIGINT` to Go for graceful shutdown.
* **Flexible Configuration**: Full environment variable override support (`SHELFD_*`) enables complete headless setup via `docker-compose.yml` without requiring a mounted `config.yaml`.

### Negative Consequences

* Compiling Cgo against musl requires `gcc` and `musl-dev` in the builder stage.
* Homelab environments running with custom user namespaces require matching PUID/PGID configuration.

## Pros and Cons of the Options

### Alpine Linux Multi-Stage with `su-exec`

* Good, because runtime image is ~35–45MB.
* Good, because `su-exec` is extremely lightweight and purpose-built for Alpine containers.
* Good, because `tini` guarantees proper signal forwarding and zombie process reaping.
* Bad, because Cgo compilation on musl requires careful header dependency handling.

### Debian Bookworm-Slim with `gosu`

* Good, because uses standard glibc.
* Bad, because base Debian slim image alone is ~75MB+, exceeding the < 50MB total target.

### Root-Only Scratch / Distroless

* Good, because absolute minimal image size (~25MB).
* Bad, because lacks shell for dynamic PUID/PGID user mapping, creating Linux permission locks on shared mounts with Audiobookshelf.
* Bad, because running daemons as root violates container security best practices.
