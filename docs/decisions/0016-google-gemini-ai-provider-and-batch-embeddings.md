# Google Gemini AI Provider & Unified Ingestion Batch Embeddings

* Status: accepted
* Deciders: Lead Systems Architect, @core, @librarian, @mcp
* Date: 2026-09-09

## Context and Problem Statement

Following the adoption of per-paragraph 256-dimension vector embeddings (ADR-0014), Shelfd eliminated the generative LLM ingestion bottleneck. However, the background ingestion worker still embedded paragraph passages sequentially (one HTTP request per paragraph) and retained a vestigial fallback loop for chapter summarization. Furthermore, Shelfd only supported Ollama and OpenAI-compatible endpoints.

How should external AI providers and the ingestion worker be structured to:
1. Support Google Gemini (`text-embedding-004` and `gemini-1.5-flash`) as a first-class AI provider?
2. Unify vector ingestion into a provider-agnostic batch embedding pipeline that achieves 30x–60x speedups during library indexing?
3. Enforce that library ingestion is purely vector-based with zero LLM generation calls?

## Decision Drivers

* **Ingestion Throughput**: Indexing a 100+ book library (~50,000 paragraphs) must complete in minutes rather than hours.
* **Provider-Agnostic Ingestion**: The worker and repository must not contain provider-specific branching (`if provider == "google"`). All providers must conform to identical batch interfaces.
* **Zero Generative Ingestion Overhead**: Eliminate lingering chapter summarization during indexing; LLMs are strictly invoked on-demand for reader RAG chat (`POST /api/v1/books/{id}/chat`).
* **Cost Ergonomics**: Keep full library ingestion cost under $2.00 for external cloud providers.
* **Native 256-Dimension Compatibility**: Preserve SQLite `vec0` 256-float tables without schema alterations.

## Considered Options

* **Unified `GenerateBatchEmbeddings` Interface with Google Gemini, Ollama, and OpenAI (Chosen)**
* **Google-Only Custom Batch Worker**
* **Retain Single-Pass Sequential Ingestion**

## Decision Outcome

Chosen option: **Unified `GenerateBatchEmbeddings` Interface with Google Gemini, Ollama, and OpenAI**.

1. **Google Gemini Client (`GoogleClient`)**:
   * Communicates with Google's REST API (`https://generativelanguage.googleapis.com`).
   * **Batch Embeddings**: Calls `POST /v1beta/models/{embedding_model}:batchEmbedContents` in chunks of up to 100 texts with `outputDimensionality: 256`. Google TPUs handle native Matryoshka Representation Learning (MRL) truncation.
   * **Chat Model**: Calls `POST /v1beta/models/{chat_model}:generateContent` with `systemInstruction` and role mapping (`user`, `model`).
   * **Authentication**: Uses `x-goog-api-key: <key>` header for secure, log-safe credential transmission.
2. **Unified `ai.Client` Batch Abstraction**:
   * Extended `ai.Client` with `GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error)`.
   * OpenAI uses `/v1/embeddings` with input arrays; Ollama uses `/api/embed` (with legacy fallback); Google uses `batchEmbedContents`.
   * Shared `ChunkTexts(texts, maxBatchSize)` helper handles provider-specific payload batch ceilings.
3. **Transactional Vector Persistence**:
   * Added `InsertParagraphVectors(ctx, items []ParagraphVector)` to `StorageEngine`.
   * In SQLite and PostgreSQL/Bun, batch inserts are wrapped inside a single atomic transaction, preventing hundreds of individual disk syncs per batch.
4. **Purging Ingestion Summarization**:
   * Completely removed the residual `GetUnindexedChapters` summarization loop from `worker.go`. Ingestion is strictly vector embedding of paragraph passages.
   * Clarified configuration naming: `chat_model` is used for interactive reader chat, with `summary_model` retained as a backward-compatible alias.

## Consequences

### Positive Consequences

* **30x–60x Ingestion Speedup**: Ingestion of a 115-book library (~52,000 paragraphs) drops from ~2 hours down to ~2–4 minutes using Google `batchEmbedContents`.
* **Sub-$1.50 Full Library Ingestion**: Without chapter summarization during indexing, embedding a 52,000-paragraph library costs ~$1.28 on Google `text-embedding-004`.
* **Zero Architectural Drift**: `worker.go` contains zero provider-specific conditionals. Swapping between local Ollama, OpenAI, or Google requires only a configuration change.
* **Schema Stability**: `outputDimensionality: 256` fits directly into existing SQLite `vec_paragraphs USING vec0(embedding float[256])`.

### Negative Consequences

* **Cloud API Dependency**: Ingesting via Google or OpenAI sends paragraph chunks across WAN and requires valid API keys (unlike local Ollama which remains 100% offline).
