# AI & Agent Integration — Embedded MCP Server over HTTP/SSE

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team
* Date: 2026-09-07

## Context and Problem Statement

Modern coding agents (Claude Desktop, Cursor, local agents) need programmatic, authenticated access to the user's ebook library to synthesize knowledge, answer thematic questions, and quote chapter text. Which protocol and transport should be implemented?

## Decision Drivers

* Industry-standard protocol for agentic integration (Model Context Protocol).
* Remote and local accessibility across the home network.
* Low token consumption to protect agent context windows.
* Strong authentication to prevent unauthorized library access.

## Considered Options

* **Model Context Protocol (MCP) over HTTP/SSE**
* **MCP over Stdio (Child Process Execution)**
* **Proprietary REST API Only**

## Decision Outcome

Chosen option: **MCP over HTTP/SSE**, embedding an SSE endpoint (`/mcp/sse`) and JSON-RPC message endpoint (`/mcp/messages`) directly in the `shelfd` daemon, protected by Bearer API tokens.

### Positive Consequences

* AI clients running anywhere on the local network (or via reverse proxy) can connect seamlessly.
* Exposes standardized tools: `search_library`, `get_book_metadata`, and `read_chapter_content`.
* Enforces strict Level-of-Detail (LOD) token ceilings (< 100 tokens per search hit; windowed chapter reads).

### Negative Consequences

* Running over HTTP requires managing network security, authentication tokens, and CORS.
* Go lacks the official Anthropic TypeScript SDK, requiring manual handler implementation.

## Pros and Cons of the Options

### MCP over HTTP/SSE

* Good, because works across network boundaries (desktop AI client connecting to home server).
* Good, because standardized protocol understood natively by Claude, Cursor, and agent frameworks.
* Bad, because requires HTTP server lifecycle and session state management.

### MCP over Stdio

* Good, because simple pipe-based transport with no network authentication needed.
* Bad, because only works if the agent runs on the exact same machine inside the container, preventing external desktop clients from connecting.

### Proprietary REST API Only

* Good, because simpler to implement without SSE or JSON-RPC 2.0 framing.
* Bad, because agents require custom tool-calling code or custom bridge plugins instead of native MCP integration.
