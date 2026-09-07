# Monorepo Orchestration — Turborepo with pnpm Workspaces

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team
* Date: 2026-09-07

## Context and Problem Statement

The `shelfd` project contains a Go backend daemon (`apps/server`), a Flutter cross-platform client (`apps/app`), and developer tooling. We need a unified developer workflow where common lifecycle tasks (build, dev, test, lint) can be executed from the root with intelligent caching across different languages and runtimes.

## Decision Drivers

* Fast, reproducible builds with output caching.
* Unified CLI commands across Go, Flutter, and potential web components.
* Clean package boundary isolation.
* Seamless integration with CI/CD pipelines.

## Considered Options

* **Turborepo with pnpm Workspaces**
* **Makefiles / Justfiles**
* **Bazel / Pants**
* **Multiple Separate Repositories**

## Decision Outcome

Chosen option: **Turborepo with pnpm Workspaces**, because Turborepo provides hash-based task caching that can wrap non-JS tools (Go compiler, Flutter CLI) via `package.json` scripts, while pnpm provides fast workspace linking.

### Positive Consequences

* Single root command (`pnpm build`, `pnpm test`, `pnpm lint`) orchestrates all projects.
* Go binaries built to `apps/server/dist/shelfd` are cached by Turborepo based on Go source file hashes (`**/*.go`, `go.mod`, `go.sum`). Unchanged builds replay in milliseconds (`>>> FULL TURBO`).
* Caching applies equally to Flutter web/desktop compilation targets (`build/**`).

### Negative Consequences

* Requires Node.js and pnpm installed on developer machines to run repo orchestration scripts.
* Building the Go server without Turborepo requires navigating to `apps/server` and invoking `go build` directly.

## Pros and Cons of the Options

### Turborepo + pnpm

* Good, because content-addressable build caching significantly speeds up local development and CI.
* Good, because task dependencies (`dependsOn`) are clearly declared in `turbo.json`.
* Good, because standardizes development commands across disparate languages.
* Bad, because introduces a Node.js tooling dependency to a Go/Flutter project.

### Makefiles / Justfiles

* Good, because no Node.js dependency required.
* Bad, because no smart content-hash caching; relies on simplistic mtime timestamp checks.
* Bad, because cross-platform differences between macOS, Linux, and Windows make Makefiles fragile.

### Separate Repositories

* Good, because completely separates Go from Flutter tooling.
* Bad, because synchronizing protocol schemas, MCP definitions, and release versions across repos creates maintenance overhead.
