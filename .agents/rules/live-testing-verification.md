# Live Testing & Verification Standards

## Invariants

1. **Independent Development Servers (Hot Reload & Watch Mode)**:
   - Always run the Go server in watch mode using `air` (`mise exec -- air`) in `apps/server` on port 8080.
   - Always run the Flutter Web application independently via `flutter run -d web-server --web-port 3000` (`mise exec -- flutter run -d web-server --web-port 3000`) in `apps/app`.
   - **NEVER serve Flutter Web statically from the Go backend during development.** Static serving loses hot reload and hot restart capabilities and leads to stale cache confusion.

2. **CORS Headers Invariant**:
   - Ensure `Access-Control-Allow-Headers` in `apps/server/internal/api/middleware.go` allows:
     `Authorization, Content-Type, Accept, Cache-Control`
   - Failure to include `Cache-Control` will break browser Server-Sent Events (SSE) queue connections.

3. **UI Screenshots via Flutter Golden/Screenshot Tests (No Backend Required)**:
   - When creating screenshots for UI changes, write Flutter widget/golden tests under `apps/app/test/` using mock repositories and Riverpod provider overrides (no need to run the Go backend or database).
   - Prefer a Flutter screenshot/golden library such as `golden_screenshot` (with `tester.loadAssets()` for real app fonts and `matchesGoldenFile(...)`) or `alchemist`, generating PNGs via `flutter test --update-goldens`.
   - Reserve live browser verification (`chrome-devtools-mcp` or `control-shelfd`) for full-stack checks that actually require the Go server (CORS/SSE, fixed-layout EPUB iframes, or end-to-end API flows).

4. **Shell Navigation vs. Fullscreen Reader Invariant**:
   - Persistent `AppShell` (`ShelfdSideNav` on desktop) MUST wrap `/books`, `/series`, `/authors`, `/books/:id`, `/series/:id`, and `/authors/:id`.
   - Reader routes (`/books/:id/read` and `/books/:id/read/:chapterIdentifier`) MUST remain outside `AppShell` for distraction-free fullscreen reading.
