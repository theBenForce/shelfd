# Turborepo Monorepo Standards

## Invariants

1. **Package Manager:**
   - Always use `pnpm` (`packageManager: pnpm@9.x`). Do not use npm or yarn.
   - Run commands via `pnpm <script>` or `turbo run <task>`.

2. **Package Naming:**
   - Applications live under `apps/*` (e.g. `@shelfd/server`, `@shelfd/app`).
   - Shared packages live under `packages/*`.

3. **Turbo Task Inputs & Outputs:**
   - When introducing new Go files, assets, or configs, ensure `inputs` in `turbo.json` covers them.
   - Output binary for the Go server is strictly `dist/shelfd`.
   - Output build artifacts for Flutter are strictly `build/**`.
   - The Go build command must be fast and cached: `go build -o dist/shelfd ./cmd/server`.

4. **Lifecycle Uniformity:**
   - Every workspace package must implement:
     - `build`
     - `test`
     - `lint`
     - `clean`
   - Never run sub-project build scripts directly by bypassing `turbo` or `pnpm --filter`.
