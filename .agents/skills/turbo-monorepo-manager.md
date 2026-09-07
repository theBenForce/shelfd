# Turbo Monorepo Management Manual

This technical manual instructs the AI on managing the Turborepo workspace, task pipeline caching rules, and pnpm script execution for Shelfd.

## Invariants
1. Package manager is strictly `pnpm` (`packageManager: pnpm@9.x`).
2. Run tasks via `pnpm <script>` or `turbo run <task>`.
3. Server outputs are strictly cached to `apps/server/dist/shelfd`.
4. Flutter outputs are strictly cached to `apps/app/build/**`.

## Protocols

### 1. Adding a New Package or App
* Place packages in `apps/<name>` or `packages/<name>`.
* Ensure a valid `package.json` with standard lifecycle scripts:
  * `build`: Compile and output to dist/build.
  * `dev`: Start local watcher/dev server.
  * `test`: Execute automated tests.
  * `lint`: Run linter/vet checks.
  * `clean`: Remove build outputs.

### 2. Cache Verification Workflow
* Run `pnpm build` twice.
* The second run must report `>>> FULL TURBO` (100% cache hit rate).
* If a task misses cache unexpectedly:
  * Check `inputs` in `turbo.json`.
  * Ensure no dynamically generated files (logs, timestamps, temporary files) match input patterns.
