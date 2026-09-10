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

3. **Chrome DevTools MCP Validation**:
   - For all frontend UI and routing updates, live verification MUST be performed using `chrome-devtools-mcp` against `http://localhost:3000`.
   - Always inspect `list_console_messages` to ensure zero runtime or CORS errors.
   - Always verify DOM / a11y trees via `take_snapshot`.
   - Always capture visual proof using `take_screenshot` and embed screenshots in `walkthrough.md`.

4. **Shell Navigation vs. Fullscreen Reader Invariant**:
   - Persistent `AppShell` (`ShelfdSideNav` on desktop) MUST wrap `/books`, `/series`, `/authors`, `/books/:id`, `/series/:id`, and `/authors/:id`.
   - Reader routes (`/books/:id/read` and `/books/:id/read/:chapterIdentifier`) MUST remain outside `AppShell` for distraction-free fullscreen reading.
