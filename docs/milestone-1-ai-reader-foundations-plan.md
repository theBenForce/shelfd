# AI Reader Foundations plan

This plan delivers Milestone 1 for Shelfd readers, MCP agent users, and backend maintainers.
It adds anti-spoiler "Previously On" recaps for issue 17, cross-book synoptic synthesis for issue 20, and MCP plus REST study deck exports for issue 21.
The program enforces SQL-level progression bounds for anti-spoiler queries, dual-dialect SQLite and Postgres schema parity, and zero new Go dependencies for Anki `.apkg` packaging.
The pull requests in order are `pr-m1-scoped-retrieval-cache`, `pr-m1-mcp-study-decks`, `pr-m1-previously-on-recap`, `pr-m1-synoptic-reader-api`, `pr-m1-study-deck-export-ui`, and `pr-m1-synoptic-studio-ui`.

## How to read this

One box is one unit of work. Every box names the evidence that checks it. A nested box is a sub-step of the box above it. Check a box only when its evidence exists, a file, a log line, a screenshot, a test run, or a SHA. The body is a how-to. The appendices explain and record.

The program runs `skills/poteto-mode/playbooks/autopilot-stack.md` under the installed plugin `.agents/plugins/pstack`. The operator reviews the three UI pull requests (`pr-m1-previously-on-recap`, `pr-m1-study-deck-export-ui`, and `pr-m1-synoptic-studio-ui`) and lands the verified bottom-to-top stack in order.

Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

## Program checklist

### Arm the program

- [ ] State the protocol and this plan to the operator, then stop. Start execution only on the operator's explicit go.
- [ ] On the operator's go, write the program objective into the standing orders and your todolist with this exact text. "docs/milestone-1-ai-reader-foundations-plan.md, pr-m1-scoped-retrieval-cache, pr-m1-mcp-study-decks, pr-m1-previously-on-recap, pr-m1-synoptic-reader-api, pr-m1-study-deck-export-ui, pr-m1-synoptic-studio-ui, verification rule, operator merges stack, all boxes checked."
- [ ] Read these from the installed plugin at program start. Re-read them at every tick.
  - [ ] `.agents/plugins/pstack/skills/poteto-mode/playbooks/autopilot-stack.md`
  - [ ] `.agents/plugins/pstack/skills/swarm/SKILL.md`
  - [ ] `.agents/skills/verify-shelfd/SKILL.md`
  - [ ] `.agents/plugins/pstack/skills/poteto-mode/playbooks/opening-a-pr.md`
  - [ ] `.agents/plugins/pstack/skills/principle-model-the-domain/SKILL.md`
  - [ ] `.agents/plugins/pstack/skills/principle-foundational-thinking/SKILL.md`
- [ ] Arm the 30-minute audit tick as a real cadence. Never leave the cadence to memory.
- [ ] Use this tick prompt, verbatim. "Re-read the execution playbook from the installed plugin and the standing orders. Audit the operation against both and fix drift in this tick. Probe every active lane and judge progress by side effects only. Stand down a lane only on affirmative failure evidence, and dispatch its replacement in the same tick. Then send the operator a status message, whether or not anything changed, with the queue table of PR, owner, state, and head SHA, the verdicts since the last tick, what merged, open operator gates, and blockers."
- [ ] On the operator's hold or stand-down, send every owner a zero-writes order at once.

### Spawn owners

- [ ] Spawn one owner per PR with the full lifecycle the execution playbook names.
- [ ] Follow this dependency graph. Start dependent work only after its parent merges, or rebase its branch onto the parent's exact tip when the execution playbook stacks. A same-repository child PR targets its parent branch. A fork child PR targets trunk while retaining local parent ancestry. Freeze the bottom-to-top order because fork PR bases do not encode it.
  - [ ] `pr-m1-scoped-retrieval-cache` and `pr-m1-mcp-study-decks` are independent and first. Both branch from `main`.
  - [ ] `pr-m1-previously-on-recap` after `pr-m1-scoped-retrieval-cache`.
  - [ ] `pr-m1-synoptic-reader-api` after `pr-m1-scoped-retrieval-cache`.
  - [ ] `pr-m1-study-deck-export-ui` after `pr-m1-mcp-study-decks`.
  - [ ] `pr-m1-synoptic-studio-ui` after `pr-m1-synoptic-reader-api`.
- [ ] Hold the file boundaries. Backend PRs (`pr-m1-scoped-retrieval-cache`, `pr-m1-synoptic-reader-api`, and `pr-m1-mcp-study-decks`) touch only `apps/server/**`. UI PRs (`pr-m1-study-deck-export-ui` and `pr-m1-synoptic-studio-ui`) touch only `apps/app/**`. `pr-m1-previously-on-recap` touches `apps/server/internal/ai/recap*`, `apps/server/internal/api/recap*`, `apps/server/internal/api/router.go`, and `apps/app/**`.
- [ ] Hold the review gate. `pr-m1-previously-on-recap`, `pr-m1-study-deck-export-ui`, and `pr-m1-synoptic-studio-ui` change an interaction. They wait for the operator's review in chat with screenshots and a video before merge.

### PR mechanics, for every PR

- [ ] Resolve the forge once. Default to `gh`; if `command -v origin` succeeds and Origin can resolve the repository, use `origin pr` for every PR operation. Record any fallback to `gh`. Record the intended PR base repository as canonical `theBenForce/shelfd` and validate it through the active forge. Do not infer it from the checkout's default remote. Capture it as a shell variable and pass `--repo "$base_repo"` to every `gh pr` command. Resolve and validate `<head-url>` through Shipping step 1 and capture it as `head_url` for the live-lane fetch. When the head repository is a fork, validate its identity and record its owner and repository name as `<fork-owner>` and `<head-name>`. Never require `gt`.
- [ ] Open the PR before self-proof and follow the readiness rule in `skills/poteto-mode/playbooks/opening-a-pr.md`. Open it ready by default. When repository instructions require a draft until named evidence exists, keep it draft until that evidence is recorded. Use `origin pr create --status open --base "$base_branch"` or `gh pr create --base "$base_branch" --repo "$base_repo"` for a ready same-repository PR. A same-repository stack child targets its parent branch. Every fork PR targets trunk. With GitHub, capture the approved PR title and body as `<title>` and `<body>`, then create it with `gh api --method POST "repos/$base_repo/pulls" -f "title=$title" -f "body=$body" -f "head=$fork_owner:$branch" -f "head_repo=$head_name" -f "base=$trunk" --jq .html_url`; add `-F draft=true` when repository instructions require a draft. Otherwise use the resolved Origin command. Stacked fork branches retain local parent ancestry.
- [ ] Run `pnpm lint` and `pnpm test` once before the PR-facing push. Push with hooks on.
- [ ] Run `/deslop` before each commit and `/no-comments` before review.
- [ ] Triage every Bugbot and security-reviewer comment per `skills/poteto-mode/references/bugbot-triage.md` under the installed plugin.
- [ ] Before babysit, rebase each independent PR and stack root onto current trunk. Rebase each unmerged stack child onto its parent's exact tip. After its parent merges, use Shipping's explicit old-base-to-trunk rebase before the child's merge-ready report.

### Verdict and merge, for every PR

- [ ] At the merge-ready head SHA, run the swarm per `skills/swarm/SKILL.md`. One gates lane. The ten live lanes from the PR's **Verify, live** block. The perf lane from its **Verify, perf** block. One audit lane that reads the diff and the receipts and distrusts the PR body.
- [ ] Clean only when every lane is `PASS`. Findings go back to the owner. A new head gets a fresh swarm and a fresh verdict.
- [ ] On a clean verdict, append the PR to the frozen bottom-to-top stack in verified order, never merge or arm auto-merge. Record the verdict SHA, current landing SHA, recorded patch base, and `git patch-id` per `skills/poteto-mode/playbooks/shipping.md`. Preserve the verdict across rebases only when the patch ID stays unchanged.

### Boot recipe, for every live lane

Each live lane is one `swarm workers` lane at the PR head, resolved through provider dispatch, in its own worktree or output directory, with its own receipt. Drive the surface only through `.agents/skills/verify-shelfd/scripts/control-shelfd`.

- [ ] `git fetch -- "$head_url" "refs/heads/$head_branch" && git checkout --detach "$head_sha"` in the lane's worktree.
- [ ] Export unique `SHELFD_VERIFY_RUN_ID`, `SHELFD_VERIFY_PORT`, `SHELFD_VERIFY_AI_PORT`, and `SHELFD_VERIFY_CDP_PORT` values, start `.agents/skills/verify-shelfd/scripts/control-shelfd launch --foreground` as a daemon, and run `.agents/skills/verify-shelfd/scripts/control-shelfd doctor` until healthy.
- [ ] Deliver input only through `.agents/skills/verify-shelfd/scripts/control-shelfd` (`browser`, `api`, `mcp`, `upload`, `cli`). Capture `.aria.txt` snapshots and `browser console --assert-clean` logs as read-only diagnostics, then run `.agents/skills/verify-shelfd/scripts/control-shelfd cleanup` and `.agents/skills/verify-shelfd/scripts/control-shelfd evidence`.
- [ ] Save every screenshot to `/tmp/shelfd-verify-evidence/swarm-<pr-id>/worker-<n>/<slug>.png` and return the paths with the receipt path.

## Extend scoped retrieval and recap cache schema (pr-m1-scoped-retrieval-cache)

**Depends on.** None.

**Files.**

- [ ] Create `apps/server/internal/database/migrations/0021_recap_cache.sql`.
- [ ] Create `apps/server/internal/database/migrations_pg/0017_recap_cache.sql`.
- [ ] Edit `apps/server/internal/repository/models.go`.
- [ ] Edit `apps/server/internal/repository/storage.go`.
- [ ] Edit `apps/server/internal/repository/bun.go`.
- [ ] Edit `apps/server/internal/repository/sqlite.go`.
- [ ] Edit `apps/server/internal/api/books.go`.
- [ ] Edit `apps/server/internal/repository/sqlite_test.go`.
- [ ] Edit `apps/server/internal/repository/postgres_test.go`.

**Build.**

- [ ] Add `recap_cache` table migration in `0021_recap_cache.sql` and `0017_recap_cache.sql` with primary key `(book_id, chapter_index, paragraph_offset)` and `ON DELETE CASCADE` on `books(id)`.
- [ ] Add `BookIDs []string`, `AuthorIDs []string`, and `MaxChapterIndex *int` to `SearchFilter`, and add `RecapCacheEntry` struct in `apps/server/internal/repository/models.go`.
- [ ] Add `GetRecapCache(ctx, bookID, chapterIndex, paragraphOffset)` and `UpsertRecapCache(ctx, entry)` to `StorageEngine` in `apps/server/internal/repository/storage.go`.
- [ ] Implement multi-ID filtering, `c.chapter_index <= ?` ceiling enforcement, adaptive SQLite KNN `k` sizing, and recap cache methods in both `apps/server/internal/repository/bun.go` and `apps/server/internal/repository/sqlite.go`.
- [ ] Add `BookID`, `BookTitle`, and `AuthorName` JSON fields to `api.BookCitation` in `apps/server/internal/api/books.go` while keeping existing fields unchanged.

**You see.**

- [ ] `go test -tags sqlite_fts5 -ldflags="-linkmode=external" ./internal/database/... ./internal/repository/...` passes with migration `0021_recap_cache.sql` applied cleanly.
- [ ] Vector and FTS5 queries with `MaxChapterIndex` set to `2` return zero rows where `chapter_index > 2`.
- [ ] Vector and FTS5 queries with `BookIDs` or `AuthorIDs` return hits restricted to those IDs.
- [ ] Deleting a book cascades and deletes its `recap_cache` rows.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `apps/server/internal/repository/sqlite_test.go` adds tests for `SearchFilter.BookIDs`, `SearchFilter.AuthorIDs`, `SearchFilter.MaxChapterIndex`, and `GetRecapCache` plus `UpsertRecapCache` plus cascade delete. Run `cd apps/server && go test -tags sqlite_fts5 -ldflags="-linkmode=external" -v ./internal/repository/... ./internal/database/... ./internal/api/...`.
- [ ] `apps/server/internal/repository/postgres_test.go` compiles and exercises the Postgres migration and `BunStorageEngine` query builders when `SHELFD_TEST_POSTGRES_URL` is set. Run `cd apps/server && go vet -tags sqlite_fts5 ./...`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` role at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run `/connect` login, `api GET /api/v1/books`, and `api GET /api/v1/search?q=Gom+Jabbar` at trunk and head. Save `lane-01-trunk-regression.png`. Pass when both trunk and head return HTTP 200 with Dune hits and zero console errors.
- [ ] Lane 2. Verify fresh database startup applies `0021_recap_cache.sql` and `control-shelfd doctor` reports healthy. Save `lane-02-migration-doctor.png`. Pass when `/health` returns 200 and `doctor` exits 0.
- [ ] Lane 3. Verify `control-shelfd api GET /api/v1/books` and `/books` catalog load cleanly after schema upgrade. Save `lane-03-catalog-load.png`. Pass when Dune and The Tale of Peter Rabbit render in the ARIA snapshot.
- [ ] Lane 4. Verify `POST /api/v1/books/{id}/chat` returns citations containing `book_id`, `book_title`, and `author_name` fields. Save `lane-04-chat-citations.png`. Pass when response JSON includes the new citation fields and UI renders citations cleanly.
- [ ] Lane 5. Verify `control-shelfd mcp search_library '{"query":"Gom Jabbar"}'` returns expected hits without regression. Save `lane-05-mcp-search.png`. Pass when MCP JSON-RPC returns matching Dune paragraphs.
- [ ] Lane 6. Verify `control-shelfd upload` ingests `earthsea.epub` and runs background paragraph embedding without SQLite lock errors. Save `lane-06-upload-ingest.png`. Pass when Earthsea appears in `GET /api/v1/books` and ABS sidecars stay byte-identical.
- [ ] Lane 7. Verify deleting an uploaded test book via `DELETE /api/v1/books/{id}` removes its chapters and any cached recap rows cleanly. Save `lane-07-cascade-delete.png`. Pass when book delete returns 200 or 204 and subsequent lookup returns 404.
- [ ] Lane 8. Verify FTS5 search on `/search` returns results for multi-word queries across seeded books. Save `lane-08-fts-search.png`. Pass when search results render and console audit is clean.
- [ ] Lane 9. Verify server restart against the same `/data/sqlite.db` runs migrations idempotently with zero errors. Save `lane-09-idempotent-restart.png`. Pass when second boot passes `doctor` and preserves seeded books.
- [ ] Lane 10. Verify `control-shelfd cli -- -scan` completes without touching Audiobookshelf `.metadata.json` or `desc.txt` files. Save `lane-10-cli-scan.png`. Pass when `doctor` confirms sidecar SHA-256 checksums match baseline.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Median latency in milliseconds of `SearchVectorParagraphs` and `SearchFTSParagraphs` on the seeded library with single-book and multi-book filters.
- [ ] Probe. Run `go test -tags sqlite_fts5 -ldflags="-linkmode=external" -bench=BenchmarkSearchParagraphs -benchtime=200ms ./internal/repository/...` at trunk and head, interleaved for 5 rounds each.
- [ ] Baseline. Record the trunk median latency in milliseconds first before running head.
- [ ] Rule. Fail if head median latency regresses by more than 15 percent over trunk or exceeds 25 ms absolute latency for filtered SQLite queries on the seeded fixture.

**Review gate.** None. pr-m1-scoped-retrieval-cache is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] After the verdict, an owner merge uses Shipping step 4 to move the PR from its recorded patch base onto current trunk. An appended stack child keeps its recorded parent tip until that parent lands. Record the current landing SHA. Preserve the verdict only when the patch ID stays unchanged.
- [ ] Root appends `pr-m1-scoped-retrieval-cache` as a stack root targeting `main` and the operator lands it first.

## Add MCP study extraction tools and deck export API (pr-m1-mcp-study-decks)

**Depends on.** None.

**Files.**

- [ ] Create `apps/server/internal/study/extractor.go`.
- [ ] Create `apps/server/internal/study/anki.go`.
- [ ] Create `apps/server/internal/study/markdown.go`.
- [ ] Create `apps/server/internal/study/study_test.go`.
- [ ] Edit `apps/server/internal/mcp/protocol.go`.
- [ ] Edit `apps/server/internal/mcp/tools.go`.
- [ ] Edit `apps/server/internal/mcp/mcp_test.go`.
- [ ] Create `apps/server/internal/api/study.go`.
- [ ] Create `apps/server/internal/api/study_test.go`.
- [ ] Edit `apps/server/internal/api/router.go`.

**Build.**

- [ ] Define `StudyConcept`, `StudyCard`, and `StudyDeckFormat` (`apkg`, `csv`, `markdown`) types plus chapter-windowed LLM extraction in `apps/server/internal/study/extractor.go` with paragraph attribution on every card.
- [ ] Implement `BuildAnkiAPKG` (using `github.com/mattn/go-sqlite3` and stdlib `archive/zip`) and `BuildAnkiCSV` in `apps/server/internal/study/anki.go` so generated `.apkg` archives contain a valid `collection.anki2` SQLite database and `media` manifest.
- [ ] Implement `BuildMarkdownVaultZip` in `apps/server/internal/study/markdown.go` producing atomic Markdown concept notes, wiki backlinks, and chapter summary index files.
- [ ] Register `extract_key_concepts` and `generate_study_cards` MCP tools in `apps/server/internal/mcp/protocol.go` and dispatch them in `apps/server/internal/mcp/tools.go`.
- [ ] Add authenticated `POST /api/v1/books/{id}/study-deck` in `apps/server/internal/api/study.go` and wire it in `apps/server/internal/api/router.go`, returning `Content-Disposition` attachment bytes for `apkg`, `csv`, or `markdown` formats.

**You see.**

- [ ] `control-shelfd mcp extract_key_concepts '{"book_id":"<id>","chapter_index":1}'` returns structured JSON concepts with `start_paragraph` and `end_paragraph` citations.
- [ ] `control-shelfd mcp generate_study_cards '{"book_id":"<id>","chapter_index":1}'` returns question-answer flashcards grounded in the chapter text.
- [ ] `POST /api/v1/books/{id}/study-deck` with `format` set to `apkg` returns a ZIP archive containing `collection.anki2` with populated `notes` and `cards` SQLite tables.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `apps/server/internal/study/study_test.go` verifies `.apkg` unzip, SQLite `collection.anki2` schema integrity, `.csv` RFC 4180 escaping, and Markdown vault backlinks. Run `cd apps/server && go test -tags sqlite_fts5 -ldflags="-linkmode=external" -v ./internal/study/... ./internal/mcp/... ./internal/api/...`.
- [ ] `apps/server/internal/mcp/mcp_test.go` verifies `tools/list` advertises `extract_key_concepts` and `generate_study_cards` and rejects invalid chapter ranges with structured JSON-RPC errors. Run `cd apps/server && go vet -tags sqlite_fts5 ./...`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` role at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run `control-shelfd mcp search_library '{"query":"Gom Jabbar"}'` and `get_book_metadata` at trunk and head, and confirm head also lists `extract_key_concepts` and `generate_study_cards`. Save `lane-01-mcp-regression.png`. Pass when trunk and head pass existing tools and head exposes both new tools.
- [ ] Lane 2. Invoke `control-shelfd mcp extract_key_concepts` on Dune chapter 1. Save `lane-02-mcp-concepts.png`. Pass when response contains at least one concept with valid `chapter_index` and paragraph offsets.
- [ ] Lane 3. Invoke `control-shelfd mcp generate_study_cards` on Dune chapter 1. Save `lane-03-mcp-cards.png`. Pass when response contains non-empty `front`, `back`, and citation fields.
- [ ] Lane 4. Call `POST /api/v1/books/{id}/study-deck` with `format` equal to `apkg` and inspect the returned archive via `unzip -l` and `sqlite3` integrity check. Save `lane-04-apkg-export.png`. Pass when HTTP 200 returns a valid ZIP containing `collection.anki2` and `media`.
- [ ] Lane 5. Call `POST /api/v1/books/{id}/study-deck` with `format` equal to `csv` and verify UTF-8 CSV headers and rows. Save `lane-05-csv-export.png`. Pass when HTTP 200 returns `text/csv` attachment with valid rows.
- [ ] Lane 6. Call `POST /api/v1/books/{id}/study-deck` with `format` equal to `markdown` and inspect the ZIP entries. Save `lane-06-md-vault.png`. Pass when HTTP 200 returns a ZIP containing `README.md` and atomic concept `.md` files.
- [ ] Lane 7. Call `POST /api/v1/books/{id}/study-deck` with an unsupported format value. Save `lane-07-invalid-format.png`. Pass when server returns HTTP 400 with a JSON `error` field.
- [ ] Lane 8. Call `POST /api/v1/books/nonexistent/study-deck` and verify error handling. Save `lane-08-missing-book.png`. Pass when server returns HTTP 404.
- [ ] Lane 9. Call `POST /api/v1/books/{id}/study-deck` without authentication credentials. Save `lane-09-auth-guard.png`. Pass when server returns HTTP 401.
- [ ] Lane 10. Verify `/books` UI and `GET /api/v1/books/{id}/markdown` still work unchanged after adding study routes. Save `lane-10-existing-export-ok.png`. Pass when existing Markdown export returns HTTP 200 and console audit is clean.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Wall-clock latency in milliseconds to compile 50 extracted `StudyCard` records into an `.apkg` archive and a Markdown vault ZIP, plus `tools/list` MCP response latency at trunk and head.
- [ ] Probe. Run `go test -tags sqlite_fts5 -ldflags="-linkmode=external" -bench=BenchmarkBuildAnkiAPKG -benchtime=200ms ./internal/study/...` on head and interleave 20 `tools/list` requests against trunk and head.
- [ ] Baseline. Record the trunk `tools/list` median latency first before measuring head.
- [ ] Rule. Fail if `tools/list` regresses by more than 10 percent over trunk, or if compiling 50 cards into `.apkg` exceeds 250 ms.

**Review gate.** None. pr-m1-mcp-study-decks is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] After the verdict, an owner merge uses Shipping step 4 to move the PR from its recorded patch base onto current trunk. An appended stack child keeps its recorded parent tip until that parent lands. Record the current landing SHA. Preserve the verdict only when the patch ID stays unchanged.
- [ ] Root appends `pr-m1-mcp-study-decks` as an independent root targeting `main` and the operator lands it before `pr-m1-study-deck-export-ui`.

## Deliver anti-spoiler Previously On recap backend and UI (pr-m1-previously-on-recap)

**Depends on.** `pr-m1-scoped-retrieval-cache`.

**Files.**

- [ ] Create `apps/server/internal/ai/recap.go`.
- [ ] Create `apps/server/internal/ai/recap_test.go`.
- [ ] Create `apps/server/internal/api/recap.go`.
- [ ] Create `apps/server/internal/api/recap_test.go`.
- [ ] Edit `apps/server/internal/api/router.go`.
- [ ] Create `apps/app/lib/data/models/recap.dart`.
- [ ] Edit `apps/app/lib/data/services/api_service.dart`.
- [ ] Edit `apps/app/lib/data/repositories/book_repository.dart`.
- [ ] Edit `apps/app/lib/ui/state/providers.dart`.
- [ ] Create `apps/app/lib/ui/features/recap/previously_on_sheet.dart`.
- [ ] Edit `apps/app/lib/ui/features/book_detail/book_detail_view.dart`.
- [ ] Edit `apps/app/lib/ui/features/reader/reader_view.dart`.
- [ ] Edit `apps/app/lib/ui/features/reader/fixed_layout_reader.dart`.
- [ ] Create `apps/app/test/ui/features/previously_on_sheet_test.dart`.

**Build.**

- [ ] Implement `RecapService.Generate(ctx, bookID, chapterIndex, paragraphOffset)` in `apps/server/internal/ai/recap.go` reading `recap_cache` first, backfilling missing `chapters.summary` rows only for `chapter_index <= target`, and invoking `ai.ChatClient.Chat` with explicit anti-spoiler prompt guardrails.
- [ ] Add `POST /api/v1/books/{id}/recap` in `apps/server/internal/api/recap.go` and wire it in `apps/server/internal/api/router.go`, returning cached or freshly generated recap text, bounded chapter range, and cache hit status.
- [ ] Add `BookRecap` model, `ApiService.getBookRecap`, `BookRepository.getBookRecap`, and `bookRecapProvider` in `apps/app/lib/`.
- [ ] Build `PreviouslyOnSheet` in `apps/app/lib/ui/features/recap/previously_on_sheet.dart` showing the active chapter ceiling, narrative recap, unresolved threads, and a regenerate option.
- [ ] Wire a **Previously On** button into `BookDetailView` hero actions and into the `endDrawer` of both `ReaderView` and `FixedLayoutReader`.

**You see.**

- [ ] Clicking **Previously On** on `BookDetailView` or inside the reader drawer opens `PreviouslyOnSheet` displaying a spoiler-bounded recap up to the reader's current chapter.
- [ ] Requesting a recap at the same `(book_id, chapter_index)` a second time returns `cached` equal to `true` without calling the LLM again.
- [ ] Chapters after `chapter_index` never appear in the prompt passed to `ai.ChatClient.Chat`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `apps/server/internal/ai/recap_test.go` verifies that a spy `ChatClient` receives zero text from chapters after `chapter_index` and that the second call hits `recap_cache`. Run `cd apps/server && go test -tags sqlite_fts5 -ldflags="-linkmode=external" -v ./internal/ai/... ./internal/api/...`.
- [ ] `apps/app/test/ui/features/previously_on_sheet_test.dart` verifies loading, rendered recap, chapter ceiling badge, and error retry state. Run `cd apps/app && flutter test test/ui/features/previously_on_sheet_test.dart test/ui/features/book_detail_view_test.dart test/ui/features/reader_view_test.dart`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` role at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Open Dune in `BookDetailView` and `ReaderView` at trunk and head, and confirm head adds the **Previously On** button without breaking reader navigation. Save `lane-01-reader-regression.png`. Pass when both trunk and head render Dune chapter 1 cleanly and head opens `PreviouslyOnSheet`.
- [ ] Lane 2. Trigger **Previously On** from `BookDetailView` on Dune and verify recap sheet opens with chapter ceiling badge. Save `lane-02-book-detail-recap.png`. Pass when ARIA snapshot shows recap text and zero console errors.
- [ ] Lane 3. Navigate to chapter 2 in `ReaderView`, open the TOC drawer, and click **Previously On**. Save `lane-03-reader-drawer-recap.png`. Pass when sheet displays ceiling "Up to Chapter 2" and renders recap text.
- [ ] Lane 4. Re-open **Previously On** at chapter 2 and verify cached response via `api POST /api/v1/books/{id}/recap`. Save `lane-04-recap-cache-hit.png`. Pass when API returns `"cached": true` and UI renders immediately.
- [ ] Lane 5. Advance to chapter 3 in `ReaderView` and request **Previously On** again. Save `lane-05-recap-new-chapter.png`. Pass when ceiling updates to Chapter 3 and cache stores a distinct entry.
- [ ] Lane 6. Click **Regenerate** in `PreviouslyOnSheet` with `force_refresh` enabled. Save `lane-06-recap-regenerate.png`. Pass when API regenerates content and updates UI without errors.
- [ ] Lane 7. Open fixed-layout book The Tale of Peter Rabbit in `FixedLayoutReader`, open the drawer, and click **Previously On**. Save `lane-07-fixed-layout-recap.png`. Pass when `browser iframes --assert-normalized` passes and recap sheet displays cleanly.
- [ ] Lane 8. Request recap at chapter index 0 (start of book) via UI or API and verify graceful empty or opening-context message. Save `lane-08-chapter-zero.png`. Pass when UI shows clear guidance without crashing.
- [ ] Lane 9. Verify `POST /api/v1/books/{id}/recap` rejects out-of-range negative `chapter_index` with HTTP 400. Save `lane-09-invalid-chapter.png`. Pass when API returns HTTP 400 JSON error.
- [ ] Lane 10. Test narrow mobile viewport (390 by 844) opening `PreviouslyOnSheet` from `ReaderView` drawer. Save `lane-10-mobile-recap.png`. Pass when sheet fits viewport with zero overflow errors in console.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. End-to-end latency in milliseconds of `POST /api/v1/books/{id}/recap` on a cache hit and `GET /api/v1/books/{id}` latency at trunk and head.
- [ ] Probe. Interleave 20 `GET /api/v1/books/{id}` calls at trunk and head, and measure 20 warm `POST /api/v1/books/{id}/recap` cache-hit calls on head against the local Ollama stub.
- [ ] Baseline. Record the trunk `GET /api/v1/books/{id}` median latency first before measuring head.
- [ ] Rule. Fail if `GET /api/v1/books/{id}` regresses by more than 10 percent over trunk, or if a warm recap cache hit exceeds 50 ms median latency.

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 2, lane 3, and lane 10 screenshots into `/tmp/shelfd-verify-evidence/review/pr-m1-previously-on-recap-review-ui.png`.
- [ ] Record a 30 to 60 second video of the change on a live lane. Save it as `/tmp/shelfd-verify-evidence/review/pr-m1-previously-on-recap-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for the operator's click.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] After the verdict, an owner merge uses Shipping step 4 to move the PR from its recorded patch base onto current trunk. An appended stack child keeps its recorded parent tip until that parent lands. Record the current landing SHA. Preserve the verdict only when the patch ID stays unchanged.
- [ ] Root appends `pr-m1-previously-on-recap` on top of `pr-m1-scoped-retrieval-cache` (targeting parent branch on same-repo, or targeting `main` with local parent ancestry on a fork) and the operator lands it in order.

## Build cross-book synoptic synthesis service and API (pr-m1-synoptic-reader-api)

**Depends on.** `pr-m1-scoped-retrieval-cache`.

**Files.**

- [ ] Create `apps/server/internal/ai/synoptic.go`.
- [ ] Create `apps/server/internal/ai/synoptic_test.go`.
- [ ] Create `apps/server/internal/api/synoptic.go`.
- [ ] Create `apps/server/internal/api/synoptic_test.go`.
- [ ] Edit `apps/server/internal/api/router.go`.

**Build.**

- [ ] Implement `SynopticService.Synthesize(ctx, SynopticRequest)` in `apps/server/internal/ai/synoptic.go` supporting `BookIDs`, `AuthorIDs`, and `GenreID` scope filters, hybrid vector plus FTS5 passage retrieval, and per-book passage capping so no single book monopolizes the prompt budget.
- [ ] Structure `SynopticResult` with `Summary`, `ConsensusPoints []SynopticPoint`, `DivergencePoints []SynopticPoint`, and clickable `Citations []BookCitation` carrying `BookID`, `BookTitle`, `AuthorName`, `ChapterIndex`, `StartParagraph`, and `EndParagraph`.
- [ ] Add authenticated `POST /api/v1/synoptic/query` in `apps/server/internal/api/synoptic.go` and wire it in `apps/server/internal/api/router.go`, enforcing non-empty query text and at least two matched books or an explicit scope filter.

**You see.**

- [ ] `POST /api/v1/synoptic/query` with two `book_ids` returns structured JSON containing `summary`, `consensus_points`, `divergence_points`, and `citations` referencing passages from both books.
- [ ] Per-book capping limits excerpts to at most `max_passages_per_book` (default 4) per book before invoking `ai.ChatClient.Chat`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `apps/server/internal/ai/synoptic_test.go` verifies fair per-book excerpt capping, citation mapping, and graceful fallback when one selected book has zero hits. Run `cd apps/server && go test -tags sqlite_fts5 -ldflags="-linkmode=external" -v ./internal/ai/... ./internal/api/...`.
- [ ] `apps/server/internal/api/synoptic_test.go` verifies request validation, auth enforcement, and JSON serialization. Run `cd apps/server && go vet -tags sqlite_fts5 ./...`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` role at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run `GET /api/v1/search?q=Gom+Jabbar` at trunk and head, and run `POST /api/v1/synoptic/query` across Dune and The Tale of Peter Rabbit on head. Save `lane-01-synoptic-regression.png`. Pass when trunk and head pass search and head returns structured synoptic JSON.
- [ ] Lane 2. Query `POST /api/v1/synoptic/query` with explicit `book_ids` for two books and verify citations include both `book_id` values. Save `lane-02-two-books.png`. Pass when response contains `consensus_points`, `divergence_points`, and citations with valid `chapter_index`.
- [ ] Lane 3. Upload `earthsea.epub` via `control-shelfd upload`, wait for indexing, and run a 3-book synoptic query. Save `lane-03-three-books.png`. Pass when response returns HTTP 200 and respects per-book passage caps.
- [ ] Lane 4. Query `POST /api/v1/synoptic/query` scoped by `genre_id` rather than `book_ids`. Save `lane-04-genre-scope.png`. Pass when response scopes retrieval only to books in that genre.
- [ ] Lane 5. Query `POST /api/v1/synoptic/query` scoped by `author_ids`. Save `lane-05-author-scope.png`. Pass when returned citations belong only to the requested authors.
- [ ] Lane 6. Send an empty `query` string to `POST /api/v1/synoptic/query`. Save `lane-06-empty-query.png`. Pass when server returns HTTP 400 with a JSON `error`.
- [ ] Lane 7. Send `POST /api/v1/synoptic/query` with unknown `book_ids` that match zero passages. Save `lane-07-zero-matches.png`. Pass when server returns HTTP 200 with empty citations and a clear message rather than HTTP 500.
- [ ] Lane 8. Send `POST /api/v1/synoptic/query` without auth token. Save `lane-08-auth-guard.png`. Pass when server returns HTTP 401.
- [ ] Lane 9. Verify existing single-book `POST /api/v1/books/{id}/chat` still scopes strictly to its single `book_id`. Save `lane-09-chat-isolation.png`. Pass when book chat returns only citations from the target book.
- [ ] Lane 10. Run `control-shelfd doctor` after all synoptic API calls to verify DB and sidecar health. Save `lane-10-doctor-clean.png`. Pass when `doctor` exits 0.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Retrieval and grouping latency in milliseconds for `SynopticService` (excluding external LLM generation) across 3 books, plus `GET /api/v1/search` median latency at trunk and head.
- [ ] Probe. Interleave 20 `GET /api/v1/search?q=Gom+Jabbar` requests at trunk and head, and run `go test -tags sqlite_fts5 -ldflags="-linkmode=external" -bench=BenchmarkSynopticRetrieval -benchtime=200ms ./internal/ai/...` on head.
- [ ] Baseline. Record the trunk `GET /api/v1/search` median latency first before measuring head.
- [ ] Rule. Fail if `GET /api/v1/search` regresses by more than 10 percent over trunk, or if 3-book synoptic retrieval and grouping exceeds 60 ms median latency.

**Review gate.** None. pr-m1-synoptic-reader-api is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] After the verdict, an owner merge uses Shipping step 4 to move the PR from its recorded patch base onto current trunk. An appended stack child keeps its recorded parent tip until that parent lands. Record the current landing SHA. Preserve the verdict only when the patch ID stays unchanged.
- [ ] Root appends `pr-m1-synoptic-reader-api` on top of `pr-m1-scoped-retrieval-cache` and the operator lands it before `pr-m1-synoptic-studio-ui`.

## Add Export Study Deck UI and file saver formats in Shelf (pr-m1-study-deck-export-ui)

**Depends on.** `pr-m1-mcp-study-decks`.

**Files.**

- [ ] Edit `apps/app/lib/ui/core/file_saver/file_saver_web.dart`.
- [ ] Edit `apps/app/lib/ui/core/file_saver/file_saver_io.dart`.
- [ ] Edit `apps/app/lib/data/services/api_service.dart`.
- [ ] Edit `apps/app/lib/data/repositories/book_repository.dart`.
- [ ] Create `apps/app/lib/ui/features/book_detail/study_deck_export_dialog.dart`.
- [ ] Edit `apps/app/lib/ui/features/book_detail/book_detail_view.dart`.
- [ ] Create `apps/app/test/ui/features/study_deck_export_dialog_test.dart`.

**Build.**

- [ ] Update `_inferMimeType` in `file_saver_web.dart` plus `_inferAllowedExtensions` and `_inferDialogTitle` in `file_saver_io.dart` to handle `.apkg`, `.csv`, and `.zip` study deck exports.
- [ ] Add `downloadStudyDeck(bookId, {format, chapterIndices})` to `ApiService` and `BookRepository` calling `POST /api/v1/books/$bookId/study-deck`.
- [ ] Build `StudyDeckExportDialog` in `apps/app/lib/ui/features/book_detail/study_deck_export_dialog.dart` letting users pick format (`Anki .apkg`, `Anki .csv`, `Markdown Vault .zip`) and select chapters.
- [ ] Add **Export Study Deck** to `_buildExportMenuItems` and `_handleExportMenuSelection` in `apps/app/lib/ui/features/book_detail/book_detail_view.dart`, triggering `saveDownloadedFile` and a floating `SnackBar` confirmation.

**You see.**

- [ ] Opening the export dropdown on `BookDetailView` lists **Export Study Deck** alongside EPUB and Markdown exports.
- [ ] Submitting `StudyDeckExportDialog` downloads a `.apkg`, `.csv`, or `.zip` file and displays a success `SnackBar`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `apps/app/test/ui/features/study_deck_export_dialog_test.dart` verifies format selection, chapter toggling, disabled submit when zero chapters are selected, and error snackbar handling. Run `cd apps/app && flutter test test/ui/features/study_deck_export_dialog_test.dart test/ui/features/book_detail_view_test.dart`.
- [ ] Run `cd apps/app && flutter analyze --no-fatal-infos --no-fatal-warnings`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` role at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Open Dune `BookDetailView` and trigger existing Markdown export at trunk and head, and confirm head adds **Export Study Deck**. Save `lane-01-export-menu-regression.png`. Pass when existing export still succeeds and new menu item appears.
- [ ] Lane 2. Open **Export Study Deck** dialog on Dune and verify format radio options and chapter list render. Save `lane-02-dialog-open.png`. Pass when ARIA snapshot shows all three formats and chapter checkboxes.
- [ ] Lane 3. Select `Anki .apkg` and chapter 1, click **Export**, and verify `POST /api/v1/books/{id}/study-deck` returns 200 and triggers browser download. Save `lane-03-export-apkg.png`. Pass when success `SnackBar` appears and console audit is clean.
- [ ] Lane 4. Select `Anki .csv` and chapter 1, click **Export**, and verify completion. Save `lane-04-export-csv.png`. Pass when success `SnackBar` appears and zero console errors occur.
- [ ] Lane 5. Select `Markdown Vault .zip` and chapters 1 and 2, click **Export**, and verify completion. Save `lane-05-export-markdown.png`. Pass when success `SnackBar` appears and zero console errors occur.
- [ ] Lane 6. Uncheck all chapters in `StudyDeckExportDialog` and confirm **Export** button is disabled. Save `lane-06-empty-selection-guard.png`. Pass when button reports disabled state in ARIA snapshot.
- [ ] Lane 7. Cancel `StudyDeckExportDialog` and verify `BookDetailView` remains interactive with no stray download. Save `lane-07-dialog-cancel.png`. Pass when dialog dismisses cleanly and no export request fires.
- [ ] Lane 8. Verify loading indicator disables duplicate clicks while an export request is in flight. Save `lane-08-inflight-guard.png`. Pass when only one network request is issued per export click.
- [ ] Lane 9. Verify `StudyDeckExportDialog` on a mobile viewport (390 by 844). Save `lane-09-mobile-dialog.png`. Pass when dialog scrolls cleanly with zero render overflow errors.
- [ ] Lane 10. Open The Tale of Peter Rabbit `BookDetailView` and verify **Export Study Deck** works on a second book. Save `lane-10-second-book-export.png`. Pass when export completes and console audit is clean.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. `BookDetailView` initial render time in milliseconds and Flutter Web bundle size delta (`apps/app/build/web/main.dart.js`) at trunk and head.
- [ ] Probe. Build `pnpm --filter @shelfd/app build:web` at trunk and head, record `main.dart.js` byte size, and measure `BookDetailView` frame time across 10 navigations interleaved on trunk and head via `control-shelfd browser`.
- [ ] Baseline. Record the trunk `main.dart.js` size and median `BookDetailView` load latency first before measuring head.
- [ ] Rule. Fail if `main.dart.js` grows by more than 150 KB or if `BookDetailView` load latency regresses by more than 15 percent over trunk.

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 2, lane 3, and lane 9 screenshots into `/tmp/shelfd-verify-evidence/review/pr-m1-study-deck-export-ui-review-ui.png`.
- [ ] Record a 30 to 60 second video of the change on a live lane. Save it as `/tmp/shelfd-verify-evidence/review/pr-m1-study-deck-export-ui-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for the operator's click.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] After the verdict, an owner merge uses Shipping step 4 to move the PR from its recorded patch base onto current trunk. An appended stack child keeps its recorded parent tip until that parent lands. Record the current landing SHA. Preserve the verdict only when the patch ID stays unchanged.
- [ ] Root appends `pr-m1-study-deck-export-ui` on top of `pr-m1-mcp-study-decks` and the operator lands it in order.

## Add Synoptic Studio comparative search UI in Shelf (pr-m1-synoptic-studio-ui)

**Depends on.** `pr-m1-synoptic-reader-api`.

**Files.**

- [ ] Create `apps/app/lib/data/models/synoptic.dart`.
- [ ] Edit `apps/app/lib/data/services/api_service.dart`.
- [ ] Edit `apps/app/lib/ui/state/providers.dart`.
- [ ] Create `apps/app/lib/ui/features/search/synoptic_studio_view.dart`.
- [ ] Edit `apps/app/lib/ui/features/search/search_view.dart`.
- [ ] Create `apps/app/test/ui/features/synoptic_studio_view_test.dart`.

**Build.**

- [ ] Add `SynopticResult`, `SynopticPoint`, and `SynopticCitation` models in `apps/app/lib/data/models/synoptic.dart` plus `ApiService.runSynopticQuery` in `apps/app/lib/data/services/api_service.dart`.
- [ ] Add `synopticProvider` (`SynopticNotifier` and `SynopticState`) in `apps/app/lib/ui/state/providers.dart` holding selected `bookIds`, optional `genreId`, query text, loading state, and `SynopticResult`.
- [ ] Build `SynopticStudioView` in `apps/app/lib/ui/features/search/synoptic_studio_view.dart` with a multi-book selector, genre filter pill row, prompt input, consensus and divergence cards, and clickable citation chips routing to `/books/${c.bookId}/${c.chapterIndex}`.
- [ ] Add a segmented toggle (`Search` and `Synoptic Studio`) at the top of `SearchView` in `apps/app/lib/ui/features/search/search_view.dart` so readers switch modes without adding a sixth bottom-nav tab.

**You see.**

- [ ] Opening `/search` shows the **Search** and **Synoptic Studio** mode toggle while preserving standard semantic search as the default view.
- [ ] Selecting two books in **Synoptic Studio** and submitting a thematic prompt renders consensus points, divergence points, and clickable book citations that jump straight to the cited chapter in `ReaderView`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `apps/app/test/ui/features/synoptic_studio_view_test.dart` verifies book selection state, consensus and divergence card rendering, citation tap navigation, and empty-result handling. Run `cd apps/app && flutter test test/ui/features/synoptic_studio_view_test.dart test/ui/features/search_view_test.dart`.
- [ ] Run `cd apps/app && flutter analyze --no-fatal-infos --no-fatal-warnings`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` role at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run standard `/search` query "Gom Jabbar" at trunk and head, and confirm head preserves search hits and adds the **Synoptic Studio** toggle. Save `lane-01-search-regression.png`. Pass when both trunk and head return Dune search cards and head renders the mode toggle.
- [ ] Lane 2. Switch to **Synoptic Studio**, select Dune and The Tale of Peter Rabbit, and submit a comparative prompt. Save `lane-02-synoptic-query.png`. Pass when consensus and divergence cards render with zero console errors.
- [ ] Lane 3. Click a Dune citation chip in the synoptic result and verify navigation to `/books/:id/:chapter`. Save `lane-03-citation-jump.png`. Pass when `ReaderView` opens at the cited chapter.
- [ ] Lane 4. Navigate back to `/search` and verify **Synoptic Studio** retains its prior selection and result in Riverpod state. Save `lane-04-state-preserved.png`. Pass when previous result remains visible without re-querying.
- [ ] Lane 5. Filter by a genre pill instead of individual book checkboxes and submit a synoptic query. Save `lane-05-genre-filter.png`. Pass when request sends `genre_id` and renders results cleanly.
- [ ] Lane 6. Submit a query with fewer than two books selected and no genre filter, and verify inline validation feedback. Save `lane-06-validation-guard.png`. Pass when UI displays clear guidance and does not crash.
- [ ] Lane 7. Toggle back to standard **Search** mode and run a fresh keyword search. Save `lane-07-toggle-back-search.png`. Pass when standard search hits render and synoptic state stays isolated.
- [ ] Lane 8. Clear selected books and verify selection chips reset to zero. Save `lane-08-clear-selection.png`. Pass when ARIA snapshot shows zero selected books.
- [ ] Lane 9. Test **Synoptic Studio** on a narrow mobile viewport (390 by 844). Save `lane-09-mobile-synoptic.png`. Pass when book picker and result cards render with zero overflow errors.
- [ ] Lane 10. Upload `earthsea.epub` via `control-shelfd upload`, refresh book picker, select 3 books, and run a 3-book synoptic query. Save `lane-10-three-book-ui.png`. Pass when all three books appear in the picker and synthesis renders cleanly.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. `/search` initial render latency in milliseconds and `main.dart.js` size delta at trunk and head.
- [ ] Probe. Build `pnpm --filter @shelfd/app build:web` at trunk and head, record `main.dart.js` byte size, and time 10 navigations to `/search` interleaved across trunk and head via `control-shelfd browser`.
- [ ] Baseline. Record the trunk `main.dart.js` size and median `/search` render latency first before measuring head.
- [ ] Rule. Fail if `main.dart.js` grows by more than 150 KB or if `/search` render latency regresses by more than 15 percent over trunk.

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 2, lane 3, and lane 9 screenshots into `/tmp/shelfd-verify-evidence/review/pr-m1-synoptic-studio-ui-review-ui.png`.
- [ ] Record a 30 to 60 second video of the change on a live lane. Save it as `/tmp/shelfd-verify-evidence/review/pr-m1-synoptic-studio-ui-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for the operator's click.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] After the verdict, an owner merge uses Shipping step 4 to move the PR from its recorded patch base onto current trunk. An appended stack child keeps its recorded parent tip until that parent lands. Record the current landing SHA. Preserve the verdict only when the patch ID stays unchanged.
- [ ] Root appends `pr-m1-synoptic-studio-ui` on top of `pr-m1-synoptic-reader-api` and the operator lands it last to complete Milestone 1.

## Close the program

- [ ] Every box above is checked with its evidence.
- [ ] Reply to the operator with the report the execution playbook names.

## Appendix A. Prototype evidence

No throwaway prototypes were run before writing this plan. Codebase exploration established the following facts directly from source files.

- Dual-dialect SQL migrations live in `apps/server/internal/database/migrations/` (latest `0020_drop_yoto_accounts.sql`) and `apps/server/internal/database/migrations_pg/` (latest `0016_drop_yoto_accounts.sql`), and `StorageEngine` has both `BunStorageEngine` (`bun.go`) and `SQLiteStorageEngine` (`sqlite.go`).
- `apps/server/internal/worker/worker.go` embeds paragraphs into `vec_paragraphs` but does not populate `chapters.summary`, so `RecapService` must backfill missing chapter summaries on demand rather than assuming pre-populated summaries.
- `apps/server/go.mod` already includes `github.com/mattn/go-sqlite3`, so `.apkg` archive generation needs no third-party Anki library.
- `apps/app/lib/ui/core/file_saver/file_saver_io.dart` and `file_saver_web.dart` currently hardcode EPUB, Markdown, and text MIME/extension mappings and need `.apkg`, `.csv`, and `.zip` entries.

Open item that remains unproven until `pr-m1-mcp-study-decks` runs live. Real Anki desktop import compatibility of the generated `collection.anki2` schema should be verified by opening a generated `.apkg` in Anki or against an Anki schema validation fixture during `pr-m1-mcp-study-decks`.

## Appendix B. Alternatives rejected

- **Adding a dedicated ebook reading-progress table in Milestone 1.** Rejected for now. `ReaderRepository` already tracks local reading position and `BookDetailView` plus `ReaderView` know the active `chapter_index`. Passing `chapter_index` and optional `paragraph_offset` directly to `POST /api/v1/books/{id}/recap` delivers issue 17 without coupling it to cross-device reading sync.
- **Reusing `ai.ChatAgent.Execute` for cross-book synoptic synthesis.** Rejected. `ChatAgent.Execute` hard-locks `args["book_id"] = bookID` on every tool call in `apps/server/internal/ai/chat_agent.go`. A dedicated `SynopticService` keeps single-book chat isolation intact and guarantees fair per-book excerpt capping.
- **Adding a sixth top-level bottom navigation tab for Synoptic Studio.** Rejected. `ShelfdBottomNav` already holds 5 tabs (`Library`, `Search`, `Uploads`, `Utilities`, `Settings`). Placing **Synoptic Studio** as a mode toggle inside `/search` keeps mobile navigation clean and avoids layout crowding.
- **Adding an external Go dependency for Anki `.apkg` generation.** Rejected. An `.apkg` file is a standard ZIP archive containing a small SQLite database (`collection.anki2`) and a `media` JSON file. Building it with `github.com/mattn/go-sqlite3` and stdlib `archive/zip` avoids supply-chain bloat.

## Appendix C. Risks

- **SQLite `sqlite-vec` KNN post-filtering selectivity (`pr-m1-scoped-retrieval-cache`, `pr-m1-synoptic-reader-api`).** `vec_paragraphs` applies `WHERE v.embedding MATCH ? AND k = ?` before SQLite filters by `book_id` or `chapter_index`. If a library grows large, `k = 100` may return too few rows for a narrow `BookIDs` or `MaxChapterIndex` filter. Owners must test selective multi-book filters on multi-book fixtures and increase `k` or fan out per book when needed.
- **Latency of on-demand chapter summarization (`pr-m1-previously-on-recap`).** Because `chapters.summary` is empty on fresh books, the first recap for a reader at Chapter 20 could trigger up to 20 summary LLM calls. Owners must cap on-demand summarization per request or summarize in bounded batches and persist each completed summary into `chapters.summary` immediately so retries make forward progress.
- **Anki `.apkg` schema compatibility (`pr-m1-mcp-study-decks`).** Anki expects exact column names and JSON blobs in `col.models` and `col.decks` inside `collection.anki2`. Owners must include a schema contract test asserting every required table, column, and JSON key in `study_test.go`.
- **LLM structured JSON drift (`pr-m1-synoptic-reader-api`, `pr-m1-mcp-study-decks`).** Local Ollama models occasionally emit Markdown code fences around JSON responses. Owners must strip fences and fall back cleanly to structured error handling when parsing fails.

## Appendix D. Links and reading list

Read these files before editing code in this program.

- `docs/PRODUCT_BRIEF.md` and `docs/TECHNICAL_DESIGN_DOCUMENT.md` for architecture invariants including Audiobookshelf sidecar sanctity.
- `.agents/skills/verify-shelfd/SKILL.md` and `.agents/skills/verify-shelfd/features/README.md` for the live verification harness and evidence conventions.
- `apps/server/internal/repository/storage.go`, `bun.go`, and `sqlite.go` before editing `pr-m1-scoped-retrieval-cache`.
- `apps/server/internal/ai/client.go` and `chat_agent.go` before editing `pr-m1-previously-on-recap` and `pr-m1-synoptic-reader-api`.
- `apps/server/internal/mcp/protocol.go` and `tools.go` before editing `pr-m1-mcp-study-decks`.
- `apps/app/lib/ui/features/book_detail/book_detail_view.dart`, `reader_view.dart`, and `search_view.dart` before editing the three Flutter UI PRs.
- Run `.agents/plugins/pstack/skills/how/SKILL.md` before `pr-m1-scoped-retrieval-cache` and `pr-m1-synoptic-reader-api` to verify SQLite KNN post-filter behavior.
- Run `.agents/plugins/pstack/skills/interrogate/SKILL.md` on `pr-m1-scoped-retrieval-cache` (anti-spoiler ceiling) and `pr-m1-mcp-study-decks` (`.apkg` schema) before marking them stack-ready.
- Keep each owner's `decisions.tsv` trail per `.agents/plugins/pstack/skills/show-me-your-work/SKILL.md` uncommitted and return it in the owner report.
