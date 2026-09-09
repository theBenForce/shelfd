# OAuth 2.0 Dynamic Client Registration, MCP SSE Keepalives, and Traefik Homelab Deployment

* Status: accepted
* Deciders: Lead Systems Architect, @core, @mcp, @security, @homelab
* Date: 2026-09-09

## Context and Problem Statement

To deploy Shelfd in a production homelab environment accessible across the public internet, users need to:
1. Access the web and mobile client applications via a custom subdomain (e.g., `books.yourdomain.com`).
2. Connect external AI agents—specifically Stitch and other MCP-compliant agents—across the public internet.
3. Support Stitch's authentication model, which relies on RFC 7591 Dynamic Client Registration, RFC 8414 Authorization Server Metadata discovery, and an interactive OAuth 2.0 authorization code flow with PKCE (RFC 7636).
4. Maintain reliable Server-Sent Events (SSE) streaming through reverse proxies and edge NATs without connection drops.
5. Provide a deterministic Turborepo build pipeline where the Go server embeds the compiled Flutter web client.

## Decision Drivers

* **RFC Standards Compliance**: Implement RFC 7591 (Dynamic Client Registration), RFC 8414 (OAuth 2.0 Authorization Server Metadata), and RFC 7636 (Proof Key for Code Exchange / PKCE).
* **AI Protocol Compatibility**: Stitch and external web-based MCP clients require CORS preflights (`OPTIONS`) and interactive human consent before issuing long-lived bearer tokens.
* **Reverse Proxy Resilience**: Traefik and intermediate load balancers drop idle SSE streams after 60 seconds; keepalive pings are required.
* **Homelab Security Best Practices**: Bind PostgreSQL strictly to loopback (`127.0.0.1:5432`), rate-limit public entrypoints, and parameterize domain/credentials.
* **Monorepo Build Determinism**: Ensure `pnpm build` via Turborepo automatically compiles `@shelfd/app` (Flutter web) before `@shelfd/server` embeds the web distribution.

## Considered Options

* **Static Pre-shared API Tokens Only**: Require users to manually generate API tokens in the web UI and copy them into external tools. (Rejected: Incompatible with Stitch and standard OAuth 2.0 clients).
* **External IdP (e.g. Authentik, Keycloak)**: Delegate OAuth 2.0 to an external identity provider container. (Rejected: Violates Shelfd's zero-bloat, single-daemon homelab principle).
* **Native OAuth 2.0 Engine with Dynamic Registration & PKCE (Chosen)**.

## Decision Outcome

Chosen option: **Native OAuth 2.0 Engine with Dynamic Registration & PKCE**.

### 1. Database Schema & Migrations
Added `0006_oauth_tables.sql` (SQLite) and `0003_oauth_tables.sql` (PostgreSQL/Bun):
* `oauth_clients`: Stores dynamically registered clients (`client_id`, `client_secret`, `client_name`, `redirect_uris`, `grant_types`, `response_types`).
* `oauth_codes`: Stores transient authorization codes with PKCE challenge/method (`code`, `client_id`, `user_id`, `redirect_uri`, `code_challenge`, `code_challenge_method`, `expires_at`).

### 2. OAuth Endpoints & Interactive Consent
* `GET /.well-known/oauth-authorization-server` and `GET /.well-known/openid-configuration`: Returns RFC 8414 metadata (endpoints, supported scopes, response types, and `code_challenge_methods_supported`: `["S256", "plain"]`).
* `POST /oauth/register`: Validates and registers dynamic OAuth clients (RFC 7591), returning `client_id` and client metadata.
* `GET /oauth/authorize` & `POST /oauth/authorize`: Interactive HTML consent and login interface. Validates credentials, creates a single-use authorization code, and redirects to the client's `redirect_uri` with `code` and `state`.
* `POST /oauth/token`: Exchanges authorization codes for bearer tokens, verifying PKCE challenges (`S256` SHA-256 base64url or `plain`). Tokens are saved into `api_tokens` (`OAuth: <client_name>`), enabling seamless access across `/mcp/*` and `/api/v1/*`.

### 3. MCP Server Keepalives & CORS Preflights
* Added a 15-second heartbeat ticker to `/mcp/sse` emitting `: keepalive\n\n` comments, preventing Traefik/NAT idle disconnections.
* Updated `mcp.AuthMiddleware` to allow HTTP `OPTIONS` requests through without authentication, allowing browser CORS preflights to succeed.
* Preserved `?token=` query parameters when constructing `/mcp/messages` endpoints for SSE clients that cannot set `Authorization` headers.

### 4. Reverse Proxy & Caching
* `internal/api/connect.go`: Resolves server URL using `X-Forwarded-Host` and `X-Forwarded-Proto`.
* `internal/api/web.go`: Implemented cache-control headers (`immutable` for hashed assets, `no-cache` for `index.html`) and registered `.svg`, `.woff2`, and `.webp` MIME types.

### 5. Turborepo Orchestration
* `apps/server/package.json`: Added `@shelfd/app` workspace dependency.
* `turbo.json`: Configured `@shelfd/server#build` with `dependsOn: ["^build:web"]`. Running `mise exec -- pnpm build` compiles Flutter web first, placing assets into `apps/app/build/web`, ready for embedding.

### 6. Docker Compose & Traefik Integration
* Added Traefik router/service labels, TLS resolver configuration (`TRAEFIK_CERT_RESOLVER`), rate-limiting middleware (`average=50`, `burst=100`), and loopback binding on PostgreSQL (`127.0.0.1:5432:5432`).

## Consequences

### Positive Consequences

* Stitch and external agents can seamlessly register, authorize, and query Shelfd across the public internet.
* MCP SSE sessions remain connected indefinitely without timing out behind Traefik.
* Zero external auth dependencies required: Shelfd remains self-contained.
* Single-command monorepo build produces a production-ready binary with embedded Flutter web client.

### Negative Consequences

* Added two database tables and migration files for SQLite and PostgreSQL.
* Transient authorization codes require periodic garbage collection or expiration checks upon consumption.
