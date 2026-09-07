# MCP Protocol Design & Testing Manual

This technical manual instructs the AI on designing, implementing, and verifying Model Context Protocol (MCP) server endpoints and tools in Shelfd.

## Invariants
1. Protocol transport is strictly HTTP with Server-Sent Events (SSE).
   * Handshake endpoint: `GET /mcp/sse`
   * Message endpoint: `POST /mcp/messages`
2. Bearer token authentication required on all MCP requests.
3. Level-of-Detail (LOD) token ceilings must be enforced:
   * `search_library`: Returns concise summaries (< 100 tokens per search hit). No raw full chapter bodies.
   * `read_chapter_content`: Must support pagination windows (`start_paragraph`, `end_paragraph`). Default cap is 2,000 words.

## Tool Verification Workflow
1. Verify SSE handshake:
   ```bash
   curl -N -H "Accept: text/event-stream" -H "Authorization: Bearer <test-token>" http://localhost:8080/mcp/sse
   ```
2. Verify tool schemas and responses using the MCP Inspector:
   ```bash
   npx @modelcontextprotocol/inspector http://localhost:8080/mcp/sse
   ```
3. Verify core tools are registered:
   * `search_library`
   * `get_book_metadata`
   * `read_chapter_content`
