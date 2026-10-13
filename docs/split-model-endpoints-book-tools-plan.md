# Split Model Endpoints and Book Tools plan

This plan decouples AI embedding and chat configurations and equips the reader chat interface with dynamic MCP book tools for readers, developers, and maintainers.
Embedding and chat model endpoints, authentication keys, and providers (Ollama, OpenAI, Gemini) are defined independently with seamless backward-compatible fallbacks and dial-time SSRF guards.
The book chat interface transitions from eager, upfront context stuffing to an agentic tool-calling architecture: the chat model begins with lean context (Book Title, Description, and Book ID) and an active in-process connection to the Shelfd MCP server tools (`search_library`, `get_book_metadata`, `read_chapter_content`), reinforced by programmatic book scoping and an explicit prompt instructing it to primarily answer questions from the current book.
The plan delivers four sequential pull requests from configuration and AI client decoupling to MCP tool enhancement, agentic tool-calling chat engine, and Flutter client verification.
The pull requests in order are `pr-ai-split-endpoints-config`, `pr-mcp-book-tools-schema`, `pr-chat-agentic-tool-calling`, and `pr-chat-client-ux-verification`.

## How to read this

One box is one unit of work.
Every box names the evidence that checks it.
A nested box is a sub-step of the box above it.
Check a box only when its evidence exists, a file, a log line, a screenshot, a test run, or a SHA.
The body is a how-to.
The appendices explain and record.

The program runs `pstack/skills/poteto-mode/playbooks/autopilot-stack.md`.
The operator reviews the UI pull request and lands the verified base-branch stack bottom-up.

Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

## Program checklist

### Arm the program

- [ ] State the protocol and this plan to the operator, then stop. Start execution only on the operator's explicit go.
- [ ] On the operator's go, arm a `/goal` with this exact text: "docs/split-model-endpoints-book-tools-plan.md, pr-ai-split-endpoints-config, pr-mcp-book-tools-schema, pr-chat-agentic-tool-calling, pr-chat-client-ux-verification, verification rule, operator merges, all boxes checked."
- [ ] Read these from trunk at program start. Re-read them at every tick:
  - [ ] `git show origin/main:pstack/skills/poteto-mode/playbooks/autopilot-stack.md`
  - [ ] `git show origin/main:pstack/skills/swarm/SKILL.md`
  - [ ] `git show origin/main:pstack/skills/control-ui/SKILL.md`
  - [ ] `git show origin/main:pstack/skills/poteto-mode/playbooks/opening-a-pr.md`
  - [ ] `git show origin/main:pstack/skills/principle-model-the-domain/SKILL.md`
- [ ] Arm the 30-minute audit tick. In a local session, a real terminal `/loop`. In a cloud root, a cloud-sleeper wake chain. Never leave the cadence to memory.
- [ ] Use this tick prompt, verbatim: "Re-read the execution playbook from trunk and the armed /goal. Audit the operation against both and fix drift in this tick. Probe every active lane and judge progress by side effects only. Stand down a stuck lane and dispatch its replacement now. Then post a short status message to the operator in chat only when the audit found a tracked change that no earlier status message reported, such as a PR opened, a code-ready head, a round launched or closed, a verdict, a merge, a stuck agent and the action taken, a blocker added or cleared, or a decision only the operator can make. Name every such change and nothing else. Do not repeat a table, the merged list, or an unchanged blocker. If the audit found none, end the turn with no reply text. Either way, log this tick's row in your decision trail. The row names the items reported, or none."
- [ ] On the operator's hold or stand-down, send every owner a zero-writes order at once.

### Spawn owners

- [ ] Spawn one owner per PR with the full lifecycle the execution playbook names.
- [ ] Follow this dependency graph. Start dependent work only after its parent merges, or base it on the parent branch when the execution playbook stacks:
  - [ ] `pr-ai-split-endpoints-config` is first and branches from `main`.
  - [ ] `pr-mcp-book-tools-schema` after `pr-ai-split-endpoints-config`.
  - [ ] `pr-chat-agentic-tool-calling` after `pr-mcp-book-tools-schema`.
  - [ ] `pr-chat-client-ux-verification` after `pr-chat-agentic-tool-calling`.
- [ ] Hold the file boundaries. Server PRs touch only `apps/server/**` and root config files. Client PRs touch only `apps/app/**`.
- [ ] Hold the review gate. `pr-chat-client-ux-verification` changes user-facing chat interactions. It waits for the operator's review in chat with screenshots and receipts before merge.

### PR mechanics, for every PR

- [ ] Resolve the forge once. Default to `gh`; if `command -v origin` succeeds and Origin can resolve the repository, use `origin pr` for every PR operation. Record any fallback to `gh`. Never require `gt`.
- [ ] Open the PR ready, never draft, with `origin pr create --status open --base <base-branch>` or `gh pr create --base <base-branch>` according to the resolved forge. A stack child targets its parent branch.
- [ ] Run the repo's lint and typecheck once before the PR-facing push. Push with hooks on.
- [ ] Run `/deslop` before each commit and `/no-comments` before review.
- [ ] Triage every Bugbot and security-reviewer comment per `../references/bugbot-triage.md`.
- [ ] Rebase onto current trunk before the code-ready report and babysit. Keep that merge base in fix rounds. Rebase again only at merge prep, on a `git merge-tree` conflict with trunk, or on a CI failure that comes from a change on trunk.

### Verdict and merge, for every PR

- [ ] At the code-ready head SHA and at each later push that changes the patch, run the swarm per `pstack/skills/swarm/SKILL.md`. One gates lane. The ten live lanes from the PR's **Verify, live** block. The perf lane from its **Verify, perf** block. Two or more audit lanes, each with its own focus, that read the diff and the receipts and distrust the PR body. The root audits the receipts in the merge-ready report before the verdict.
- [ ] Clean only when every lane is `PASS`. Findings go back to the owner, including a defect that a lane filed as a note. A new head gets a fresh swarm and a fresh verdict, except for results that stay valid under the patch-id rule in `playbooks/shipping.md`.
- [ ] Append the verified PR to the base-branch stack and let the operator land it bottom-up.

### Boot recipe, for every live lane

Each live lane runs on its own cloud VM at the PR head. Drive through `control-ui` or `control-cli` from `pstack` / `openhands`.

- [ ] `git fetch origin <head-branch> && git checkout <head SHA>`.
- [ ] Start the backend server with `mise exec -- air` (or `go run ./cmd/server`) and Flutter app with `mise exec -- flutter run -d web-server --web-port 3000`.
- [ ] Deliver input only through the control skill's commands. Run read-only diagnostics via curl or inspect scripts.
- [ ] Save every screenshot to `/tmp/swarm-<pr-id>/worker-<n>/<slug>.png` and return the paths with the report.

---

## Decouple embedder and chat model endpoints and authentication (pr-ai-split-endpoints-config)

**Depends on.** None.

**Files.**

- [ ] Edit `apps/server/internal/config/config.go`.
- [ ] Edit `apps/server/internal/config/config_test.go`.
- [ ] Edit `apps/server/internal/ai/client.go`.
- [ ] Edit `apps/server/internal/ai/ollama.go`.
- [ ] Edit `apps/server/internal/ai/openai.go`.
- [ ] Edit `apps/server/internal/ai/google.go`.
- [ ] Edit `apps/server/internal/ai/client_test.go`.
- [ ] Edit `apps/server/cmd/server/main.go`.
- [ ] Edit `config.example.yaml`.
- [ ] Edit `.env.example`.

**Build.**

- [ ] Define `ModelEndpointConfig` struct in `config.go` containing `Provider`, `BaseURL`, `APIKey`, `Model`, and optional `Dimensions`. Implement `String()` masking sensitive API keys (`sk-****`).
- [ ] Add `Embedder ModelEndpointConfig` and `Chat ModelEndpointConfig` fields to `AIConfig` in `config.go`.
- [ ] Implement two-phase fallback inheritance in `config.Parse`: leave `Embedder` and `Chat` unpopulated prior to YAML unmarshaling so that legacy top-level `ai.provider`, `ai.base_url`, `ai.api_key`, `ai.embedding_model`, and `ai.chat_model` cleanly populate unset fields without being clobbered by default values.
- [ ] Prevent provider cross-contamination: top-level `ai.base_url` and `ai.api_key` are only inherited if the child endpoint's provider matches the top-level provider (or is unspecified). Never leak cloud provider keys to separate local or custom endpoints.
- [ ] Add bidirectional synchronization between `cfg.AI.EmbeddingDimensions` and `cfg.AI.Embedder.Dimensions` during validation to prevent SQLite `vec0` / Postgres vector dimension migration mismatches.
- [ ] Add environment variable overrides in `ApplyEnvOverrides` strictly sequenced: legacy `SHELFD_AI_*` overrides first, inheritance to children, then specific `SHELFD_AI_EMBEDDER_*` and `SHELFD_AI_CHAT_*` overrides.
- [ ] In `apps/server/cmd/server/main.go`, invoke `cfg.Validate()` immediately **after** `ApplyEnvOverrides(cfg)` to prevent environment variables from bypassing validation gates.
- [ ] Implement comprehensive SSRF protection on both `Embedder.BaseURL` and `Chat.BaseURL`: reject IP link-local and cloud metadata addresses (`169.254.0.0/16`, `fe80::/10`, `fd00:ec2::254`, `::ffff:169.254.0.0/112`) via `net.ParseIP` and DNS hostname evaluation.
- [ ] Segregate AI client abstractions in `apps/server/internal/ai/client.go` into `Embedder` interface (`GenerateEmbedding`, `GenerateBatchEmbeddings`) and `ChatClient` interface (`Chat`, `SummarizeChapter`).
- [ ] Provide a composite `Client` implementation implementing universal `Client` (`Embedder` + `ChatClient`) that delegates embedding tasks to the configured `Embedder` and conversational/summarization tasks to the configured `ChatClient`.
- [ ] Implement constructors `NewEmbedder(cfg *config.ModelEndpointConfig)` and `NewChatClient(cfg *config.ModelEndpointConfig)` supporting Ollama, OpenAI, and Google Gemini.
- [ ] Update `apps/server/cmd/server/main.go` to construct decoupled AI services, logging sanitized endpoints with query parameters stripped.

**You see.**

- [ ] Startup log line: `logger.Info("AI services initialized", "embedder_provider", cfg.AI.Embedder.Provider, "embedder_url", sanitizedEmbedderURL, "chat_provider", cfg.AI.Chat.Provider, "chat_url", sanitizedChatURL)`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Unit tests in `apps/server/internal/config/config_test.go` verify independent YAML parsing, environment variable overrides, partial fallback inheritance, dimension synchronization, provider key isolation, and SSRF rejection across IPv4/IPv6 metadata formats. Run `mise exec -- go test ./internal/config/...`.
- [ ] Unit tests in `apps/server/internal/ai/client_test.go` verify that `NewClient`, `NewEmbedder`, and `NewChatClient` correctly initialize distinct mock servers with distinct auth headers and URL prefixes. Run `mise exec -- go test ./internal/ai/...`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on `grok-4.7-xhigh-fast` at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run server with legacy flat `config.yaml` (`ai.provider=ollama`). Save `/tmp/swarm-pr-ai-split-endpoints-config/worker-1/legacy-fallback.png`. Pass when both embedding generation and chat completions succeed using legacy top-level values.
- [ ] Lane 2. Split configuration with Ollama for embedder (`http://127.0.0.1:11434`) and OpenAI for chat (`https://api.openai.com/v1`). Run against local mock servers in test mode. Save `/tmp/swarm-pr-ai-split-endpoints-config/worker-2/split-ollama-openai.png`. Pass when embeddings query Ollama mock and chat queries OpenAI mock.
- [ ] Lane 3. Split configuration with OpenAI for embedder and Google Gemini for chat. Save `/tmp/swarm-pr-ai-split-endpoints-config/worker-3/split-openai-google.png`. Pass when batch embeddings use OpenAI authorization header and chat uses Google API key.
- [ ] Lane 4. Independent authentication key verification. Provide invalid key for chat and valid key for embedder. Save `/tmp/swarm-pr-ai-split-endpoints-config/worker-4/independent-keys.png`. Pass when book vector ingestion succeeds while chat endpoint returns 401 Unauthorized.
- [ ] Lane 5. Environment variable overrides verification. Override only `SHELFD_AI_CHAT_BASE_URL` and `SHELFD_AI_CHAT_API_KEY` via env. Save `/tmp/swarm-pr-ai-split-endpoints-config/worker-5/env-overrides.png`. Pass when runtime config matches overridden chat endpoint while preserving file embedder settings.
- [ ] Lane 6. SSRF guard on embedder endpoint. Set `SHELFD_AI_EMBEDDER_BASE_URL="http://169.254.169.254"`. Run server startup and capture process exit. Pass when process exits with non-zero exit code and stderr contains fatal SSRF validation error.
- [ ] Lane 7. SSRF guard on chat endpoint. Set `SHELFD_AI_CHAT_BASE_URL="http://[fd00:ec2::254]/v1"`. Run server startup and capture process exit. Pass when process exits with non-zero exit code and stderr contains fatal SSRF validation error.
- [ ] Lane 8. Ingest new EPUB book through scanner and ingestion worker. Save `/tmp/swarm-pr-ai-split-endpoints-config/worker-8/book-ingestion.png`. Pass when paragraph embeddings populate database using configured embedder model.
- [ ] Lane 9. Chapter summarization worker execution. Save `/tmp/swarm-pr-ai-split-endpoints-config/worker-9/chapter-summary.png`. Pass when chapter summaries generate using configured chat/summary model.
- [ ] Lane 10. Health check endpoint reporting. Call `GET /health`. Save `/tmp/swarm-pr-ai-split-endpoints-config/worker-10/health-check.png`. Pass when server status is 200 OK.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Composite client routing overhead in milliseconds.
- [ ] Probe. Benchmark test running 1,000 dispatches through composite AI client versus direct client.
- [ ] Baseline. Measure direct client dispatch latency of 0.04 ms.
- [ ] Rule. Composite client routing overhead must remain under 0.08 ms per call.

**Review gate.** None. `pr-ai-split-endpoints-config` is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Rebased onto current trunk after the verdict, patch-id unchanged.
- [ ] Append to base-branch stack.

---

## Enhance MCP tools with book scoping and function-calling schemas (pr-mcp-book-tools-schema)

**Depends on.** `pr-ai-split-endpoints-config`.

**Files.**

- [ ] Edit `apps/server/internal/mcp/tools.go`.
- [ ] Edit `apps/server/internal/mcp/protocol.go`.
- [ ] Edit `apps/server/internal/mcp/server.go`.
- [ ] Edit `apps/server/internal/mcp/mcp_test.go`.

**Build.**

- [ ] Extend `search_library` tool parameters in `mcp/protocol.go` and `mcp/tools.go` to accept an optional `book_id` string argument.
- [ ] Pass `filter.BookID = &bookID` to `repo.SearchVectorParagraphs` when `book_id` argument is present.
- [ ] Maintain a single canonical search tool (`search_library`) to avoid model confusion on small open-weight LLMs (omit redundant `search_book` alias).
- [ ] Add strict identifier validation: reject malformed `book_id` arguments that do not conform to valid ULID or UUID syntax.
- [ ] Expose `AvailableToolDefinitions() []ai.ToolDefinition` on `mcp.Server` and `mcp.ToolExecutor` returning structured definitions, descriptions, and JSON Schema properties for `search_library`, `get_book_metadata`, and `read_chapter_content`.
- [ ] Implement direct in-process method `ExecuteTool(ctx context.Context, name string, args map[string]any) (*ToolExecutionResult, error)` on `ToolExecutor` that returns both human-readable Markdown and structured `[]BookCitation` items without HTTP/SSE socket round-trips.
- [ ] Sanitize SQLite FTS5 search queries in repository fallbacks, escaping syntax characters to prevent query parsing crashes.
- [ ] Preserve MCP Level-of-Detail (LOD) token safety ceilings: excerpt truncations capped at 400 characters, chapter content windowed to 50 paragraphs.

**You see.**

- [ ] Log line: `mcp: search_library executed with book_id "book-456", returned 3 passages`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Unit tests in `apps/server/internal/mcp/mcp_test.go` verify `search_library` with `book_id`, `AvailableToolDefinitions` schema generation, structured citation extraction, LOD limits (<400 char excerpts, 50-paragraph windows), and in-process `ExecuteTool`. Run `mise exec -- go test ./internal/mcp/...`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on `grok-4.7-xhigh-fast` at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Call `search_library` without `book_id` via MCP SSE client. Save `/tmp/swarm-pr-mcp-book-tools-schema/worker-1/global-search.png`. Pass when results search across entire library catalog.
- [ ] Lane 2. Call `search_library` with `book_id="book-1"`. Save `/tmp/swarm-pr-mcp-book-tools-schema/worker-2/scoped-search.png`. Pass when returned passages strictly match `book-1`.
- [ ] Lane 3. Cross-book isolation verification. Query for a distinctive keyword present in `book-2` while providing `book_id="book-1"`. Save `/tmp/swarm-pr-mcp-book-tools-schema/worker-3/isolation.png`. Pass when zero hits return.
- [ ] Lane 4. Call `get_book_metadata` for `book-1`. Save `/tmp/swarm-pr-mcp-book-tools-schema/worker-4/book-metadata.png`. Pass when response contains title, description, and table of contents with chapter IDs.
- [ ] Lane 5. Call `read_chapter_content` with `book_id` and `chapter_id`. Save `/tmp/swarm-pr-mcp-book-tools-schema/worker-5/read-chapter-id.png`. Pass when paragraphs 1 through 50 return cleanly formatted.
- [ ] Lane 6. Call `read_chapter_content` with `book_id` and `chapter_index=2` with window `start_paragraph=10` and `end_paragraph=20`. Save `/tmp/swarm-pr-mcp-book-tools-schema/worker-6/read-window.png`. Pass when exact paragraph slice returns.
- [ ] Lane 7. Validate exported tool definition schema against OpenAI function declaration JSON schema spec. Save `/tmp/swarm-pr-mcp-book-tools-schema/worker-7/schema-valid.png`. Pass when schema contains type, properties, and required fields.
- [ ] Lane 8. In-process `ExecuteTool` direct invocation without network socket. Save `/tmp/swarm-pr-mcp-book-tools-schema/worker-8/in-process-exec.png`. Pass when execution completes synchronously.
- [ ] Lane 9. Call `search_library` with invalid/non-existent `book_id`. Save `/tmp/swarm-pr-mcp-book-tools-schema/worker-9/invalid-book.png`. Pass when tool returns empty passages without crashing.
- [ ] Lane 10. Call `read_chapter_content` with out-of-range `start_paragraph`. Save `/tmp/swarm-pr-mcp-book-tools-schema/worker-10/out-of-range.png`. Pass when tool returns descriptive error message.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. In-process tool execution latency in milliseconds.
- [ ] Probe. Benchmark in-process `ExecuteTool` for `get_book_metadata` on 50-chapter book.
- [ ] Baseline. Measure HTTP MCP tool invocation baseline of 18 ms.
- [ ] Rule. In-process execution latency must remain under 4 ms.

**Review gate.** None. `pr-mcp-book-tools-schema` is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Rebased onto current trunk after the verdict, patch-id unchanged.
- [ ] Append to base-branch stack.

---

## Implement agentic tool-calling chat engine with lean system prompt (pr-chat-agentic-tool-calling)

**Depends on.** `pr-mcp-book-tools-schema`.

**Files.**

- [ ] Edit `apps/server/internal/ai/client.go`.
- [ ] Edit `apps/server/internal/ai/ollama.go`.
- [ ] Edit `apps/server/internal/ai/openai.go`.
- [ ] Edit `apps/server/internal/ai/google.go`.
- [ ] Create `apps/server/internal/ai/chat_agent.go`.
- [ ] Create `apps/server/internal/ai/chat_agent_test.go`.
- [ ] Edit `apps/server/internal/api/books.go`.
- [ ] Edit `apps/server/internal/api/router.go`.
- [ ] Edit `apps/server/internal/api/api_test.go`.
- [ ] Edit `apps/server/cmd/server/main.go`.

**Build.**

- [ ] Add function-calling types to `apps/server/internal/ai/client.go` (`ToolDefinition`, `ToolCall`, `ChatTurnResponse`).
- [ ] Extend `ChatClient` interface with `ChatWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition) (*ChatTurnResponse, error)`.
- [ ] Implement `ChatWithTools` across providers:
  - OpenAI: pass `tools` array, parse stringified arguments in `tool_calls[i].function.arguments`, preserve `tool_call_id`.
  - Ollama: pass `tools` parameter to `/api/chat`, parse polymorphic argument payload via `json.RawMessage`.
  - Google Gemini: transform schema types to uppercase (`OBJECT`, `STRING`), format tool output with object envelope (`response: {"output": ...}`).
- [ ] In `apps/server/cmd/server/main.go`, construct `ToolExecutor` independently of `cfg.MCP.Enabled` and pass it to `api.NewRouter` so internal book chat always has tool access.
- [ ] Implement `ChatAgent` in `apps/server/internal/ai/chat_agent.go`:
  - **Programmatic Book Scoping Wrapper**: Programmatically enforce `args["book_id"] = targetBook.ID` on all tool calls (`search_library`, `get_book_metadata`, `read_chapter_content`), preventing prompt injection attacks from breaking out to other books.
  - Limit parallel tool calls to a maximum of 3 per turn.
  - Wrap retrieved passages in `<retrieved_passage>` XML tags instructing the model that excerpts are passive text.
  - Multi-turn execution loop capped at 4 turns with **Guaranteed Synthesis Pattern**: on turn 4, execute pending tools and invoke the model with `tools=nil` to guarantee a non-empty textual response.
  - Tool error resilience: feed execution errors back as tool message turns so the model can auto-correct rather than failing with status 500.
  - Request context timeout propagation (45s ceiling).
- [ ] Refactor `POST /api/v1/books/{id}/chat` (`ChatBook` in `apps/server/internal/api/books.go`):
  - Completely eliminate upfront RAG paragraph embedding and context stuffing.
  - Construct lean system prompt containing only: Book ID (`book.ID`), Book Title (`book.Title`), Book Description (`book.Description`), and Author/Series names.
  - Prompt directive: "You are an insightful literary companion assisting a reader with the book \"%s\" (Book ID: `%s`). Answer the reader's question thoughtfully and accurately. Primarily answer questions from the current book. Use the connected Shelfd book tools with Book ID `%s` to look up information, read chapters, or retrieve passages whenever needed."
  - Delegate execution to `ChatAgent` and return `BookChatResponse{Reply: reply, Citations: citations}`.

**You see.**

- [ ] Log line: `chat: agent completed query for book "Moby Dick" with 2 tool calls and 3 citations`.
- [ ] HTTP response returns status 200 with dynamic `citations` matching executed tool queries.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Unit tests in `apps/server/internal/ai/chat_agent_test.go` verify tool-calling loops, multi-provider argument formats, programmatic book_id enforcement, guaranteed synthesis on turn 4, context timeout cancellation, and citation accumulation. Run `mise exec -- go test ./internal/ai/...`.
- [ ] Unit tests in `apps/server/internal/api/api_test.go` verify `POST /api/v1/books/{id}/chat` returns 200 with reply and citations without upfront context stuffing. Run `mise exec -- go test ./internal/api/...`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on `grok-4.7-xhigh-fast` at the PR head, per the boot recipe. Use `temperature: 0.0` for repeatable verification.

- [ ] Lane 1. Regression lane against trunk. Execute standard reader chat query on trunk and head. Record prompt token size on both. Save `/tmp/swarm-pr-chat-agentic-tool-calling/worker-1/regression-prompt.png`. Pass when head answers accurately with > 70% fewer upfront prompt tokens.
- [ ] Lane 2. Specific plot question triggers `search_library` tool call. Post "What happened during the storm in Chapter 4?". Save `/tmp/swarm-pr-chat-agentic-tool-calling/worker-2/plot-tool-call.png`. Pass when server log confirms `search_library` invoked and response answers with storm details.
- [ ] Lane 3. Table of contents / structure query triggers `get_book_metadata` tool call. Post "How many chapters are in this book and what are their titles?". Save `/tmp/swarm-pr-chat-agentic-tool-calling/worker-3/toc-tool-call.png`. Pass when log confirms `get_book_metadata` invoked.
- [ ] Lane 4. Exact quote / paragraph query triggers `read_chapter_content` tool call. Post "What are the exact opening sentences of Chapter 1?". Save `/tmp/swarm-pr-chat-agentic-tool-calling/worker-4/read-chapter-call.png`. Pass when log confirms `read_chapter_content` invoked and exact text quoted.
- [ ] Lane 5. Multi-turn tool execution. Model calls `get_book_metadata` to find chapter index, then calls `read_chapter_content` to inspect it. Save `/tmp/swarm-pr-chat-agentic-tool-calling/worker-5/multi-tool.png`. Pass when both tools execute in sequence before final answer.
- [ ] Lane 6. General question answered without tool calls. Post "What is the title of this book?". Save `/tmp/swarm-pr-chat-agentic-tool-calling/worker-6/no-tool-needed.png`. Pass when model answers directly from lean prompt metadata without calling tools.
- [ ] Lane 7. Programmatic book scoping security verification. Post a prompt injection attempting to search book ID "book-other". Save `/tmp/swarm-pr-chat-agentic-tool-calling/worker-7/book-scoping.png`. Pass when server log proves tool call was forced to active book ID.
- [ ] Lane 8. Max turn ceiling protection with guaranteed synthesis. Simulate a model that repeatedly requests tool calls. Save `/tmp/swarm-pr-chat-agentic-tool-calling/worker-8/turn-ceiling.png`. Pass when loop halts at turn 4 and returns a synthesized non-empty response.
- [ ] Lane 9. Dynamic citation verification. Save `/tmp/swarm-pr-chat-agentic-tool-calling/worker-9/citations-payload.png`. Pass when response `citations` array contains exact passages retrieved by tool calls.
- [ ] Lane 10. Backward compatibility verification. Send request with legacy `history` payload. Save `/tmp/swarm-pr-chat-agentic-tool-calling/worker-10/history-compat.png`. Pass when past conversation turns are preserved in chat context.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Initial system prompt character count for standard book chat request.
- [ ] Probe. Measure initial prompt string length constructed in `ChatBook`.
- [ ] Baseline. Measure trunk prompt baseline of ~3,500 characters (~900 tokens) with 5 pre-loaded passage chunks.
- [ ] Rule. Head prompt character count must remain under 800 characters (< 200 tokens), achieving > 75% prompt token reduction.

**Review gate.** None. `pr-chat-agentic-tool-calling` is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Rebased onto current trunk after the verdict, patch-id unchanged.
- [ ] Append to base-branch stack.

---

## Verify client chat experience and dynamic citations (pr-chat-client-ux-verification)

**Depends on.** `pr-chat-agentic-tool-calling`.

**Files.**

- [ ] Edit `apps/app/lib/data/models/book_chat.dart`.
- [ ] Edit `apps/app/lib/ui/features/book_detail/book_detail_view.dart`.
- [ ] Edit `apps/app/lib/ui/state/providers.dart`.
- [ ] Create `apps/app/test/data/models/book_chat_test.dart`.
- [ ] Edit `apps/app/test/ui/features/book_detail_view_test.dart`.
- [ ] Edit `apps/app/test/data/services/api_service_test.dart`.

**Build.**

- [ ] Update `BookCitation` in `apps/app/lib/data/models/book_chat.dart` to include `startParagraph`, `endParagraph`, and `excerpt` with safe `fromJson`/`toJson` handling.
- [ ] Update `BookDetailView` reader chat interface in `apps/app` to present dynamic citations retrieved by the backend MCP tools.
- [ ] Enhance citation chips with chapter badges, paragraph index indicators, and tap-to-expand preview bottom sheets (mobile) / popovers (desktop) showing excerpt text with a "Read in Reader" action button.
- [ ] Deduplicate citations by chapter ID and paragraph range before rendering the citation chip row.
- [ ] Render a staged status indicator while the unary HTTP chat request is in-flight (`isSending: true`), rotating copy on a timer (`"Consulting book tools..."` → `"Retrieving chapter passages..."`).
- [ ] Add a "Retry" button to the error banner in `_buildChatTab` for convenient one-tap retry on timeouts or network errors.
- [ ] Configure explicit 60-second client timeout on `ApiService.chatWithBook`.
- [ ] Maintain theme styling across obsidian dark and bone white themes with high contrast ratios (>9.5:1).
- [ ] Add unit and widget tests verifying model serialization, message rendering, citation interactions, and retry flows.

**You see.**

- [ ] Screen renders reader chat sheet with user message, assistant response, and interactive citation chips for passages retrieved by tools.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Unit tests in `apps/app/test/data/models/book_chat_test.dart` verify serialization and deserialization of `BookCitation` with paragraph ranges and excerpts. Run `mise exec -- flutter test test/data/models/book_chat_test.dart`.
- [ ] Unit tests in `apps/app/test/data/services/api_service_test.dart` verify parsing of chat response payloads with dynamic citations. Run `mise exec -- flutter test test/data/services/api_service_test.dart`.
- [ ] Widget tests in `apps/app/test/ui/features/book_detail_view_test.dart` verify message rendering, citation chip tap interactions, preview sheet opening, and retry button clicks. Run `mise exec -- flutter test test/ui/features/book_detail_view_test.dart`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on `grok-4.7-xhigh-fast` at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Launch reader chat on trunk and head. Save `/tmp/swarm-pr-chat-client-ux-verification/worker-1/chat-regression.png`. Pass when chat launches cleanly on both without UI regressions.
- [ ] Lane 2. Open book detail view and switch to Chat / Ask Shelfd tab. Save `/tmp/swarm-pr-chat-client-ux-verification/worker-2/chat-tab.png`. Pass when lean book title and suggested query chips display.
- [ ] Lane 3. Submit question "What happens to the protagonist?". Save `/tmp/swarm-pr-chat-client-ux-verification/worker-3/send-message.png`. Pass when staged loading status displays while backend executes tools.
- [ ] Lane 4. Assistant message arrival. Save `/tmp/swarm-pr-chat-client-ux-verification/worker-4/message-received.png`. Pass when markdown text renders cleanly with citation chips below the bubble.
- [ ] Lane 5. Tap citation chip to inspect cited passage. Save `/tmp/swarm-pr-chat-client-ux-verification/worker-5/citation-modal.png`. Pass when modal sheet displays chapter title, paragraph range, excerpt text, and "Read in Reader" button.
- [ ] Lane 6. Follow-up conversational turn. Submit "Tell me more about the ending." Save `/tmp/swarm-pr-chat-client-ux-verification/worker-6/follow-up.png`. Pass when conversation history persists and new citations append.
- [ ] Lane 7. Clear chat button interaction. Tap clear conversation icon. Save `/tmp/swarm-pr-chat-client-ux-verification/worker-7/clear-chat.png`. Pass when chat resets to clean initial state.
- [ ] Lane 8. Responsive layout check on mobile viewport (375x667). Save `/tmp/swarm-pr-chat-client-ux-verification/worker-8/mobile-layout.png`. Pass when input bar, message bubbles, and citation chips wrap without overflow.
- [ ] Lane 9. Responsive layout check on desktop viewport (1440x900). Save `/tmp/swarm-pr-chat-client-ux-verification/worker-9/desktop-layout.png`. Pass when chat tab integrates cleanly into book detail layout.
- [ ] Lane 10. Theme toggle verification. Toggle between dark and light themes. Save `/tmp/swarm-pr-chat-client-ux-verification/worker-10/theme-toggle.png`. Pass when citation pills, borders, and text contrast conform to Shelfd design tokens.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. UI frame render time during chat scrolling with active citations.
- [ ] Probe. Flutter driver performance profile scrolling 20-message chat history.
- [ ] Baseline. Measure book detail scroll frame baseline of 9.1 ms.
- [ ] Rule. 99th percentile frame render time during chat interaction must stay under 16.6 ms (60 fps).

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 4, 5, and 8 screenshots into `docs/screenshots/pr-chat-client-ux-verification-review-chat.png`, `docs/screenshots/pr-chat-client-ux-verification-review-citation.png`, and `docs/screenshots/pr-chat-client-ux-verification-review-mobile.png`.
- [ ] Post the screenshots and live receipts in chat. Stop at merge-ready. Wait for the operator's review.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Rebased onto current trunk after the verdict, patch-id unchanged.
- [ ] Append to base-branch stack and let operator land stack.

---

## Close the program

- [ ] Every box above is checked with its evidence.
- [ ] Reply to the operator with the report the execution playbook names.

---

## Appendix A. Prototype evidence

Verification of tool calling and MCP decoupling confirmed that:
1. Decoupled AI endpoints allow homelabs to run resource-efficient embedding extraction locally (e.g. `nomic-embed-text` on Ollama or Apple Silicon MLX) while routing conversational reader chat to reasoning-optimized models (e.g. `gpt-4o-mini`, `gemini-1.5-flash`, or a local quantized LLM) with independent credentials and URLs.
2. Lean system prompt injection eliminates upfront RAG token waste. Rather than dumping 5 paragraphs (800–1,200 tokens) on every single turn regardless of need, the chat model begins with < 200 tokens of book metadata and only queries the MCP server when specific passage lookup or chapter reading is required.
3. In-process MCP tool invocation via `ToolExecutor` eliminates network loopback overhead, token hashing latency, and HTTP round-trips while reusing the exact same MCP tools (`search_library`, `get_book_metadata`, `read_chapter_content`).
4. Programmatic book scoping in `ChatAgent` guarantees that the model cannot be coerced into reading or searching books outside the currently viewed title, solving prompt injection security concerns.

## Appendix B. Alternatives rejected

- Shared monolithic AI configuration retained: Rejected. Forcing the embedder and chat model to share a single `base_url` and `api_key` prevented common homelab architectures (local embeddings with cloud chat, or cloud embeddings with private on-prem chat).
- Upfront RAG context stuffing in system prompt: Rejected. Eagerly injecting paragraphs into the system prompt bloats the prompt context on simple queries (e.g. "Who wrote this?"), wastes tokens, and prevents the model from choosing to browse chapters or search other passages dynamically.
- Spawning internal HTTP/SSE network calls to Shelfd's own MCP server for reader chat: Rejected. Direct in-process execution via `mcp.ToolExecutor` achieves identical tool logic with zero HTTP serialization overhead and zero credential handling complexity.
- Proliferating redundant tools (e.g. adding `search_book` alongside `search_library`): Rejected. Dual search tools confuse smaller open-weight models (e.g. `llama3.2:3b`); adding an optional `book_id` parameter to `search_library` keeps the tool schema clean and unambiguous.
- Relying on system prompt natural language for access control: Rejected. Natural language cannot enforce authorization boundaries; `ChatAgent` programmatically enforces `args["book_id"] = activeBookID`.
- Single monolithic pull request: Rejected. Dividing the feature into four distinct pull requests isolates configuration decoupling, MCP schema extensions, agentic chat loops, and Flutter UI verification into independently verifiable units.

## Appendix C. Risks

- Infinite tool loops with remote models: Addressed by enforcing a strict maximum turn ceiling of 4 turns in `ChatAgent` and invoking the Guaranteed Synthesis Pattern on turn 4 (`tools=nil`).
- Chat models hallucinating rather than using tools: Addressed by explicit system prompt directives stating the Book ID and instructing the model to retrieve facts via tools rather than guessing.
- Heterogeneous function-calling formats across providers: Addressed by provider-specific adapters in `apps/server/internal/ai` translating to and from unified `ToolDefinition` and `ToolCall` structures (handling stringified arguments in OpenAI, raw JSON in Ollama, and uppercase types in Gemini).
- SSRF vulnerabilities in decoupled base URLs: Addressed by running dial-time SSRF and IP validation across IPv4 and IPv6 metadata ranges on both `embedder.base_url` and `chat.base_url`, and re-validating after environment variable overrides.
- Missing excerpt and paragraph fields in client citation model: Addressed by updating `BookCitation` in `apps/app/lib/data/models/book_chat.dart` in PR 4.

## Appendix D. Links and reading list

- Autopilot Stack Playbook in `pstack/skills/poteto-mode/playbooks/autopilot-stack.md`.
- Model Context Protocol (MCP) Tools in `apps/server/internal/mcp/tools.go`.
- Server Configuration in `apps/server/internal/config/config.go`.
- Principle Model The Domain in `pstack/skills/principle-model-the-domain/SKILL.md`.
- Principle Boundary Discipline in `pstack/skills/principle-boundary-discipline/SKILL.md`.
- Principle Sequence Verifiable Units in `pstack/skills/principle-sequence-verifiable-units/SKILL.md`.
- Principle Prove It Works in `pstack/skills/principle-prove-it-works/SKILL.md`.
