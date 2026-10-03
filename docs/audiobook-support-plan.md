# Audiobook Support plan

This plan adds complete audiobook playback support to shelfd for listeners and maintainers.
Audiobook files in M4B and MP3 formats stream cleanly with chapter metadata and position sync.
Audiobooks bypass vector embedding indexing entirely to keep AI compute light.
The plan delivers four sequential pull requests from database schema to Flutter UI.
The pull requests in order are pr-audiobook-schema-scanner, pr-audiobook-streaming-api, pr-audiobook-client-player-core, and pr-audiobook-client-ui-stitch.

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
- [ ] On the operator's go, arm a `/goal` with this exact text. "docs/audiobook-support-plan.md, pr-audiobook-schema-scanner, pr-audiobook-streaming-api, pr-audiobook-client-player-core, pr-audiobook-client-ui-stitch, verification rule, operator merges, all boxes checked."
- [ ] Read these from trunk at program start. Re-read them at every tick.
  - [ ] `git show origin/main:pstack/skills/poteto-mode/playbooks/autopilot-stack.md`
  - [ ] `git show origin/main:pstack/skills/swarm/SKILL.md`
  - [ ] `git show origin/main:pstack/skills/control-ui/SKILL.md`
  - [ ] `git show origin/main:pstack/skills/poteto-mode/playbooks/opening-a-pr.md`
  - [ ] `git show origin/main:pstack/skills/principle-model-the-domain/SKILL.md`
- [ ] Arm the 30-minute audit tick. In a local session, a real terminal `/loop`. In a cloud root, a cloud-sleeper wake chain. Never leave the cadence to memory.
- [ ] Use this tick prompt, verbatim. "Re-read the execution playbook from trunk and the armed /goal. Audit the operation against both and fix drift in this tick. Probe every active lane and judge progress by side effects only. Stand down a stuck lane and dispatch its replacement now. Then post a short status message to the operator in chat only when the audit found a tracked change that no earlier status message reported, such as a PR opened, a code-ready head, a round launched or closed, a verdict, a merge, a stuck agent and the action taken, a blocker added or cleared, or a decision only the operator can make. Name every such change and nothing else. Do not repeat a table, the merged list, or an unchanged blocker. If the audit found none, end the turn with no reply text. Either way, log this tick's row in your decision trail. The row names the items reported, or none."
- [ ] On the operator's hold or stand-down, send every owner a zero-writes order at once.

### Spawn owners

- [ ] Spawn one owner per PR with the full lifecycle the execution playbook names.
- [ ] Follow this dependency graph. Start dependent work only after its parent merges, or base it on the parent branch when the execution playbook stacks.
  - [ ] pr-audiobook-schema-scanner is first and branches from `main`.
  - [ ] pr-audiobook-streaming-api after pr-audiobook-schema-scanner.
  - [ ] pr-audiobook-client-player-core after pr-audiobook-streaming-api.
  - [ ] pr-audiobook-client-ui-stitch after pr-audiobook-client-player-core.
- [ ] Hold the file boundaries. Server PRs touch only `apps/server/**`. Client PRs touch only `apps/app/**`.
- [ ] Hold the review gate. pr-audiobook-client-ui-stitch changes the primary interaction. It waits for the operator's review in chat with screenshots and a video before merge.

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
- [ ] Start the backend server with `go run ./cmd/server` and Flutter app with `flutter run -d chrome --web-port 8080`.
- [ ] Deliver input only through the control skill's commands. Run read-only diagnostics via curl or inspect scripts.
- [ ] Save every screenshot to `/tmp/swarm-<pr-id>/worker-<n>/<slug>.png` and return the paths with the report.

## Add media format schema and audio metadata scanner (pr-audiobook-schema-scanner)

**Depends on.** None.

**Files.**

- [ ] Edit `apps/server/internal/repository/models.go`.
- [ ] Edit `apps/server/internal/scanner/scanner.go`.
- [ ] Edit `apps/server/internal/scanner/ingester.go`.
- [ ] Create `apps/server/internal/audio/metadata.go`.
- [ ] Create `apps/server/internal/audio/metadata_test.go`.
- [ ] Create `apps/server/internal/database/migrations/0013_audiobook_support.sql`.
- [ ] Create `apps/server/internal/database/migrations_pg/0004_audiobook_support.sql`.

**Build.**

- [ ] Add `book_type` and `duration_seconds` columns to the `books` table schema.
- [ ] Create `book_files` table for associated multi-format files such as EPUB, PDF, M4A, and cover image.
- [ ] Create `audio_chapters` table for chapter title, start offset, and duration.
- [ ] Update `Scanner.Scan` to discover `.m4b`, `.mp3`, `.m4a`, `.pdf`, and `.epub` files.
- [ ] Implement audio tag reader extracting duration, title, narrator, and chapters.
- [ ] Ensure background indexing skips vector embeddings for books of type audiobook.

**You see.**

- [ ] Log line confirms audio file ingestion. `scanner: ingested audiobook "The Architecture of Solitude" with 14 chapters`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Unit tests in `internal/audio/metadata_test.go` parse M4B atoms and MP3 ID3 tags. Run `go test ./internal/audio/... ./internal/scanner/...`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on `grok-4.7-xhigh-fast` at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run standard EPUB library scan on trunk and head. If trunk lacks audiobooks, record that and gate EPUB import integrity plus newly detected M4B files. Save `/tmp/swarm-pr-audiobook-schema-scanner/worker-1/scan-integrity.png`. Pass when both EPUBs and M4Bs populate catalog.
- [ ] Lane 2. Scan single M4B file with embedded Nero chapters. Save `/tmp/swarm-pr-audiobook-schema-scanner/worker-2/m4b-scan.png`. Pass when chapters and total duration match audio file headers.
- [ ] Lane 3. Scan single MP3 file with ID3v2 tags. Save `/tmp/swarm-pr-audiobook-schema-scanner/worker-3/mp3-scan.png`. Pass when author, narrator, and title persist correctly.
- [ ] Lane 4. Scan multi-track directory of audio files. Save `/tmp/swarm-pr-audiobook-schema-scanner/worker-4/multi-track-scan.png`. Pass when tracks sort in sequence.
- [ ] Lane 5. Scan audio file with embedded cover art. Save `/tmp/swarm-pr-audiobook-schema-scanner/worker-5/cover-art.png`. Pass when cover art extracts to storage.
- [ ] Lane 6. Ingest audiobook without triggering vector embedding jobs. Save `/tmp/swarm-pr-audiobook-schema-scanner/worker-6/skip-embeddings.png`. Pass when vector table has zero new rows.
- [ ] Lane 7. Re-scan unchanged audiobook library. Save `/tmp/swarm-pr-audiobook-schema-scanner/worker-7/rescan-idempotent.png`. Pass when no duplicate records or updated timestamps occur.
- [ ] Lane 8. Ingest audiobook with missing tags. Save `/tmp/swarm-pr-audiobook-schema-scanner/worker-8/fallback-metadata.png`. Pass when filename becomes title without crashing.
- [ ] Lane 9. Verify PostgreSQL and SQLite migration compatibility. Save `/tmp/swarm-pr-audiobook-schema-scanner/worker-9/db-migration.png`. Pass when schema migration runs clean on both engines.
- [ ] Lane 10. Query catalog API for mixed book types. Save `/tmp/swarm-pr-audiobook-schema-scanner/worker-10/mixed-catalog.png`. Pass when API returns correct book_type for ebooks and audiobooks.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Library scan time for 100 audio files in milliseconds.
- [ ] Probe. Run `go test -bench=BenchmarkScanAudiobooks ./internal/scanner`.
- [ ] Baseline. Record trunk baseline of 0 ms for unsupported audio files.
- [ ] Rule. Head scan time must process 100 audio files in under 1500 ms without reading full audio payload.

**Review gate.** None. pr-audiobook-schema-scanner is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Rebased onto current trunk after the verdict, patch-id unchanged.
- [ ] Append to base-branch stack.

## Implement audio streaming and progress sync APIs (pr-audiobook-streaming-api)

**Depends on.** pr-audiobook-schema-scanner.

**Files.**

- [ ] Edit `apps/server/internal/api/router.go`.
- [ ] Create `apps/server/internal/api/audiobooks.go`.
- [ ] Create `apps/server/internal/api/audiobooks_test.go`.
- [ ] Edit `apps/server/internal/repository/books.go`.

**Build.**

- [ ] Add HTTP byte-range audio streaming handler supporting `Accept-Ranges`, `Content-Range`, and 206 Partial Content.
- [ ] Update book detail endpoint `GET /api/v1/books/{id}` to return associated `files` list.
- [ ] Add chapter list endpoint `/api/v1/audiobooks/{id}/chapters`.
- [ ] Add playback progress sync endpoint `POST /api/v1/audiobooks/{id}/progress` with position and speed payload.
- [ ] Add playback state retrieval endpoint `GET /api/v1/audiobooks/{id}/progress`.

**You see.**

- [ ] HTTP response returns status 206 Partial Content with audio bytes when sending `Range: bytes=0-1048575`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Tests in `internal/api/audiobooks_test.go` verify 200, 206, and 416 range responses and progress persistence. Run `go test ./internal/api/...`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on `grok-4.7-xhigh-fast` at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run standard book download endpoint on trunk and head. If trunk lacks audio streaming, record that and gate full download plus byte range audio stream. Save `/tmp/swarm-pr-audiobook-streaming-api/worker-1/streaming-regression.png`. Pass when partial content returns valid audio headers.
- [ ] Lane 2. Request first 1MB audio segment via HTTP range header. Save `/tmp/swarm-pr-audiobook-streaming-api/worker-2/range-start.png`. Pass when status is 206 and length is 1048576 bytes.
- [ ] Lane 3. Request middle audio chunk simulating seek. Save `/tmp/swarm-pr-audiobook-streaming-api/worker-3/range-middle.png`. Pass when server streams requested byte offset.
- [ ] Lane 4. Request invalid range past EOF. Save `/tmp/swarm-pr-audiobook-streaming-api/worker-4/range-invalid.png`. Pass when server responds with 416 Range Not Satisfiable.
- [ ] Lane 5. Fetch chapter markers for multi-chapter M4B. Save `/tmp/swarm-pr-audiobook-streaming-api/worker-5/chapters-json.png`. Pass when JSON contains titles and timestamp offsets.
- [ ] Lane 6. Post playback progress update with position 1245.5 seconds. Save `/tmp/swarm-pr-audiobook-streaming-api/worker-6/progress-post.png`. Pass when response returns 200 and updated timestamp.
- [ ] Lane 7. Fetch progress for authenticated user. Save `/tmp/swarm-pr-audiobook-streaming-api/worker-7/progress-get.png`. Pass when returned position equals saved position.
- [ ] Lane 8. Stream audio under expired token. Save `/tmp/swarm-pr-audiobook-streaming-api/worker-8/auth-fail.png`. Pass when server returns 401 Unauthorized.
- [ ] Lane 9. Concurrent range requests from multiple clients. Save `/tmp/swarm-pr-audiobook-streaming-api/worker-9/concurrent-streams.png`. Pass when all streams complete without file lock contention.
- [ ] Lane 10. Stream audio with speed header and HEAD request. Save `/tmp/swarm-pr-audiobook-streaming-api/worker-10/head-support.png`. Pass when HEAD returns Content-Length and Accept-Ranges.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Time to first byte for 206 range request in milliseconds.
- [ ] Probe. Run `curl -s -w "%{time_starttransfer}\\n" -o /dev/null -H "Range: bytes=1000000-2000000" http://localhost:8080/api/v1/audiobooks/test-id/stream`.
- [ ] Baseline. Measure trunk static asset TTFB baseline of 12 ms.
- [ ] Rule. Head audio streaming TTFB must remain under 30 ms on local disk.

**Review gate.** None. pr-audiobook-streaming-api is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Rebased onto current trunk after the verdict, patch-id unchanged.
- [ ] Append to base-branch stack.

## Add client audio playback service and state management (pr-audiobook-client-player-core)

**Depends on.** pr-audiobook-streaming-api.

**Files.**

- [ ] Edit `apps/app/pubspec.yaml`.
- [ ] Create `apps/app/lib/data/models/audiobook.dart`.
- [ ] Create `apps/app/lib/data/repositories/audio_repository.dart`.
- [ ] Create `apps/app/lib/ui/state/audio_player_state.dart`.
- [ ] Create `apps/app/lib/ui/state/audio_player_provider.dart`.
- [ ] Create `apps/app/test/audio_player_provider_test.dart`.

**Build.**

- [ ] Add `just_audio` and `audio_session` dependencies to `apps/app/pubspec.yaml`.
- [ ] Implement `AudioRepository` to communicate with streaming and progress APIs.
- [ ] Implement `AudioPlayerNotifier` to manage playback, speed (0.75x to 2.0x), chapter skipping, and sleep timer.
- [ ] Add background playback session handling and audio interruption recovery.
- [ ] Add debounced position sync to persist listening progress to server every 10 seconds.

**You see.**

- [ ] State notifier emits `AudioPlaybackState.playing` with stream position updates.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Unit tests in `test/audio_player_provider_test.dart` verify speed changes, seek bounds, chapter transitions, and sleep timer countdown. Run `flutter test test/audio_player_provider_test.dart`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on `grok-4.7-xhigh-fast` at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run ebook reading state provider on trunk and head. If trunk lacks audio player state, record that and gate ebook progress provider alongside new audio player provider. Save `/tmp/swarm-pr-audiobook-client-player-core/worker-1/state-regression.png`. Pass when both state models operate concurrently.
- [ ] Lane 2. Initialize audio player with remote stream URL. Save `/tmp/swarm-pr-audiobook-client-player-core/worker-2/init-player.png`. Pass when state transitions from loading to ready.
- [ ] Lane 3. Play and pause audio track. Save `/tmp/swarm-pr-audiobook-client-player-core/worker-3/play-pause.png`. Pass when audio state reflects play/pause toggle accurately.
- [ ] Lane 4. Seek forward by 30 seconds and backward by 15 seconds. Save `/tmp/swarm-pr-audiobook-client-player-core/worker-4/seek-controls.png`. Pass when target position matches current position plus delta.
- [ ] Lane 5. Change playback speed to 1.25x and 1.5x. Save `/tmp/swarm-pr-audiobook-client-player-core/worker-5/speed-change.png`. Pass when player rate property equals selected speed.
- [ ] Lane 6. Jump directly to Chapter 3 from chapter list. Save `/tmp/swarm-pr-audiobook-client-player-core/worker-6/chapter-jump.png`. Pass when playhead moves to chapter start offset.
- [ ] Lane 7. Set 15-minute sleep timer. Save `/tmp/swarm-pr-audiobook-client-player-core/worker-7/sleep-timer.png`. Pass when timer pauses playback upon reaching zero.
- [ ] Lane 8. Trigger sleep timer fade out. Save `/tmp/swarm-pr-audiobook-client-player-core/worker-8/sleep-fade.png`. Pass when volume decreases smoothly in final 30 seconds.
- [ ] Lane 9. Simulate app backgrounding and resume. Save `/tmp/swarm-pr-audiobook-client-player-core/worker-9/background-audio.png`. Pass when playback continues uninterrupted.
- [ ] Lane 10. Sync progress periodically to server. Save `/tmp/swarm-pr-audiobook-client-player-core/worker-10/sync-progress.png`. Pass when server receives debounced progress update.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Player state transition latency on play/pause in milliseconds.
- [ ] Probe. Run benchmark test asserting time between toggle call and emitted state.
- [ ] Baseline. Measure baseline UI state dispatch latency of 4 ms.
- [ ] Rule. Audio player state transitions must complete within 16 ms to avoid dropping UI frames.

**Review gate.** None. pr-audiobook-client-player-core is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Rebased onto current trunk after the verdict, patch-id unchanged.
- [ ] Append to base-branch stack.

## Build audiobook player UI and chapters drawer from Stitch designs (pr-audiobook-client-ui-stitch)

**Depends on.** pr-audiobook-client-player-core.

**Files.**

- [ ] Edit `apps/app/lib/ui/features/book_detail/book_detail_view.dart`.
- [ ] Create `apps/app/lib/ui/features/audiobook/audiobook_player_view.dart`.
- [ ] Create `apps/app/lib/ui/features/audiobook/widgets/scrubber_bar.dart`.
- [ ] Create `apps/app/lib/ui/features/audiobook/widgets/playback_controls.dart`.
- [ ] Create `apps/app/lib/ui/features/audiobook/widgets/speed_selector_dialog.dart`.
- [ ] Create `apps/app/lib/ui/features/audiobook/widgets/sleep_timer_sheet.dart`.
- [ ] Create `apps/app/lib/ui/features/audiobook/widgets/chapter_drawer.dart`.
- [ ] Edit `apps/app/lib/ui/features/library/widgets/book_card.dart`.
- [ ] Edit `apps/app/lib/ui/router.dart`.
- [ ] Create `apps/app/test/audiobook_player_view_test.dart`.

**Build.**

- [ ] Update `BookDetailView` following Stitch screen `0f10230802354ef3aed8ee2fa4141949` to render associated multi-format files list and dual Read and Listen action triggers.
- [ ] Implement full-screen and sheet `AudiobookPlayerView` following Stitch screen `ae0425cb5fa6496fb3c736a3c529d424`.
- [ ] Implement `ChapterDrawer` modal sheet with active equalizer animation and bookmark list per Stitch screen `f4528296b18a4971bf20f36304b127bc`.
- [ ] Implement tactile scrubber bar with elapsed time, chapter remaining time, and bookmark markers.
- [ ] Add audiobook badges and listening progress bars to library cards and book detail views.
- [ ] Register `/audiobook/:id` route in router and add mini-player persistent bar on bottom navigation.

**You see.**

- [ ] Screen renders dark obsidian theme with glowing amber controls, book cover art, tactile waveform scrubber, and chapter drawer.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Widget tests in `test/audiobook_player_view_test.dart` verify widget tree rendering, scrubber gestures, speed menu, and chapter selection. Run `flutter test test/audiobook_player_view_test.dart`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on `grok-4.7-xhigh-fast` at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run ebook reader view on trunk and head. If trunk lacks audiobook UI, record that and gate ebook reader rendering plus new audiobook player interface. Save `/tmp/swarm-pr-audiobook-client-ui-stitch/worker-1/ui-regression.png`. Pass when ebook reading and audiobook listening both launch without layout shifts.
- [ ] Lane 2. Open audiobook player from library card. Save `/tmp/swarm-pr-audiobook-client-ui-stitch/worker-2/player-open.png`. Pass when cover art, title, author, and narrator display clearly.
- [ ] Lane 3. Drag scrubber thumb along playback bar. Save `/tmp/swarm-pr-audiobook-client-ui-stitch/worker-3/scrubber-drag.png`. Pass when scrub timestamps update smoothly during gesture.
- [ ] Lane 4. Open chapter drawer sheet. Save `/tmp/swarm-pr-audiobook-client-ui-stitch/worker-4/chapter-drawer-open.png`. Pass when active chapter exhibits animated equalizer bars.
- [ ] Lane 5. Select different chapter from drawer. Save `/tmp/swarm-pr-audiobook-client-ui-stitch/worker-5/select-chapter.png`. Pass when player updates chapter title and playhead position.
- [ ] Lane 6. Open speed selector modal and pick 1.5x. Save `/tmp/swarm-pr-audiobook-client-ui-stitch/worker-6/speed-modal.png`. Pass when speed chip reflects 1.5x active state.
- [ ] Lane 7. Open sleep timer sheet and pick 30 minutes. Save `/tmp/swarm-pr-audiobook-client-ui-stitch/worker-7/sleep-sheet.png`. Pass when moon icon displays active timer badge.
- [ ] Lane 8. Minimize player to persistent bottom mini-player bar. Save `/tmp/swarm-pr-audiobook-client-ui-stitch/worker-8/mini-player.png`. Pass when mini-player sits above navigation dock without obstructing lists.
- [ ] Lane 9. Verify responsive layout on mobile and desktop viewports. Save `/tmp/swarm-pr-audiobook-client-ui-stitch/worker-9/responsive-layout.png`. Pass when layout expands cleanly on wider displays.
- [ ] Lane 10. Switch between dark and light app themes. Save `/tmp/swarm-pr-audiobook-client-ui-stitch/worker-10/theme-toggle.png`. Pass when colors preserve high contrast and obsidian amber styling.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Frame render time during scrubber drag in milliseconds per frame.
- [ ] Probe. Run Flutter driver performance profile during 5-second scrubber drag.
- [ ] Baseline. Measure ebook reader scroll frame time baseline of 8.2 ms.
- [ ] Rule. 99th percentile frame render time during scrubbing must stay under 16.6 ms (60 fps).

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 2, 4, and 8 screenshots into `docs/screenshots/pr-audiobook-client-ui-stitch-review-player.png`, `docs/screenshots/pr-audiobook-client-ui-stitch-review-drawer.png`, and `docs/screenshots/pr-audiobook-client-ui-stitch-review-mini.png`.
- [ ] Record a 30 to 60 second video of the change on a lane VM. Save it as `docs/screenshots/pr-audiobook-client-ui-stitch-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for the operator's click.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Rebased onto current trunk after the verdict, patch-id unchanged.
- [ ] Append to base-branch stack and let operator land stack.

## Close the program

- [ ] Every box above is checked with its evidence.
- [ ] Reply to the operator with the report the execution playbook names.

## Appendix A. Prototype evidence

Stitch prototype generation settled the UI architecture for the player interface and book details.
Project ID `15215746235566030932` contains the complete design system and screen prototypes.
Screen `0f10230802354ef3aed8ee2fa4141949` proved the book details layout with associated multi-format files list and dual Read and Listen actions.
Screen `ae0425cb5fa6496fb3c736a3c529d424` proved the single-screen mobile player layout with hero artwork, amber circular play button, tactile waveform scrubber, and secondary transport pill bar.
Screen `f4528296b18a4971bf20f36304b127bc` proved the modal bottom drawer for chapter browsing, active equalizer bars, audio bookmarks, and sleep timer quick actions.
Design system `Shelfd Reader` (`assets/b8f7f2561fcc4498a44fc4d9ca236224`) establishes the deep obsidian base (`#141315`), warm amber primary accents (`#E5A967`), and Newsreader plus Manrope typography tokens.

## Appendix B. Alternatives rejected

Embedding audiobooks into the vector pipeline was rejected. Text chunking and AI embedding extraction for audio transcripts would introduce heavy processing overhead without immediate playback necessity.
Bundling full audio transcoding inside the server was rejected. Direct byte-range HTTP streaming of M4B and MP3 formats allows native platform players to handle playback with zero server CPU transcoding overhead.
Single monolithic pull request was rejected. Splitting backend schema, streaming APIs, audio state management, and UI into four distinct PRs allows independent unit, live, and perf verification at each step.

## Appendix C. Risks

Large M4B files with extensive chapter tables can produce high memory allocations during scanning. The owner of pr-audiobook-schema-scanner must stream container atoms rather than buffering full audio files in memory.
Network latency during scrubbing could cause audio stutter. The owner of pr-audiobook-client-player-core must configure pre-buffering windows in `just_audio`.
Background audio session interruption by incoming calls or navigation apps must be handled cleanly. The owner of pr-audiobook-client-player-core must subscribe to audio session interruptions.

## Appendix D. Links and reading list

- Design System and Player Screens in Stitch Project `15215746235566030932`.
- Principle Model The Domain in `pstack/skills/principle-model-the-domain/SKILL.md`.
- Principle Boundary Discipline in `pstack/skills/principle-boundary-discipline/SKILL.md`.
- Principle Type System Discipline in `pstack/skills/principle-type-system-discipline/SKILL.md`.
- Principle Sequence Verifiable Units in `pstack/skills/principle-sequence-verifiable-units/SKILL.md`.
- Principle Prove It Works in `pstack/skills/principle-prove-it-works/SKILL.md`.
- Principle Experience First in `pstack/skills/principle-experience-first/SKILL.md`.
- Autopilot Stack Playbook in `pstack/skills/poteto-mode/playbooks/autopilot-stack.md`.
