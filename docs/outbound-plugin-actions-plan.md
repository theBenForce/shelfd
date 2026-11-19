# Outbound plugin actions and declarative input interfaces plan

Readers and homelab operators gain extensible book and series actions where plugins declare their own forms, media pickers, account linking, client handoffs, and background transfer jobs.
Core Shelfd drops bespoke Yoto and NotebookLM handlers, config blocks, database tables, and Flutter dialogs while keeping generic Markdown export in core.
Both `plugins/yoto` and `plugins/notebooklm` ship in-repo alongside `plugins/project-gutenberg` and run out of process over stdio JSON-RPC 2.0 and scoped loopback HTTP.
Every pull request compiles and passes workspace lint, unit tests, ten live browser and API lanes, and a dual-sided performance probe at its own head SHA.
The pull requests in order are `pr-plugin-actions-host`, `pr-builtin-yoto-notebooklm-plugins`, `pr-flutter-declarative-plugin-ui`, and `pr-delete-legacy-yoto-notebooklm-core`.

## How to read this

One box is one unit of work. Every box names the evidence that checks it. A nested box is a sub-step of the box above it. Check a box only when its evidence exists, a file, a log line, a screenshot, a test run, or a SHA. The body is a how-to. The appendices explain and record.

The program runs `/shared/user/.gemini/skills/poteto-mode/playbooks/autopilot-stack.md` under the installed plugin. Owners append verified pull requests to a bottom-up stack, `pr-flutter-declarative-plugin-ui` stops at merge-ready for operator UI review, and the operator lands the stack in order.

Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

## Program checklist

### Arm the program

- [ ] State the protocol and this plan to the operator, then stop. Start execution only on the operator's explicit go.
- [ ] On the operator's go, copy this approved plan to `docs/outbound-plugin-actions-plan.md` on the stack root branch and write the program objective into the standing orders and your todolist with this exact text. "`docs/outbound-plugin-actions-plan.md`, `pr-plugin-actions-host`, `pr-builtin-yoto-notebooklm-plugins`, `pr-flutter-declarative-plugin-ui`, `pr-delete-legacy-yoto-notebooklm-core`, verification rule, operator reviews `pr-flutter-declarative-plugin-ui` and merges stack in order, all boxes checked."
- [ ] Read these from the installed plugin at program start. Re-read them at every tick.
  - [ ] `/shared/user/.gemini/skills/poteto-mode/playbooks/autopilot-stack.md`
  - [ ] `/shared/user/.gemini/skills/swarm/SKILL.md`
  - [ ] `.agents/skills/verify-shelfd/SKILL.md`
  - [ ] `/shared/user/.gemini/skills/poteto-mode/playbooks/opening-a-pr.md`
  - [ ] `/shared/user/.gemini/skills/principle-redesign-from-first-principles/SKILL.md`
  - [ ] `/shared/user/.gemini/skills/principle-model-the-domain/SKILL.md`
  - [ ] `/shared/user/.gemini/skills/principle-type-system-discipline/SKILL.md`
  - [ ] `/shared/user/.gemini/skills/principle-boundary-discipline/SKILL.md`
  - [ ] `/shared/user/.gemini/skills/principle-sequence-verifiable-units/SKILL.md`
  - [ ] `/shared/user/.gemini/skills/principle-migrate-callers-then-delete-legacy-apis/SKILL.md`
  - [ ] `/shared/user/.gemini/skills/principle-subtract-before-you-add/SKILL.md`
  - [ ] `/shared/user/.gemini/skills/principle-prove-it-works/SKILL.md`
- [ ] Arm the 30-minute audit tick as a real cadence per `/shared/user/.gemini/skills/poteto-mode/references/antigravity-tools.md`. Never leave the cadence to memory.
- [ ] Use this tick prompt, verbatim. "Re-read the execution playbook from the installed plugin and the standing orders. Audit the operation against both and fix drift in this tick. Probe every active lane and judge progress by side effects only. Stand down a lane only on affirmative failure evidence, and dispatch its replacement in the same tick. Then send the operator a status message, whether or not anything changed, with the queue table of PR, owner, state, and head SHA, the verdicts since the last tick, what merged, open operator gates, and blockers."
- [ ] On the operator's hold or stand-down, send every owner a zero-writes order at once.

### Spawn owners

- [ ] Spawn one owner per PR with the full lifecycle the execution playbook names.
- [ ] Follow this dependency graph. Start dependent work only after its parent merges, or rebase its branch onto the parent's exact tip when the execution playbook stacks. A same-repository child PR targets its parent branch. A fork child PR targets trunk while retaining local parent ancestry. Freeze the bottom-to-top order because fork PR bases do not encode it.
  - [ ] `pr-plugin-actions-host` is first and branches from `main`.
  - [ ] `pr-builtin-yoto-notebooklm-plugins` after `pr-plugin-actions-host`.
  - [ ] `pr-flutter-declarative-plugin-ui` after `pr-builtin-yoto-notebooklm-plugins`.
  - [ ] `pr-delete-legacy-yoto-notebooklm-core` after `pr-flutter-declarative-plugin-ui`.
- [ ] Hold the file boundaries. `pr-plugin-actions-host` touches only `apps/server/internal/{plugins,api}/**`, `packages/plugin-sdk/**`, `skills/create-shelfd-plugin/**`, and `docs/outbound-plugin-actions-plan.md`. `pr-builtin-yoto-notebooklm-plugins` touches only `plugins/{yoto,notebooklm}/**`, `Dockerfile`, `docker/**`, `turbo.json`, and `apps/server/internal/api/plugins_test.go`. `pr-flutter-declarative-plugin-ui` touches only `apps/app/**` plus the Dart widget-key contract assertions in `apps/server/internal/api/{ui_e2e_test.go,yoto_test.go}`. `pr-delete-legacy-yoto-notebooklm-core` touches only `apps/server/**`, `config.example.yaml`, `README.md`, and `.agents/skills/verify-shelfd/**`.
- [ ] Hold the review gate. `pr-flutter-declarative-plugin-ui` changes book, series, settings, and upload-banner user interactions. It waits for the operator's review in chat with screenshots and a video before merge.

### PR mechanics, for every PR

- [ ] Resolve the forge once. Default to `gh`; if `command -v origin` succeeds and Origin can resolve the repository, use `origin pr` for every PR operation. Record any fallback to `gh`. Record the intended PR base repository as canonical `<base-repo>` and validate it through the active forge. Do not infer it from the checkout's default remote. Capture it as a shell variable and pass `--repo "$base_repo"` to every `gh pr` command. Resolve and validate `<head-url>` through Shipping step 1 and capture it as `head_url` for the live-lane fetch. When the head repository is a fork, validate its identity and record its owner and repository name as `<fork-owner>` and `<head-name>`. Never require `gt`.
- [ ] Open the PR before self-proof and follow the readiness rule in `/shared/user/.gemini/skills/poteto-mode/playbooks/opening-a-pr.md`. Open it ready by default. When repository instructions require a draft until named evidence exists, keep it draft until that evidence is recorded. Use `origin pr create --status open --base "$base_branch"` or `gh pr create --base "$base_branch" --repo "$base_repo"` for a ready same-repository PR. A same-repository stack child targets its parent branch. Every fork PR targets trunk. With GitHub, capture the approved PR title and body as `<title>` and `<body>`, then create it with `gh api --method POST "repos/$base_repo/pulls" -f "title=$title" -f "body=$body" -f "head=$fork_owner:$branch" -f "head_repo=$head_name" -f "base=$trunk" --jq .html_url`; add `-F draft=true` when repository instructions require a draft. Otherwise use the resolved Origin command. Stacked fork branches retain local parent ancestry.
- [ ] Run `pnpm lint` and `pnpm test` once before the PR-facing push. Push with hooks on.
- [ ] Run `/deslop` before each commit and `/no-comments` before review.
- [ ] Triage every Bugbot and security-reviewer comment per `/shared/user/.gemini/skills/poteto-mode/references/bugbot-triage.md` under the installed plugin.
- [ ] Before babysit, rebase each independent PR and stack root onto current trunk. Rebase each unmerged stack child onto its parent's exact tip. After its parent merges, use Shipping's explicit old-base-to-trunk rebase before the child's merge-ready report.

### Verdict and merge, for every PR

- [ ] At the merge-ready head SHA, run the swarm per `/shared/user/.gemini/skills/swarm/SKILL.md`. One gates lane. The ten live lanes from the PR's **Verify, live** block. The perf lane from its **Verify, perf** block. One audit lane that reads the diff and the receipts and distrusts the PR body.
- [ ] Clean only when every lane is `PASS`. Findings go back to the owner. A new head gets a fresh swarm and a fresh verdict.
- [ ] Append each verified PR to the frozen bottom-to-top stack per `/shared/user/.gemini/skills/poteto-mode/playbooks/shipping.md`, recording the verdict SHA, current landing SHA, recorded patch base, and `git patch-id --stable` value so the operator lands the stack in order.

### Boot recipe, for every live lane

Each live lane is one `swarm workers` lane at the PR head, resolved through provider dispatch, in its own worktree or output directory, with its own receipt. Drive the surface only through `.agents/skills/verify-shelfd/SKILL.md`.

- [ ] `git fetch -- "$head_url" "refs/heads/$head_branch" && git checkout --detach "$head_sha"` in the lane's worktree.
- [ ] Ensure `flutter` is on `PATH` (bootstrapping Flutter stable into `/tmp/flutter` if absent on an ephemeral VM), build in-repo plugins with `pnpm build`, and start an isolated stack with `SHELFD_VERIFY_RUN_ID="pr-<id>-w<n>" SHELFD_VERIFY_PORT=$((18080 + n)) SHELFD_VERIFY_AI_PORT=$((13000 + n)) SHELFD_VERIFY_CDP_PORT=$((19222 + n)) .agents/skills/verify-shelfd/scripts/control-shelfd launch --foreground` as a background daemon, then run `control-shelfd doctor` until healthy.
- [ ] Deliver browser input only through `.agents/skills/verify-shelfd/scripts/control-shelfd browser ...` and run read-only diagnostics through `control-shelfd api ...`, `control-shelfd mcp ...`, `control-shelfd browser console --assert-clean`, and SQLite or log inspection inside `$SHELFD_RUNTIME_DIR`.
- [ ] Save every screenshot to `/tmp/swarm-<pr-id>/worker-<n>/<slug>.png` and return the paths with the receipt path, then stop the lane stack with `control-shelfd cleanup`.

## Add declarative plugin actions, auth RPCs, and background jobs to host and SDK (pr-plugin-actions-host)

**Depends on.** None.

**Files.**

- [ ] Create `docs/outbound-plugin-actions-plan.md`.
- [ ] Edit `apps/server/internal/plugins/manifest.go`.
- [ ] Edit `apps/server/internal/plugins/types.go`.
- [ ] Create `apps/server/internal/plugins/jobs.go`.
- [ ] Edit `apps/server/internal/plugins/host.go`.
- [ ] Edit `apps/server/internal/plugins/plugins_test.go`.
- [ ] Edit `apps/server/internal/api/plugins.go`.
- [ ] Edit `apps/server/internal/api/books.go`.
- [ ] Edit `apps/server/internal/api/taxonomy.go`.
- [ ] Edit `apps/server/internal/api/middleware.go`.
- [ ] Edit `apps/server/internal/api/router.go`.
- [ ] Edit `apps/server/internal/api/plugins_test.go`.
- [ ] Edit `apps/server/internal/api/books_test.go`.
- [ ] Edit `packages/plugin-sdk/src/types.ts`.
- [ ] Edit `packages/plugin-sdk/src/index.ts`.
- [ ] Edit `packages/plugin-sdk/src/cli.ts`.
- [ ] Edit `packages/plugin-sdk/test/sdk.test.mjs`.
- [ ] Edit `packages/plugin-sdk/skills/create-shelfd-plugin/SKILL.md`.
- [ ] Edit `packages/plugin-sdk/skills/create-shelfd-plugin/references/protocol-and-sdk.md`.
- [ ] Edit `skills/create-shelfd-plugin/SKILL.md`.
- [ ] Edit `skills/create-shelfd-plugin/references/protocol-and-sdk.md`.

**Build.**

- [ ] Allowlist capability `"actions"` in `ValidateManifest` (`apps/server/internal/plugins/manifest.go`) and add `Auth *AuthSpec` plus `Actions []ActionDef` to `Manifest`, `PluginSummary`, and `PluginDetail` (`apps/server/internal/plugins/types.go`).
- [ ] Model `ActionDef` (`Name`, `Title`, `Description`, `Icon`, `Targets` of `"book"` or `"series"`, `MediaTypes` of `"ebook"` or `"audiobook"`, `Mode` of `"immediate"` or `"form"`, `Execution` of `"sync"` or `"job"`), `AuthSpec` (`Methods` of `"oauth_pkce"` or `"token"`, `Title`, `Description`, `CallbackParam`, `Fields`), `ActionFormSpec` (`Title`, `SubmitLabel`, `AuthRequired`, `AuthMessage`, `Entries`, `Fields`), discriminated `InputFieldSpec` (`Kind` of `"text"`, `"boolean"`, `"select"`, `"segmented"`, `"remote_select"`, or `"asset_picker"` with `VisibleWhen` predicates and `PerEntryOverride` metadata), `OptionItem`, and `ActionResult` (`Status` of `"completed"`, `"client_handoff"`, or `"job_started"`) in both `apps/server/internal/plugins/types.go` and `packages/plugin-sdk/src/types.ts`.
- [ ] Implement `PluginJobManager` in `apps/server/internal/plugins/jobs.go` tracking per-user `PluginJob` records (`queued`, `running`, `completed`, `failed`), holding an active lease on `managedPlugin` so `reapIdleProcesses` (`apps/server/internal/plugins/host.go`) never kills a non-daemon plugin while a job is running, resetting a 5-minute activity watchdog on each `progress` notification, and broadcasting user-scoped `events.EventPlugin` (`kind` set to `"job_progress"`) payloads to `events.Hub`.
- [ ] Add inbound JSON-RPC request handling for `"host/refresh_token"` in `(*Host).handlePluginInbound` (`apps/server/internal/plugins/host.go`) so long-running plugin jobs renew their 5-minute loopback JWT without broadening token lifetime.
- [ ] Add `GetAuthStatus`, `BeginAuth`, `CompleteAuth`, `DisconnectAuth`, `PrepareAction`, `FetchActionOptions`, `UploadActionAsset` (5 MB multipart limit, base64 or temp-file RPC bridge), `InvokeAction` (synchronous or background job), `ListUserJobs`, and `DismissJob` methods on `Host` (`apps/server/internal/plugins/host.go`) and expose them on `PluginHandler` (`apps/server/internal/api/plugins.go`) and `NewRouter` (`apps/server/internal/api/router.go`) under `/api/v1/plugins/{name}/auth/*`, `/api/v1/plugins/{name}/actions/{action}/{prepare,options,assets,invoke}`, and `/api/v1/plugins/jobs`.
- [ ] Add generic `ExportBookMarkdown` (`GET /api/v1/books/{id}/markdown`) refactored from `compileBookNotebookLMMarkdown` in `apps/server/internal/api/books.go` while keeping `ExportBookNotebookLM` delegating to it until PR 4.
- [ ] Add `GetSeries` (`GET /api/v1/series/{id}`) on `TaxonomyHandler` (`apps/server/internal/api/taxonomy.go`) returning the `Series` metadata and ordered books via `repo.GetSeriesByID` and `repo.ListBooks`.
- [ ] Expand `PluginScopeAllowsRequest` (`apps/server/internal/api/middleware.go`) so `books:read` permits `GET` and `HEAD` on `/api/v1/audiobooks/` and `/api/v1/series/` in addition to `/api/v1/books/`, blocking `POST`, `PUT`, and `DELETE` across all library endpoints.
- [ ] Extend `definePlugin` in `packages/plugin-sdk/src/index.ts` and `shelfd-plugin verify` in `packages/plugin-sdk/src/cli.ts` with typed handlers for `auth/{status,begin,complete,disconnect}` and `actions/{prepare,options,upload_asset,invoke}`, plus loopback helpers `ctx.fetchLoopback`, `ctx.openLoopbackStream` (returning a web `ReadableStream<Uint8Array>` and content length without buffering whole files in memory), `ctx.getBookMarkdown`, and `ctx.refreshInvocationToken` (automatically retrying on 401 expiry).
- [ ] Document `"actions"`, `AuthSpec`, `InputFieldSpec`, and streaming loopback helpers in both `packages/plugin-sdk/skills/create-shelfd-plugin/` and `skills/create-shelfd-plugin/`.

**You see.**

- [ ] `GET /api/v1/plugins` returns `actions` and `auth` metadata on plugins declaring `"actions"`, `GET /api/v1/books/{id}/markdown` returns compiled book Markdown with highlights and chapters, `GET /api/v1/series/{id}` returns series metadata with ordered books, and `GET /api/v1/plugins/jobs` lists active and completed background plugin jobs.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `apps/server/internal/plugins/plugins_test.go` covers `"actions"` manifest validation, `auth/*` and `actions/*` RPC dispatch, scale-to-zero lease retention during active background jobs, watchdog timeout on stalled jobs, and `"host/refresh_token"` renewal. Run `cd apps/server && go test -tags sqlite_fts5 -ldflags="-linkmode=external" -v ./internal/plugins`.
- [ ] `apps/server/internal/api/plugins_test.go` and `apps/server/internal/api/books_test.go` cover `GET /api/v1/books/{id}/markdown`, `GET /api/v1/series/{id}`, `books:read` scope enforcement on `/api/v1/audiobooks/{id}/chapters` and `/api/v1/series/{id}`, multipart asset upload limits, and SSE `job_progress` broadcasts. Run `cd apps/server && go test -tags sqlite_fts5 -ldflags="-linkmode=external" -v ./internal/api`.
- [ ] `packages/plugin-sdk/test/sdk.test.mjs` covers `definePlugin` action and auth handlers, `ctx.openLoopbackStream`, automatic `host/refresh_token` retry, and `shelfd-plugin verify` checks on an action plugin fixture. Run `pnpm --filter @shelfd/plugin-sdk run test`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` role at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Boot stack on trunk and head, open Settings, verify `project-gutenberg` discovery and catalog search in `/plugins/catalog`. Save `/tmp/swarm-pr-plugin-actions-host/worker-1/gutenberg-regression.png`. Pass when `project-gutenberg` renders `idle` or `running` and returns search results with zero console errors.
- [ ] Lane 2. Install a `.zip` fixture plugin declaring `"actions"` and `"auth"` via `control-shelfd api POST /api/v1/plugins/install` and refresh `/settings`. Save `/tmp/swarm-pr-plugin-actions-host/worker-2/install-action-plugin.png`. Pass when `GET /api/v1/plugins` lists `actions` and `auth` descriptors and the plugin card appears in Settings.
- [ ] Lane 3. Call `POST /api/v1/plugins/<fixture>/actions/demo/prepare` for seeded `Dune` book ID and inspect returned `ActionFormSpec`. Save `/tmp/swarm-pr-plugin-actions-host/worker-3/prepare-form-spec.png`. Pass when `fields`, `entries`, and `visible_when` serialize cleanly with HTTP 200.
- [ ] Lane 4. Query `GET /api/v1/plugins/<fixture>/actions/demo/options?source=targets&q=test` and upload a 16x16 PNG to `POST /api/v1/plugins/<fixture>/actions/demo/assets`. Save `/tmp/swarm-pr-plugin-actions-host/worker-4/options-and-asset-upload.png`. Pass when filtered `OptionItem` entries and uploaded asset URI return HTTP 200 and payloads over 5 MB return HTTP 413 or 400.
- [ ] Lane 5. Exercise `POST /api/v1/plugins/<fixture>/auth/begin`, `complete`, `status`, and `disconnect` and inspect `plugin_configs` in `$SHELFD_RUNTIME_DIR/data/sqlite.db`. Save `/tmp/swarm-pr-plugin-actions-host/worker-5/auth-pkce-encryption.png`. Pass when stored token values start with `enc:v1:` in SQLite and `GET /api/v1/plugins/<fixture>/config` returns `__SHELFD_REDACTED__`.
- [ ] Lane 6. Invoke a background job action with `IdleTimeoutSeconds` set to 2 seconds while the fixture job runs for 5 seconds emitting `progress` notifications. Save `/tmp/swarm-pr-plugin-actions-host/worker-6/job-lease-scale-to-zero.png`. Pass when the plugin stays alive until job completion, SSE emits `job_progress`, and the process scales to `idle` after the job completes.
- [ ] Lane 7. Configure 2-second invocation token TTL in fixture test mode and run a 4-second job that calls `ctx.getBookMarkdown` after refreshing its token via `host/refresh_token`. Save `/tmp/swarm-pr-plugin-actions-host/worker-7/token-refresh-loopback.png`. Pass when loopback `GET /api/v1/books/{id}/markdown` succeeds with HTTP 200 after initial token expiry.
- [ ] Lane 8. Verify `GET /api/v1/books/{id}/markdown` on seeded `Dune` after adding a highlight and note via API. Save `/tmp/swarm-pr-plugin-actions-host/worker-8/generic-book-markdown.png`. Pass when response `Content-Type` is `text/markdown` and body contains metadata, highlight quote, note text, and chapter text.
- [ ] Lane 9. Verify `GET /api/v1/series/{id}` for seeded `Dune Chronicles` and check `books:read` plugin scope access to `/api/v1/series/{id}` and `/api/v1/audiobooks/{id}/chapters` while attempting `DELETE /api/v1/books/{id}`. Save `/tmp/swarm-pr-plugin-actions-host/worker-9/scope-boundaries.png`. Pass when `GET` requests return HTTP 200 and `DELETE` returns HTTP 403.
- [ ] Lane 10. Verify `shelfd-plugin verify` CLI execution against both `plugins/project-gutenberg` and the action fixture plugin, then audit browser console on `/books`. Save `/tmp/swarm-pr-plugin-actions-host/worker-10/cli-verify-clean.png`. Pass when all `shelfd-plugin verify` checks report `ok` and browser console has zero errors.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. `GET /api/v1/books/{id}/markdown` p95 latency in milliseconds on trunk (`GET /api/v1/books/{id}/notebooklm`) and head (`GET /api/v1/books/{id}/markdown`), plus p95 overhead of `POST /api/v1/plugins/<fixture>/actions/demo/prepare` over warm stdio JSON-RPC.
- [ ] Probe. Run 50 interleaved authenticated requests to the Markdown export endpoint at trunk and head against seeded `Dune`, and 50 warm `actions/prepare` calls at head.
- [ ] Baseline. Record the trunk p95 Markdown export latency in milliseconds first.
- [ ] Rule. Head Markdown export p95 must not exceed trunk p95 by more than 15 percent, and warm `actions/prepare` p95 must stay under 25 milliseconds.

**Review gate.** None. `pr-plugin-actions-host` is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] After the verdict, an owner merge uses Shipping step 4 to move the PR from its recorded patch base onto current trunk. An appended stack child keeps its recorded parent tip until that parent lands. Record the current landing SHA. Preserve the verdict only when the patch ID stays unchanged.
- [ ] Root appends `pr-plugin-actions-host` to the frozen bottom-to-top stack targeting `main`, and the operator lands the stack in order.

## Build in-repo Yoto and NotebookLM plugins (pr-builtin-yoto-notebooklm-plugins)

**Depends on.** `pr-plugin-actions-host`.

**Files.**

- [ ] Create `plugins/yoto/package.json`.
- [ ] Create `plugins/yoto/plugin.json`.
- [ ] Create `plugins/yoto/src/index.ts`.
- [ ] Create `plugins/yoto/src/yoto-client.ts`.
- [ ] Create `plugins/yoto/src/image-16x16.ts`.
- [ ] Create `plugins/yoto/test/yoto.test.mjs`.
- [ ] Create `plugins/notebooklm/package.json`.
- [ ] Create `plugins/notebooklm/plugin.json`.
- [ ] Create `plugins/notebooklm/src/index.ts`.
- [ ] Create `plugins/notebooklm/test/notebooklm.test.mjs`.
- [ ] Edit `Dockerfile`.
- [ ] Edit `docker/entrypoint.sh`.
- [ ] Edit `docker/entrypoint_test.sh`.
- [ ] Edit `turbo.json`.
- [ ] Edit `apps/server/internal/api/plugins_test.go`.

**Build.**

- [ ] Create `plugins/yoto/plugin.json` declaring `"capabilities"` as `["actions", "tools", "events"]`, `"api_scopes"` as `["books:read"]`, `"permissions"` with `"network_domains"` set to `["api.yotoplay.com", "login.yotoplay.com"]` and `"library_read"` set to `true`, `config_schema` (`client_id`, `client_secret` secret, `api_base_url`, `auth_base_url`, plus encrypted `access_token`, `refresh_token`, `token_expires_at`), `auth` supporting `"oauth_pkce"` and `"token"`, and action `send_to_yoto` targeting `["book", "series"]` with `media_types` set to `["audiobook"]`, `mode` set to `"form"`, and `execution` set to `"job"`.
- [ ] Implement `plugins/yoto/src/yoto-client.ts` porting OAuth code exchange, automatic refresh-token rotation with `ctx.patchConfig`, `getMyPlaylists` (with card chapter count enrichment), `getCard`, `createOrUpdateContent` (supporting `"create"`, `"append"`, and `"replace"` strategies), `getPublicIcons`, `getUserIcons`, `uploadCustomIcon`, `requestAudioUploadUrl`, streaming `uploadAudioStream` using Node `fetch` with `ReadableStream` body and `duplex` set to `"half"`, `waitForTranscoding`, and `uploadCoverImage`.
- [ ] Implement pure TypeScript 16x16 PNG resize and encode in `plugins/yoto/src/image-16x16.ts` (decoding PNG and JPEG headers/pixels or lightweight pure-JS decoder + Node `node:zlib` deflate CRC32 PNG encoder) so custom icon uploads and `"Auto-convert cover art to 16x16 icon"` work identically in Node 22 with zero native C++ addons.
- [ ] Implement `plugins/yoto/src/index.ts` exposing `auth/{status,begin,complete,disconnect}`, `actions/prepare` (resolving single-book chapters or ordered series audiobooks via loopback `/api/v1/books/{id}`, `/api/v1/audiobooks/{id}/chapters`, and `/api/v1/series/{id}`, and returning the declarative `ActionFormSpec` with target mode, conditional MYO playlist `remote_select` or new card title `text`, cover upload `boolean`, auto-cover icon `boolean`, and 16x16 `asset_picker` with `public` and `custom` tabs, custom upload, and `per_entry_override`), `actions/options`, `actions/upload_asset`, `actions/invoke` (streaming audio files to Yoto S3 with `ctx.reportProgress` per chapter/book), and MCP tools `list_playlists` and `send_to_yoto`.
- [ ] Create `plugins/notebooklm/plugin.json` declaring `"capabilities"` as `["actions", "tools"]`, `"api_scopes"` as `["books:read"]`, `"permissions"` with `"network_domains"` set to `["notebooklm.google.com", "discoveryengine.googleapis.com"]` and `"library_read"` set to `true`, `config_schema` (`notebook_url`, `webhook_url`, `endpoint`, `project_id`, `location`, and secret `api_key`), and immediate action `send_to_notebooklm` targeting `["book"]` with `media_types` set to `["ebook"]`.
- [ ] Implement `plugins/notebooklm/src/index.ts` with dial-time SSRF validation rejecting loopback, link-local, RFC1918, and cloud metadata IPs on `webhook_url` and `endpoint` (unless explicitly overridden in test mode), pushing compiled Markdown from `ctx.getBookMarkdown(bookId)` to configured webhook or Google Cloud Discovery Engine (`v1alpha` create notebook + raw source upload), returning `status` `"completed"` when pushed server-side or `status` `"client_handoff"` (`download_api_path` set to `/api/v1/books/${bookId}/markdown`, `open_url` set to `notebook_url`) when unconfigured, plus MCP tool `send_book_to_notebooklm`.
- [ ] Update `Dockerfile` (`plugin-builder` stage) to build and copy `plugins/yoto` and `plugins/notebooklm` into `/out/builtin-plugins/`, update `docker/entrypoint.sh` (and `docker/entrypoint_test.sh`) so built-in `plugin.json` and `dist/` refresh cleanly across container image updates while preserving `/data/plugins/<name>/.data`, and add `@shelfd/plugin-yoto#build` and `@shelfd/plugin-notebooklm#build` to `@shelfd/server#test` `dependsOn` in `turbo.json`.
- [ ] Add end-to-end Go host integration tests in `apps/server/internal/api/plugins_test.go` running the compiled `plugins/yoto` and `plugins/notebooklm` bundles against local `httptest.Server` Yoto, Webhook, and Discovery Engine stubs.

**You see.**

- [ ] `shelfd-plugin verify` passes on `plugins/yoto` and `plugins/notebooklm`, `GET /api/v1/plugins` lists `project-gutenberg`, `yoto`, and `notebooklm` with healthy status, and invoking `yoto` or `notebooklm` actions via `/api/v1/plugins/{name}/actions/...` completes end to end.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `plugins/yoto/test/yoto.test.mjs` covers PKCE exchange, token refresh + `config/patch`, 16x16 PNG normalization, playlist and icon option queries, streaming S3 audio upload, transcoding poll, create/append/replace card assembly, and standalone bundle verification. Run `pnpm --filter @shelfd/plugin-yoto run test`.
- [ ] `plugins/notebooklm/test/notebooklm.test.mjs` covers zero-config `client_handoff`, webhook push, Discovery Engine two-step notebook create + raw upload, SSRF rejection on `169.254.169.254` and `[fd00:ec2::254]`, and standalone bundle verification. Run `pnpm --filter @shelfd/plugin-notebooklm run test`.
- [ ] `docker/entrypoint_test.sh` and `apps/server/internal/api/plugins_test.go` verify seeding of all three built-in plugins and full Go-to-Node E2E execution of `yoto` and `notebooklm`. Run `./docker/entrypoint_test.sh && cd apps/server && go test -tags sqlite_fts5 -ldflags="-linkmode=external" -run 'TestAPI_Real.*Plugin' -v ./internal/api`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` role at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Open `/settings` and `/plugins/catalog` on trunk and head. Save `/tmp/swarm-pr-builtin-yoto-notebooklm-plugins/worker-1/plugins-settings-regression.png`. Pass when `project-gutenberg` still browses and stages books while `yoto` and `notebooklm` appear as enabled cards in `/settings`.
- [ ] Lane 2. Link `yoto` plugin via `POST /api/v1/plugins/yoto/auth/begin` and `complete` against a local Yoto OAuth mock stub and open `PluginConfigDialog` in `/settings`. Save `/tmp/swarm-pr-builtin-yoto-notebooklm-plugins/worker-2/yoto-oauth-linked.png`. Pass when `GET /api/v1/plugins/yoto/auth/status` returns `linked` true and encrypted tokens persist in `plugin_configs`.
- [ ] Lane 3. Call `POST /api/v1/plugins/yoto/actions/send_to_yoto/prepare` for seeded `The Tale of Peter Rabbit` (which includes playable MP3 audio). Save `/tmp/swarm-pr-builtin-yoto-notebooklm-plugins/worker-3/yoto-prepare-book.png`. Pass when response returns `ActionFormSpec` with target mode, playlist `remote_select`, 16x16 `asset_picker`, and resolved chapter entries.
- [ ] Lane 4. Query `GET /api/v1/plugins/yoto/actions/send_to_yoto/options?source=icons&tab=public&q=book` and upload a 64x64 PNG via `POST /api/v1/plugins/yoto/actions/send_to_yoto/assets`. Save `/tmp/swarm-pr-builtin-yoto-notebooklm-plugins/worker-4/yoto-icons-16x16.png`. Pass when the mock Yoto icon endpoint receives a normalized 16x16 PNG and returns a `yoto://#...` URI.
- [ ] Lane 5. Invoke `POST /api/v1/plugins/yoto/actions/send_to_yoto/invoke` in `"create"` mode on `The Tale of Peter Rabbit` against a local Yoto S3/transcode/content mock and stream `/api/v1/queue/events`. Save `/tmp/swarm-pr-builtin-yoto-notebooklm-plugins/worker-5/yoto-book-create-job.png`. Pass when `job_progress` events stream to `"completed"` and mock Yoto `/content` receives the card with cover and chapter icon URIs.
- [ ] Lane 6. Invoke `POST /api/v1/plugins/yoto/actions/send_to_yoto/invoke` in `"append"` and `"replace"` modes on an existing mock MYO card. Save `/tmp/swarm-pr-builtin-yoto-notebooklm-plugins/worker-6/yoto-append-replace.png`. Pass when `"append"` preserves prior chapters before new chapters and `"replace"` overwrites chapters on the target `cardId`.
- [ ] Lane 7. Seed a two-audiobook series and invoke `send_to_yoto` targeting `type` `"series"`. Save `/tmp/swarm-pr-builtin-yoto-notebooklm-plugins/worker-7/yoto-series-job.png`. Pass when both audiobooks upload in sequence order into one MYO card with per-book chapter titles.
- [ ] Lane 8. Invoke `POST /api/v1/plugins/notebooklm/actions/send_to_notebooklm/invoke` on seeded `Dune` with default unconfigured `notebooklm` plugin settings. Save `/tmp/swarm-pr-builtin-yoto-notebooklm-plugins/worker-8/notebooklm-client-handoff.png`. Pass when response returns `status` `"client_handoff"`, `download_api_path` `/api/v1/books/<id>/markdown`, and `open_url` `https://notebooklm.google.com/`.
- [ ] Lane 9. Configure `notebooklm` plugin `webhook_url` and Discovery Engine `endpoint` against local mock servers and invoke `send_to_notebooklm`, then test `webhook_url` set to `http://169.254.169.254/latest/meta-data`. Save `/tmp/swarm-pr-builtin-yoto-notebooklm-plugins/worker-9/notebooklm-push-and-ssrf.png`. Pass when valid mock pushes return `status` `"completed"` and metadata IP targets fail closed with SSRF error before dialing.
- [ ] Lane 10. Call MCP `tools/list` and `tools/call` via `control-shelfd mcp` for `yoto__list_playlists` and `notebooklm__send_book_to_notebooklm`. Save `/tmp/swarm-pr-builtin-yoto-notebooklm-plugins/worker-10/mcp-plugin-tools.png`. Pass when both plugin tools appear in `tools/list` and return structured Markdown output under 4,000 characters.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Peak child Node RSS in megabytes and end-to-end transfer duration in milliseconds while uploading a 64 MB synthetic M4B audiobook to a local mock S3 endpoint at trunk (`POST /api/v1/books/{id}/yoto`) and head (`POST /api/v1/plugins/yoto/actions/send_to_yoto/invoke`).
- [ ] Probe. Run 5 interleaved 64 MB audiobook uploads at trunk and head against the same in-memory HTTP sink, sampling Go RSS at trunk and Go + Node child RSS (`/proc/<pid>/status` `VmRSS`) at head every 50 milliseconds.
- [ ] Baseline. Record the trunk transfer duration and server RSS delta first.
- [ ] Rule. Node `yoto` plugin RSS must stay below 96 MB throughout the 64 MB stream (proving chunked streaming without full-file buffering) and end-to-end transfer duration at head must stay within 25 percent of trunk.

**Review gate.** None. `pr-builtin-yoto-notebooklm-plugins` is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] After the verdict, an owner merge uses Shipping step 4 to move the PR from its recorded patch base onto current trunk. An appended stack child keeps its recorded parent tip until that parent lands. Record the current landing SHA. Preserve the verdict only when the patch ID stays unchanged.
- [ ] Root appends `pr-builtin-yoto-notebooklm-plugins` onto `pr-plugin-actions-host` in the frozen bottom-to-top stack, and the operator lands the stack in order.

## Render declarative plugin input interfaces and job progress in Flutter (pr-flutter-declarative-plugin-ui)

**Depends on.** `pr-builtin-yoto-notebooklm-plugins`.

**Files.**

- [ ] Edit `apps/app/lib/data/models/plugin.dart`.
- [ ] Edit `apps/app/lib/data/services/api_service.dart`.
- [ ] Edit `apps/app/lib/data/services/storage_service.dart`.
- [ ] Edit `apps/app/lib/data/repositories/book_repository.dart`.
- [ ] Edit `apps/app/lib/ui/core/file_saver/file_saver_io.dart`.
- [ ] Edit `apps/app/lib/ui/state/plugins_provider.dart`.
- [ ] Edit `apps/app/lib/ui/state/providers.dart`.
- [ ] Create `apps/app/lib/ui/features/plugins/plugin_action_dialog.dart`.
- [ ] Create `apps/app/lib/ui/features/plugins/plugin_asset_picker_dialog.dart`.
- [ ] Create `apps/app/lib/ui/features/plugins/plugin_action_runner.dart`.
- [ ] Edit `apps/app/lib/ui/features/settings/plugin_config_dialog.dart`.
- [ ] Edit `apps/app/lib/ui/features/settings/settings_view.dart`.
- [ ] Edit `apps/app/lib/ui/features/book_detail/book_detail_view.dart`.
- [ ] Edit `apps/app/lib/ui/features/series/series_detail_view.dart`.
- [ ] Edit `apps/app/lib/ui/core/shared_layout.dart`.
- [ ] Delete `apps/app/lib/data/models/yoto.dart`.
- [ ] Delete `apps/app/lib/data/repositories/yoto_repository.dart`.
- [ ] Delete `apps/app/lib/ui/state/yoto_provider.dart`.
- [ ] Delete `apps/app/lib/ui/features/book_detail/send_to_yoto_dialog.dart`.
- [ ] Delete `apps/app/lib/ui/features/book_detail/yoto_icon_picker_dialog.dart`.
- [ ] Delete `apps/app/test/ui/features/yoto_integration_test.dart`.
- [ ] Create `apps/app/test/ui/features/plugin_actions_and_auth_test.dart`.
- [ ] Edit `apps/app/test/data/models/plugin_model_test.dart`.
- [ ] Edit `apps/app/test/data/services/api_service_test.dart`.
- [ ] Edit `apps/app/test/ui/features/plugins_catalog_and_config_test.dart`.
- [ ] Edit `apps/app/test/ui/features/book_detail_view_test.dart`.
- [ ] Edit `apps/app/test/ui/features/series_detail_view_test.dart`.
- [ ] Edit `apps/app/test/ui/tactile_scriptorium_e2e_test.dart`.
- [ ] Edit `apps/server/internal/api/ui_e2e_test.go`.
- [ ] Edit `apps/server/internal/api/yoto_test.go`.

**Build.**

- [ ] Add Dart models `PluginAuthSpec`, `PluginAuthStatus`, `PluginActionDef`, `PluginActionFormSpec`, sealed or discriminated `PluginInputFieldSpec` (`TextFieldSpec`, `BooleanFieldSpec`, `SelectFieldSpec`, `SegmentedFieldSpec`, `RemoteSelectFieldSpec`, `AssetPickerFieldSpec`), `PluginFieldVisibilityRule`, `PluginFormEntry`, `PluginOptionItem`, `PluginActionResult`, and `PluginJob` in `apps/app/lib/data/models/plugin.dart`.
- [ ] Add `getPluginAuthStatus`, `beginPluginAuth`, `completePluginAuth`, `disconnectPluginAuth`, `preparePluginAction`, `getPluginActionOptions`, `uploadPluginActionAsset`, `invokePluginAction`, `getPluginJobs`, `dismissPluginJob`, and `downloadBookMarkdown` (`GET /api/v1/books/$bookId/markdown`) in `apps/app/lib/data/services/api_service.dart`, while deleting all `Yoto*` and `NotebookLM*` methods and DTOs from `api_service.dart`, `book_repository.dart`, `storage_service.dart` (replacing Yoto-specific PKCE keys with generic `plugin_pkce_<pluginName>` keys), and `file_saver_io.dart`.
- [ ] Extend `PluginConfigDialog` (`apps/app/lib/ui/features/settings/plugin_config_dialog.dart`) and `SettingsView` (`apps/app/lib/ui/features/settings/settings_view.dart`) to render any plugin's `auth` block (connected account badge, OAuth 2.0 PKCE S256 browser launch + `/settings?plugin=<name>&code=...` automatic callback completion + manual code paste + token paste + disconnect button, plus `401` reconnect prompt), and delete the hardcoded `yoto_integration_settings_card` from `SettingsView`.
- [ ] Build `PluginActionDialog` (`apps/app/lib/ui/features/plugins/plugin_action_dialog.dart`) and `PluginAssetPickerDialog` (`apps/app/lib/ui/features/plugins/plugin_asset_picker_dialog.dart`) that call `preparePluginAction` and render any plugin's declared `PluginInputFieldSpec` list with reactive `visibleWhen` evaluation, searchable `remote_select` lists, unlinked-account CTA opening `PluginConfigDialog`, `FilterQuality.none` pixel-art previews for `square_pixel` assets, multipart custom asset upload, and expandable per-entry overrides over `form.entries`.
- [ ] Build `PluginActionRunner` (`apps/app/lib/ui/features/plugins/plugin_action_runner.dart`) handling both `mode == 'immediate'` and `mode == 'form'` actions, executing `completed` toasts/links, `client_handoff` authenticated file download + `openExternalUrl`, and `job_started` queue tracking.
- [ ] Replace hardcoded `Send to NotebookLM` and `Send to Yoto` items in `BookDetailView` (`apps/app/lib/ui/features/book_detail/book_detail_view.dart`) and `SeriesDetailView` (`apps/app/lib/ui/features/series/series_detail_view.dart`) with dynamic menu/action items derived from enabled plugins in `pluginsProvider` matching target (`book` or `series`) and media availability (`ebook` or `audiobook`), keeping `Download Markdown (.md)` wired to `downloadBookMarkdown`.
- [ ] Replace `YotoUploadQueueBanner` in `apps/app/lib/ui/core/shared_layout.dart` and `QueueNotifier` SSE handling in `apps/app/lib/ui/state/providers.dart` with generic `PluginJobsBanner` (`Key('plugin_jobs_banner')`) driven by `pluginJobsProvider` (`GET /api/v1/plugins/jobs` + live `PluginEvent` `"job_progress"` updates + dismissible error toast).
- [ ] Delete `yoto.dart`, `yoto_repository.dart`, `yoto_provider.dart`, `send_to_yoto_dialog.dart`, `yoto_icon_picker_dialog.dart`, and `yoto_integration_test.dart`, add `plugin_actions_and_auth_test.dart`, and update widget-key contract assertions in `apps/server/internal/api/ui_e2e_test.go` (`F06_T1_05` and `F12_T1_04`) and `apps/server/internal/api/yoto_test.go` (`TestYotoUIContractVerification`) so both `flutter test` and `go test ./...` pass at PR 3's commit.

**You see.**

- [ ] Book Detail and Series Detail menus dynamically list `Send to Yoto` (for audiobooks) and `Send to NotebookLM` (for ebooks) when their plugins are enabled, hide them immediately when toggled off in Settings, render Yoto's full playlist/icon/per-chapter form via `PluginActionDialog`, stream job progress in `PluginJobsBanner`, and manage Yoto OAuth PKCE inside `PluginConfigDialog`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `apps/app/test/data/models/plugin_model_test.dart` and `apps/app/test/data/services/api_service_test.dart` verify serialization and HTTP calls for `PluginActionDef`, `PluginActionFormSpec`, `PluginAuthSpec`, `PluginJob`, `client_handoff`, and `downloadBookMarkdown`. Run `cd apps/app && flutter test test/data/models/plugin_model_test.dart test/data/services/api_service_test.dart`.
- [ ] `apps/app/test/ui/features/plugin_actions_and_auth_test.dart`, `plugins_catalog_and_config_test.dart`, `book_detail_view_test.dart`, `series_detail_view_test.dart`, and `tactile_scriptorium_e2e_test.dart` verify `PluginActionDialog` conditional fields, `PluginAssetPickerDialog` tabs/search/upload/per-entry overrides, `PluginConfigDialog` PKCE S256 + token linking, dynamic book/series action menus, and `PluginJobsBanner` progress/error states, plus `flutter analyze`. Run `cd apps/app && flutter analyze --no-fatal-infos --no-fatal-warnings && flutter test`.
- [ ] `apps/server/internal/api/ui_e2e_test.go` passes updated UI widget-key integrity checks. Run `cd apps/server && go test -tags sqlite_fts5 -ldflags="-linkmode=external" -run 'TestUIE2E' ./internal/api`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` role at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Open `Dune` Book Detail menu and click `Download Markdown (.md)` on trunk and head, and open `The Tale of Peter Rabbit` to trigger `Send to Yoto`. Save `/tmp/swarm-pr-flutter-declarative-plugin-ui/worker-1/book-actions-regression.png`. Pass when Markdown export downloads cleanly and `Send to Yoto` opens `PluginActionDialog` with parity fields.
- [ ] Lane 2. Navigate to `/settings`, verify legacy `Yoto Player Integration` card is gone, click `Configure` on `yoto` plugin card, link via PKCE callback `/settings?plugin=yoto&code=mock-code` against mock Yoto auth, and reopen `Configure`. Save `/tmp/swarm-pr-flutter-declarative-plugin-ui/worker-2/settings-plugin-pkce-auth.png`. Pass when `PluginConfigDialog` displays linked account status and `Disconnect` button.
- [ ] Lane 3. Open `The Tale of Peter Rabbit` Book Detail while `yoto` is unlinked, click `Send to Yoto`, and verify the unlinked prompt inside `PluginActionDialog` opens `PluginConfigDialog` directly. Save `/tmp/swarm-pr-flutter-declarative-plugin-ui/worker-3/unlinked-action-cta.png`. Pass when `Configure` CTA opens `PluginConfigDialog` without navigating away from Book Detail.
- [ ] Lane 4. With `yoto` linked, open `Send to Yoto` on `The Tale of Peter Rabbit`, toggle between `Create New Card` and `Existing Card`, and search playlists in the `remote_select` field. Save `/tmp/swarm-pr-flutter-declarative-plugin-ui/worker-4/declarative-form-conditional-fields.png`. Pass when `New Card Title` hides in `Existing Card` mode and filtered MYO playlists render with chapter counts.
- [ ] Lane 5. Inside `PluginActionDialog`, open `PluginAssetPickerDialog`, search `Public Icons`, switch to `My Icons`, upload a custom PNG icon, and assign a per-chapter override icon on Chapter 1. Save `/tmp/swarm-pr-flutter-declarative-plugin-ui/worker-5/declarative-asset-picker.png`. Pass when 16x16 icons render crisply, custom upload selects the new icon, and Chapter 1 displays its override icon.
- [ ] Lane 6. Submit `Send to Yoto` on `The Tale of Peter Rabbit` with a 2-second mock transcode delay and capture `PluginJobsBanner` in `AppShell` during upload and upon completion. Save `/tmp/swarm-pr-flutter-declarative-plugin-ui/worker-6/plugin-jobs-banner.png`. Pass when `PluginJobsBanner` shows live progress bar updates via SSE and clears cleanly on completion.
- [ ] Lane 7. Open a seeded audiobook series in `/series/:id`, click `Send to Yoto` series action button, and submit a multi-book MYO card job. Save `/tmp/swarm-pr-flutter-declarative-plugin-ui/worker-7/series-plugin-action.png`. Pass when `PluginActionDialog` lists each audiobook in `entries` and completes the multi-book upload job.
- [ ] Lane 8. Open `Dune` Book Detail, click `Send to NotebookLM` from the action menu in zero-config handoff mode, then configure `notebooklm` webhook URL in `/settings` and click `Send to NotebookLM` again. Save `/tmp/swarm-pr-flutter-declarative-plugin-ui/worker-8/notebooklm-plugin-action.png`. Pass when handoff mode downloads `Dune.md` and webhook mode displays the completion toast.
- [ ] Lane 9. Disable both `yoto` and `notebooklm` switches in `/settings`, then revisit `Dune` Book Detail and `The Tale of Peter Rabbit` Book Detail and Series Detail. Save `/tmp/swarm-pr-flutter-declarative-plugin-ui/worker-9/disabled-plugins-hide-actions.png`. Pass when neither `Send to Yoto` nor `Send to NotebookLM` appears in menus while `Download EPUB` and `Download Markdown (.md)` remain.
- [ ] Lane 10. Test responsive mobile viewport (`390x844`) and theme switch (`Bone`, `Sepia`, `Dark`) across `PluginActionDialog`, `PluginAssetPickerDialog`, and `PluginConfigDialog`, checking `browser console --assert-clean`. Save `/tmp/swarm-pr-flutter-declarative-plugin-ui/worker-10/mobile-and-themes.png`. Pass when zero RenderFlex overflows and zero console errors occur across all three themes.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Time in milliseconds from clicking `Send to Yoto` on `The Tale of Peter Rabbit` to interactive dialog render in headless Chromium at trunk (`SendToYotoDialog`) and head (`PluginActionDialog` backed by `actions/prepare`).
- [ ] Probe. Run 10 interleaved CDP `click` to ARIA-ready dialog measurements at trunk and head against the local mock Yoto server with warm plugin process.
- [ ] Baseline. Record the trunk median and p95 dialog open latency in milliseconds first.
- [ ] Rule. Head p95 dialog open latency must stay under 350 milliseconds and within 75 milliseconds of trunk.

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 2, 4, 5, 6, and 8 screenshots into `docs/screenshots/pr-flutter-declarative-plugin-ui-review-auth.png`, `docs/screenshots/pr-flutter-declarative-plugin-ui-review-form.png`, `docs/screenshots/pr-flutter-declarative-plugin-ui-review-icons.png`, `docs/screenshots/pr-flutter-declarative-plugin-ui-review-jobs.png`, and `docs/screenshots/pr-flutter-declarative-plugin-ui-review-notebooklm.png`.
- [ ] Record a 30 to 60 second video of linking Yoto in `PluginConfigDialog`, customizing playlist and 16x16 chapter icons in `PluginActionDialog`, watching `PluginJobsBanner` complete, and triggering `Send to NotebookLM` on a live lane. Save it as `docs/screenshots/pr-flutter-declarative-plugin-ui-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for the operator's click.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] After the verdict, an owner merge uses Shipping step 4 to move the PR from its recorded patch base onto current trunk. An appended stack child keeps its recorded parent tip until that parent lands. Record the current landing SHA. Preserve the verdict only when the patch ID stays unchanged.
- [ ] Root appends `pr-flutter-declarative-plugin-ui` onto `pr-builtin-yoto-notebooklm-plugins` in the frozen bottom-to-top stack after operator review approval, and the operator lands the stack in order.

## Delete legacy Yoto and NotebookLM server code and schema (pr-delete-legacy-yoto-notebooklm-core)

**Depends on.** `pr-flutter-declarative-plugin-ui`.

**Files.**

- [ ] Delete `apps/server/internal/yoto/client.go`.
- [ ] Delete `apps/server/internal/yoto/client_test.go`.
- [ ] Delete `apps/server/internal/api/yoto.go`.
- [ ] Delete `apps/server/internal/api/yoto_test.go`.
- [ ] Create `apps/server/internal/database/migrations/0020_drop_yoto_accounts.sql`.
- [ ] Create `apps/server/internal/database/migrations_pg/0016_drop_yoto_accounts.sql`.
- [ ] Edit `apps/server/internal/repository/models.go`.
- [ ] Edit `apps/server/internal/repository/storage.go`.
- [ ] Edit `apps/server/internal/repository/sqlite.go`.
- [ ] Edit `apps/server/internal/repository/bun.go`.
- [ ] Edit `apps/server/internal/config/config.go`.
- [ ] Edit `apps/server/internal/config/config_test.go`.
- [ ] Edit `apps/server/internal/api/books.go`.
- [ ] Edit `apps/server/internal/api/books_test.go`.
- [ ] Edit `apps/server/internal/api/router.go`.
- [ ] Edit `apps/server/internal/api/ui_e2e_test.go`.
- [ ] Edit `apps/server/cmd/server/main.go`.
- [ ] Edit `config.example.yaml`.
- [ ] Edit `README.md`.
- [ ] Edit `.agents/skills/verify-shelfd/features/README.md`.
- [ ] Create `.agents/skills/verify-shelfd/features/plugins-and-outbound-actions.md`.

**Build.**

- [ ] Delete package `apps/server/internal/yoto` and handler `apps/server/internal/api/yoto.go` (`yoto_test.go`), and remove `/api/v1/integrations/yoto/*`, `POST /api/v1/books/{id}/yoto`, and `POST /api/v1/series/{id}/yoto` from `apps/server/internal/api/router.go`.
- [ ] Delete `ExportBookNotebookLM`, `SendBookToNotebookLM`, `NotebookLMSendResponse`, `notebookLMFilename`, and `SetNotebookLMConfig` from `apps/server/internal/api/books.go`, and remove `GET /api/v1/books/{id}/notebooklm` and `POST /api/v1/books/{id}/notebooklm` from `apps/server/internal/api/router.go` (retaining generic `GET /api/v1/books/{id}/markdown`).
- [ ] Delete `NotebookLMConfig`, `YotoConfig`, `DefaultNotebookLMURL`, `DefaultYotoAPIBaseURL`, `DefaultYotoAuthBaseURL`, `SHELFD_NOTEBOOKLM_*`, and `SHELFD_YOTO_*` from `apps/server/internal/config/config.go`, `apps/server/cmd/server/main.go`, and `config.example.yaml`.
- [ ] Delete `YotoAccount` (`apps/server/internal/repository/models.go`) and `GetYotoAccountByUserID`, `UpsertYotoAccount`, and `DeleteYotoAccount` from `storage.go`, `sqlite.go`, and `bun.go`, and add `DROP TABLE IF EXISTS yoto_accounts;` in `0020_drop_yoto_accounts.sql` (SQLite) and `0016_drop_yoto_accounts.sql` (Postgres).
- [ ] Update `config_test.go`, `books_test.go`, and `ui_e2e_test.go` (renaming `seedNotebookLMTestBook` to `seedMarkdownTestBook` and updating `F06_T1_04` and `T4_Journey04` to assert `GET /api/v1/books/{id}/markdown` plus plugin action routes).
- [ ] Update `README.md` and `.agents/skills/verify-shelfd/features/` (`README.md` and new `plugins-and-outbound-actions.md`) so operator and verification docs describe the built-in `yoto` and `notebooklm` plugins and declarative plugin actions.

**You see.**

- [ ] `git grep -i "yoto"` and `git grep -i "notebooklm"` inside `apps/server/` and `apps/app/` match zero files outside historical migrations `0018_yoto_accounts.sql`, `0014_yoto_accounts.sql`, `0020_drop_yoto_accounts.sql`, `0016_drop_yoto_accounts.sql`, and plugin integration tests in `plugins_test.go`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Full Go server test suite and `go vet` pass with SQLite migrations `0001` through `0020` applied cleanly. Run `cd apps/server && go vet -tags sqlite_fts5 ./... && go test -tags sqlite_fts5 -ldflags="-linkmode=external" -v ./...`.
- [ ] Full workspace lint, test, and build pass across SDK, plugins, server, and Flutter client. Run `pnpm lint && pnpm test && pnpm build`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` role at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Boot full stack, sign in via `/connect`, browse `/books`, read `Dune`, export `Dune.md`, and run `Send to Yoto` and `Send to NotebookLM` via plugins. Save `/tmp/swarm-pr-delete-legacy-yoto-notebooklm-core/worker-1/e2e-plugin-parity.png`. Pass when all user journeys succeed with zero legacy endpoints present.
- [ ] Lane 2. Probe removed routes `GET /api/v1/integrations/yoto/status`, `POST /api/v1/books/<id>/yoto`, `POST /api/v1/series/<id>/yoto`, `GET /api/v1/books/<id>/notebooklm`, and `POST /api/v1/books/<id>/notebooklm` via `control-shelfd api`. Save `/tmp/swarm-pr-delete-legacy-yoto-notebooklm-core/worker-2/legacy-routes-404.png`. Pass when every removed route returns HTTP 404 or 405.
- [ ] Lane 3. Inspect `$SHELFD_RUNTIME_DIR/data/sqlite.db` `sqlite_master` after fresh boot and after migrating a pre-0020 SQLite DB seeded with `yoto_accounts`. Save `/tmp/swarm-pr-delete-legacy-yoto-notebooklm-core/worker-3/yoto-table-dropped.png`. Pass when `yoto_accounts` is absent from `sqlite_master` and `plugin_configs` remains intact.
- [ ] Lane 4. Boot `shelfd` with legacy `yoto` and `notebooklm` keys omitted and verify `config.example.yaml` parses cleanly via `shelfd -config config.example.yaml` validation test. Save `/tmp/swarm-pr-delete-legacy-yoto-notebooklm-core/worker-4/clean-config-boot.png`. Pass when daemon boots cleanly with zero unknown-config errors.
- [ ] Lane 5. Execute the new `.agents/skills/verify-shelfd/features/plugins-and-outbound-actions.md` recipe end to end through `control-shelfd`. Save `/tmp/swarm-pr-delete-legacy-yoto-notebooklm-core/worker-5/verify-shelfd-feature-recipe.png`. Pass when every step in `plugins-and-outbound-actions.md` succeeds verbatim.
- [ ] Lane 6. Highlight a passage and add a note in `Dune` reader (`/books/:id/read`), then download Markdown via `Download Markdown (.md)` on Book Detail. Save `/tmp/swarm-pr-delete-legacy-yoto-notebooklm-core/worker-6/reader-highlight-markdown.png`. Pass when downloaded `.md` contains the new highlight and note via `GET /api/v1/books/{id}/markdown`.
- [ ] Lane 7. Play `The Tale of Peter Rabbit` in `/books/:id/listen` (Audiobook and Read-Along modes) and then send it to mock Yoto via `Send to Yoto` plugin action. Save `/tmp/swarm-pr-delete-legacy-yoto-notebooklm-core/worker-7/audiobook-and-yoto-plugin.png`. Pass when audio playback and plugin S3 upload both succeed without file-lock contention.
- [ ] Lane 8. Run `control-shelfd mcp` `tools/list` and reader `Chat with Book` on `Dune`. Save `/tmp/swarm-pr-delete-legacy-yoto-notebooklm-core/worker-8/mcp-and-book-chat.png`. Pass when core book tools and enabled plugin tools coexist cleanly and reader chat responds with citations.
- [ ] Lane 9. Verify `./docker/entrypoint_test.sh` seeding of `project-gutenberg`, `yoto`, and `notebooklm` and test uninstall/re-install of `notebooklm` `.zip` archive via `/settings`. Save `/tmp/swarm-pr-delete-legacy-yoto-notebooklm-core/worker-9/plugin-zip-reinstall.png`. Pass when `.zip` upload hot-reloads `notebooklm` cleanly.
- [ ] Lane 10. Audit source tree with `git grep -i "yoto" apps/server apps/app` and `git grep -i "notebooklm" apps/server apps/app` and check `browser console --assert-clean` across `/books`, `/series`, `/uploads`, `/plugins/catalog`, and `/settings`. Save `/tmp/swarm-pr-delete-legacy-yoto-notebooklm-core/worker-10/zero-legacy-residue.png`. Pass when grep matches only expected migration/plugin-test files and console error count is 0.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Compiled `apps/server/dist/shelfd` binary size in bytes and cold server startup-to-`/health` latency in milliseconds at trunk and head.
- [ ] Probe. Build `apps/server/dist/shelfd` at trunk and head with identical `go build -tags sqlite_fts5` flags and run 10 interleaved cold startups against an empty SQLite database with the three built-in plugins disabled, polling `/health` every 5 milliseconds.
- [ ] Baseline. Record the trunk binary byte size and median startup latency in milliseconds first.
- [ ] Rule. Head binary size must be smaller than trunk and cold startup median latency must not regress by more than 10 percent.

**Review gate.** None. `pr-delete-legacy-yoto-notebooklm-core` is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] After the verdict, an owner merge uses Shipping step 4 to move the PR from its recorded patch base onto current trunk. An appended stack child keeps its recorded parent tip until that parent lands. Record the current landing SHA. Preserve the verdict only when the patch ID stays unchanged.
- [ ] Root appends `pr-delete-legacy-yoto-notebooklm-core` onto `pr-flutter-declarative-plugin-ui` at the top of the frozen stack, and the operator lands the verified stack bottom-up.

## Close the program

- [ ] Every box above is checked with its evidence.
- [ ] Reply to the operator with the report the execution playbook names.

## Appendix A. Prototype evidence

Codebase inspection against commit `HEAD` on `main` settled the architectural questions without speculative external calls.

- Out-of-process plugin foundation already exists in `apps/server/internal/plugins/host.go` (`Host`), `packages/plugin-sdk/src/index.ts` (`definePlugin`), `Dockerfile` (`plugin-builder` stage), and `docker/entrypoint.sh` (`/usr/share/shelfd/builtin-plugins` seeding), currently shipping `plugins/project-gutenberg` with capabilities `tools`, `commands`, `events`, and `catalog`.
- Loopback streaming already exists for audio files at `GET /api/v1/books/{id}/files/{fileId}/stream` (`apps/server/internal/api/audiobooks.go`) and matches `/api/v1/books/` in `PluginScopeAllowsRequest` (`apps/server/internal/api/middleware.go`), while `GET /api/v1/audiobooks/{id}/chapters` and `GET /api/v1/series/{id}` were missing from `books:read` and are added in `pr-plugin-actions-host`.
- `ExportBookNotebookLM` (`apps/server/internal/api/books.go`) compiles generic book Markdown (metadata, synopsis, highlights, notes, and DB or live-EPUB chapters) that is also used by the standalone `Download Markdown (.md)` button in `apps/app/lib/ui/features/book_detail/book_detail_view.dart`; promoting `GET /api/v1/books/{id}/markdown` keeps generic Markdown export in core while allowing `plugins/notebooklm` to operate purely over `books:read`.
- Cross-workspace test inspection uncovered Go tests in `apps/server/internal/api/yoto_test.go` (`TestYotoUIContractVerification`) and `apps/server/internal/api/ui_e2e_test.go` (`F06_T1_05` and `F12_T1_04`) that read `.dart` files directly from `apps/app/lib/`, dictating that `pr-flutter-declarative-plugin-ui` updates those assertions in lockstep with the Dart UI cutover so `pnpm test` stays green on every PR.
- Live credentials for production `api.yotoplay.com` and Google Cloud Discovery Engine stay unproven in automated lanes and are exercised through local HTTP mock servers in `Verify, live` per `.agents/skills/verify-shelfd/features/README.md`.

## Appendix B. Alternatives rejected

- Hardcoding a `"destination"` capability modeled specifically around Yoto playlists and 16x16 icons. Rejected because it leaks Yoto domain concepts back into Go and Flutter under a new name instead of letting any plugin declare its own input interface (`text`, `boolean`, `select`, `segmented`, `remote_select`, `asset_picker`, `visible_when`, and `per_entry_override`).
- Rendering raw arbitrary HTML or plugin-supplied iframes for plugin action dialogs. Rejected because arbitrary webviews break Shelfd's tactile Bone, Sepia, and Dark design tokens, complicate accessibility semantics in Flutter Web and desktop/mobile builds, and widen XSS surface compared to a typed declarative field union.
- Passing local `/library` filesystem paths into plugin child processes so `plugins/yoto` reads audio files directly from disk. Rejected because ADR 0022 isolates plugins behind scoped loopback HTTP (`books:read`) and Audiobookshelf Sanctity forbids plugins from touching `/library` inodes directly; streaming `GET /api/v1/books/{id}/files/{fileId}/stream` into Node `fetch` with `ReadableStream` keeps memory flat while honoring the boundary.
- Requiring long-transfer action plugins to set `"daemon": true` or inflating plugin invocation JWT TTL to hours. Rejected because `"daemon": true` wastes memory when idle and long-lived tokens weaken scope isolation; host-managed `PluginJob` leases plus `"host/refresh_token"` keep scale-to-zero intact and tokens short-lived.
- Retaining `yoto_accounts` compatibility shims or dual-reading legacy `config.yaml` `yoto` and `notebooklm` blocks. Rejected per explicit operator direction as sole user, allowing clean subtraction in `pr-delete-legacy-yoto-notebooklm-core`.

## Appendix C. Risks

- Scale-to-zero reaper killing `plugins/yoto` mid-transcode (`pr-plugin-actions-host`, `pr-builtin-yoto-notebooklm-plugins`). Owner watches `PluginJobManager` lease accounting on `managedPlugin` and verifies Lane 6 with a 2-second idle timeout during a 5-second active job.
- Memory spikes when uploading multi-hundred-MB `.m4b` files through Node (`pr-plugin-actions-host`, `pr-builtin-yoto-notebooklm-plugins`). Owner avoids `arrayBuffer()` on audio streams, pipes web `ReadableStream` with `Content-Length` and `duplex` set to `"half"`, and verifies `< 96 MB` Node RSS in PR 2's perf probe.
- Pure-JS 16x16 icon normalization fidelity across PNG and JPEG inputs (`pr-builtin-yoto-notebooklm-plugins`). Owner ports `NormalizeIcon16x16` test vectors from `apps/server/internal/yoto/client_test.go` into `plugins/yoto/test/yoto.test.mjs` and rejects unsupported image formats with a clear validation error.
- SSRF exposure when `plugins/notebooklm` dials user-configured `webhook_url` or `endpoint` (`pr-builtin-yoto-notebooklm-plugins`). Owner enforces DNS resolution and IP range checks in `plugins/notebooklm` rejecting loopback, RFC1918, link-local, and cloud metadata (`169.254.0.0/16`, `fe80::/10`, `fd00:ec2::254`) addresses, verified in PR 2 Lane 9.
- Built-in plugin staleness across Docker upgrades when `/data/plugins/<name>` already exists on a persistent volume (`pr-builtin-yoto-notebooklm-plugins`). Owner updates `docker/entrypoint.sh` to refresh `plugin.json` and `dist/` for built-in plugins while preserving `/data/plugins/<name>/.data` and SQLite `plugin_configs`.
- Ephemeral verification VMs lacking preinstalled `flutter` binary on `PATH` (`pr-flutter-declarative-plugin-ui`, `pr-delete-legacy-yoto-notebooklm-core`). Owner follows the boot recipe to check `command -v flutter` and install Flutter stable into user space (`/tmp/flutter`) before running `flutter test` or `control-shelfd launch`.

## Appendix D. Links and reading list

- ADR 0022 Out-of-Process Plugin System in `docs/decisions/0022-out-of-process-plugin-system-and-catalog-importers.md`.
- Plugin Host Maintainer Guardrails in `.agents/skills/plugin-host-maintainer.md`.
- Create Shelfd Plugin Skill in `skills/create-shelfd-plugin/SKILL.md` and `skills/create-shelfd-plugin/references/protocol-and-sdk.md`.
- Live Shelfd Verification Harness in `.agents/skills/verify-shelfd/SKILL.md` and `.agents/skills/verify-shelfd/features/README.md`.
- Flutter Pre-PR Checklist and Design Tokens in `.agents/skills/flutter-pre-pr-checklist.md` and `.agents/skills/stitch-ui-designer.md`.
- Every PR owner runs `/shared/user/.gemini/skills/how/SKILL.md` before coding and `/shared/user/.gemini/skills/interrogate/SKILL.md` across `pr-plugin-actions-host` and `pr-flutter-declarative-plugin-ui` RPC/UI boundaries, recording decisions per `/shared/user/.gemini/skills/show-me-your-work/SKILL.md`.
