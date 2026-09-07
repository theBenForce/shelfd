# Semantic Pipeline Evaluation Manual

This technical manual instructs the AI on evaluating chapter summary quality, embedding dimension alignment, and vector retrieval accuracy in Shelfd.

## Protocols

### 1. Summary Prompt Constraints
* Summaries must be 2–4 sentences per chapter.
* Focus on:
  * Key character actions, decisions, and character arcs.
  * Core thematic concepts and subject keywords.
  * Essential plot progression or factual arguments.
* Avoid generic filler phrases like "In this chapter, the narrative continues...".

### 2. Dimension Consistency Check
* Ensure the dimension declared in `config.yaml` (`ai.embedding_dimensions`) strictly matches the `vec0` virtual table definition in SQLite:
  ```sql
  CREATE VIRTUAL TABLE vec_chapters USING vec0(chapter_id TEXT PRIMARY KEY, embedding float[1536]);
  ```
* Any change in embedding models requires migrating the table schema and re-indexing existing chapters.

### 3. Recall & Distance Verification
* Test sample natural language queries against known chapter summaries.
* Ensure relevant search hits produce distance scores below the acceptable similarity cutoff (< 0.70).
