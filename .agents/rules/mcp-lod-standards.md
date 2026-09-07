# Model Context Protocol (MCP) & Token Economy Standards

## Invariants

1. **Transport:**
   - Server runs over HTTP with Server-Sent Events (SSE).
   - Handshake path: `/mcp/sse`.
   - Message post path: `/mcp/messages`.
   - Bearer token authentication header required for all MCP endpoints.

2. **Level of Detail (LOD) Ceilings:**
   - **L0 (Tool List / Schema)**: Minimal descriptions, tightly typed JSON schema parameters.
   - **L1 (Search Results)**: `search_library` responses must return concise chapter summaries (< 100 tokens per search hit). Do NOT dump full chapter text into search results.
   - **L2 (Interactive Reading)**: `read_chapter_content` must support optional pagination/windowing (`start_paragraph`, `end_paragraph`). If omitted, default maximum chunk limit is 2,000 words to prevent context blowup.

3. **Cross-Book Synthesis:**
   - Tools must return consistent identifiers (`book_id`, `chapter_id`, `book_title`, `author`, `series`, `series_index`) so agents can cite multiple books in a single response.
