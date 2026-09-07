# Core Language & Runtime — Go for Daemon Host

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team
* Date: 2026-09-07

## Context and Problem Statement

The `shelfd` backend daemon must operate 24/7 on low-power home servers, NAS devices, and shared media hardware alongside services like Audiobookshelf and Jellyfin. It needs a minimal memory footprint, instant startup, and straightforward packaging into a small container. Which language and runtime should be used for the core server?

## Decision Drivers

* Minimal memory usage (< 50MB idle) on self-hosted environments.
* Fast compilation to a single static binary for distroless/alpine Docker images.
* Robust standard library for HTTP services, concurrency, and filesystem access.
* Mature Cgo bindings for embedded SQLite and C extensions (`sqlite-vec`).

## Considered Options

* **Go (1.22+)**
* **TypeScript / Node.js (Fastify)**
* **Rust (Axum)**
* **Python (FastAPI)**

## Decision Outcome

Chosen option: **Go (1.22+)**, because it strikes the best balance between low resource usage, fast development velocity, and effortless container distribution.

### Positive Consequences

* Single static binary distribution keeps container image size under 40MB.
* Memory usage idles around 20–40MB, leaving host resources free for media transcoding and Audiobookshelf.
* Native Cgo support allows direct integration with SQLite and `sqlite-vec`.
* Concurrency primitives (goroutines/channels) make background library scanning and AI ingestion straightforward.

### Negative Consequences

* Unlike TypeScript, there is no official Anthropic MCP SDK, requiring an internal implementation of the Model Context Protocol over HTTP/SSE.
* Handling Cgo adds slight build overhead compared to pure Go.

## Pros and Cons of the Options

### Go (1.22+)

* Good, because of low memory overhead and instant cold starts.
* Good, because standard library HTTP primitives reduce reliance on heavy frameworks.
* Good, because cross-compilation and single-binary packaging are first-class.
* Bad, because Cgo is required for native SQLite extensions.

### TypeScript / Node.js

* Good, because official `@modelcontextprotocol/sdk` is available.
* Good, because EPUB parsing libraries on npm are mature.
* Bad, because runtime memory footprint is 150MB–250MB+ idle.
* Bad, because container images require Node.js runtime layers.

### Rust

* Good, because zero-cost abstractions and memory safety.
* Bad, because longer compile times and higher development friction for MVP velocity.

### Python

* Good, because rich AI/ML ecosystem.
* Bad, because high memory consumption, GIL concurrency limitations, and complex virtualenv packaging in containers.
