# Antigravity tool mapping for pstack

pstack skills retain Claude Code tool language (`Skill`, `Agent`, `AskUserQuestion`, `View`, `Edit`, `Write`, `Bash`, `Grep`, `Glob`, `LS`) in shared prose. In Google Antigravity, the files are the same; those tool names resolve to Antigravity's native tools and primitives. Model execution is not translated here. Read [`provider-dispatch.md`](provider-dispatch.md) for the parent-owned Claude/Codex/Grok route table and provider-qualified descriptors.

## Tool actions

| pstack / Claude action | Antigravity equivalent |
|------------------------|------------------------|
| Read a file (`View` / `cat`) | `view_file` |
| Create a new file (`Write`) | `write_to_file` |
| Edit an existing file (`Edit` / `apply_patch`) | `replace_file_content` (or `write_to_file` with `Overwrite: true`) |
| Run a shell command (`Bash` / `shell`) | `run_command` |
| Search file contents (`Grep` / `rg`) | `grep_search` |
| Find files / list directories (`Glob`, `LS` / `find`, `ls`) | `find_by_name`, `list_dir` |
| Fetch a URL (`WebFetch` / `curl`) | `read_url_content` (or `read_browser_page` for interactive pages) |
| Search the web (`WebSearch`) | `search_web` |
| Generate an image / diagram | `generate_image` |
| Invoke a skill (the `Skill` tool, `/command`) | Skills load natively into the `<skills>` context. Follow instructions by reading `SKILL.md` via `view_file`. |
| Dispatch a subagent (the `Agent`/`Task` tool) | Native Antigravity subagent dispatch (`send_message`, background tasks) |
| Dispatch N parallel subagents in one turn | Concurrent subagent dispatch or background task invocations |
| Supervise / wait for a subagent or command | Reactive wakeups (incoming messages automatically wake the agent) or `manage_task` |
| Track tasks (the todolist / `TodoWrite`) | Markdown task list in context or scratchpad |
| Ask the human a fixed-choice question (`AskUserQuestion`) | `ask_question` tool with structured questions and options |
| Periodic pacing / heartbeat (`loop` skill) | `schedule` tool (one-shot timer or cron expression) |

## Subagent policy

poteto-mode's Subagents section sets Claude-specific defaults (`subagent_type: "poteto-agent"`, `run_in_background: true`). In Antigravity:

- There is no hardcoded `poteto-agent` subagent type. Route an ad-hoc subagent through poteto-mode's style by dispatching instructions that tell the subagent to read the `poteto-mode` skill (`skills/poteto-mode/SKILL.md`) in full first.
- Subagents run concurrently and communicate via `send_message`. When background commands run, notifications arrive reactively without polling.
- There is no `comment-sicko` subagent type either. The **no-comments** skill spawns it on Claude Code; in Antigravity, dispatch a subagent whose instructions tell it to read `agents/comment-sicko.md` in full first.
- Isolation: when workers write code, give each worker its own git worktree or branch. The **swarm** skill's workers and the fan-out playbooks (`orchestrate`, `autopilot-full`, `autopilot-stack`) isolate writers with worktrees.
- Keep the rest of the policy unchanged. Pass file pointers not inlined context, give each worker its own worktree or branch when they write, and review every subagent's diff yourself.

## Models and providers

Do not replace every configured entry with an Antigravity model. `/pstack:setup-pstack` writes portable descriptors such as `claude:fable@max`, `codex:gpt-5.6-sol@max`, and `grok:grok-4.6@xhigh`. In an Antigravity parent:
- External frontier models (`claude:*`, `codex:*`, `grok:*`) route through the external launcher `pstack-runner --parent antigravity` exactly as [`provider-dispatch.md`](provider-dispatch.md) specifies.
- `inherit-parent` and `auto` route natively to Antigravity subagents using the parent's current model.
- The current default panel intentionally keeps four-provider frontier diversity.

## Claude built-in skills pstack references

Some triggers name skills that ship with Claude Code, not pstack. They do not exist on Antigravity. Substitute the behavior:

| Claude built-in named in pstack | In Antigravity |
|---------------------------------|----------------|
| `run` (drive a CLI/TUI to see a change work) | Run the app yourself via `run_command` and observe the real output. |
| `verify` (drive a UI to confirm a fix) | Drive the UI with whatever automation you have (Playwright, Puppeteer, Chrome DevTools MCP), or hand the user a concrete verification check. Do not claim done without observing the artifact. |
| `plugin-dev:skill-development` (Claude's SKILL.md authoring guidance) | Follow standard skill-authoring guidance: YAML frontmatter with `name` and `description`, progressive disclosure, clear steps. |
| `loop` (recurring/self-paced re-invocation, used by `babysit` / `autonomous-run`) | Use the `schedule` tool (one-shot timer or recurring cron schedule) or reactive task notifications. |
| `AskUserQuestion` (structured question modal) | Use the native `ask_question` tool with structured options. |

## Vendored scripts

`skills/poteto-mode/scripts/` ships the `watch-pr` PR watcher, the `orch` store CLI, `worktree-audit.sh`, and `runner/pstack-runner`. They are plain bun and bash scripts, so they run in Antigravity via `run_command`. The external runner needs the assigned `claude`, `codex`, or `grok` executable authenticated on the host. The scripts need `bun`, `gh`, and (for `worktree-audit.sh`) `jq` and `rg`.

## Instructions file

Where a pstack skill says "your instructions file", in Antigravity that is `GEMINI.md` (project root, plus `~/.gemini/GEMINI.md` or App Data Directory `/Users/bforce/.gemini/antigravity-cli/` global rules/memories) or `AGENTS.md`. On Codex it is `AGENTS.md`. On Claude Code it is `CLAUDE.md`.
