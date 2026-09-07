# Authentication Architecture — JWT & Scoped API Tokens

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team
* Date: 2026-09-07

## Context and Problem Statement

`shelfd` is deployed as a self-hosted daemon for personal and family use. It needs an authentication strategy that accommodates interactive client applications (the Flutter app on mobile/desktop) as well as headless, automated AI coding agents connecting via the Model Context Protocol (MCP). How should authentication and authorization be structured?

## Decision Drivers

* Stateless, secure session handling for interactive client applications.
* Simple, long-lived, revocable authentication for MCP tools and AI clients.
* Low server-side storage overhead (avoiding heavy session tables).
* Protection of personal book notes, collections, and server resources.

## Considered Options

* **Dual-Tier: JWTs for Users + Scoped Bearer API Tokens for MCP**
* **Session Cookies Only**
* **HTTP Basic Auth Across All Endpoints**
* **External OIDC / OAuth2 Provider Only**

## Decision Outcome

Chosen option: **Dual-Tier: JWTs for Users + Scoped Bearer API Tokens for MCP**. Interactive users authenticate with username/password, receiving a short-lived signed JWT access token and refresh token. Headless MCP agents authenticate with persistent Bearer API tokens managed via user settings and stored as SHA-256 hashes in SQLite (`api_tokens`).

### Positive Consequences

* Client apps enjoy stateless JWT verification without roundtrip database lookups on every request.
* MCP agents (Claude Desktop, Cursor) can be configured with a single persistent Bearer token string without interactive login flows.
* API tokens can be individually named, scoped, and revoked in the database.
* Zero external auth server required for basic self-hosting.

### Negative Consequences

* Managing JWT secret keys in `config.yaml`.
* Requires token refresh logic in the Flutter client.

## Pros and Cons of the Options

### Dual-Tier (JWT + API Tokens)

* Good, because tailored to both interactive GUI clients and headless CLI/agent clients.
* Good, because API tokens are simple to revoke in the `api_tokens` table.
* Good, because lightweight and self-contained within SQLite.
* Bad, because two distinct token validation paths must be maintained in the auth middleware.

### Session Cookies Only

* Good, because simple for web browsers.
* Bad, because difficult to manage across native Flutter mobile apps and external MCP agent processes.

### HTTP Basic Auth

* Good, because easiest to configure.
* Bad, because credentials must be sent with every request, cannot be easily scoped or rotated without changing passwords.

### External OIDC Only (Authelia / Authentik)

* Good, because centralizes authentication across homelab services.
* Bad, because requires an external SSO service running, breaking the zero-dependency single-container experience.
