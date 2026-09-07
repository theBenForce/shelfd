# Ingestion & Semantic Retrieval — Chapter-Level Summaries

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team
* Date: 2026-09-07

## Context and Problem Statement

Semantic search requires converting text into vector embeddings. If an ebook library is indexed at the paragraph level with sliding windows, a 300-book collection produces roughly 500,000 vectors, incurring heavy initial LLM embedding API costs, long ingest times, and fragmented search hits. How should text be chunked and indexed for the MVP?

## Decision Drivers

* High semantic relevance for high-level thematic queries across books.
* Low initial ingestion cost and fast time-to-first-search.
* Low storage footprint (< 50MB for embeddings).
* Natural upgrade path to full-text chunking in Phase 2.

## Considered Options

* **Chapter-Level Summaries + Chapter Vector Embeddings**
* **Full-Text Paragraph Chunking (Sliding Window ~500 tokens)**
* **Book-Level Metadata & Synopsis Only**

## Decision Outcome

Chosen option: **Chapter-Level Summaries + Chapter Vector Embeddings**, using an external or self-hosted LLM (Ollama or OpenAI-compatible endpoint) to summarize each chapter, then embedding the summaries into `sqlite-vec`.

### Positive Consequences

* Produces ~20 vectors per book (~6,000 vectors for 300 books).
* Vectors fit easily in memory and can be scanned in < 1ms using CPU SIMD instructions in `sqlite-vec`.
* Chapter summaries capture thematic concepts, major character arcs, and core arguments without losing narrative context.
* Search results link directly to chapter boundaries, making it easy for the reader or MCP agent to fetch the surrounding text.

### Negative Consequences

* Fine-grained, quote-specific retrieval (finding a specific sentence) is limited compared to full-text chunking.
* Requires an external LLM call per chapter during ingest.

## Pros and Cons of the Options

### Chapter-Level Summaries

* Good, because keeps vector count low and search latency sub-millisecond.
* Good, because summaries distill noisy prose into dense semantic concepts.
* Good, because preserves clear navigational structure (Book -> Chapter).
* Bad, because cannot match verbatim quotes if the concept wasn't captured in the summary.

### Full-Text Paragraph Chunking

* Good, because allows pinpoint sentence-level search.
* Bad, because produces hundreds of thousands of vectors, increasing memory and storage requirements.
* Bad, because fragmented paragraphs lose broader narrative context without complex parent-document retrieval.

### Book-Level Only

* Good, because cheapest and fastest to index.
* Bad, because book-level blurbs are too coarse to answer specific topical questions.
