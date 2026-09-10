# Live Verification & Testing Workflow Manual

This technical manual instructs the `@qa` and `@reader` personas on conducting live, end-to-end stack verification for **Shelfd** using Go watch mode (`air`), Flutter Web development mode, and Chrome DevTools MCP automation.

---

## Invariants

1. **Independent Dev Servers (No Static Serving During UI Dev)**:
   * **Backend**: Run Go via `air` on `http://localhost:8080`.
   * **Frontend**: Run Flutter Web via `flutter run -d web-server --web-port 3000` from `apps/app/`.
   * **Rule**: Never serve Flutter Web statically from the Go backend (`dist/`) during active UI development; static serving loses hot reload and hot restart.
2. **CORS Header Compliance**:
   * The Go backend's `CORSMiddleware` (`apps/server/internal/api/middleware.go`) must permit:
     ```go
     w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Cache-Control")
     ```
   * Missing `Cache-Control` will break browser Server-Sent Events (SSE) queue streaming.
3. **Automated Visual Proof**:
   * Every UI change or routing update must be visually verified in Chrome via `chrome-devtools-mcp`.
   * Screenshots must be saved or copied to the conversation brain artifact directory (`<appDataDir>/brain/<conversation-id>/`) and embedded in `walkthrough.md`.
4. **Shell Navigation vs. Fullscreen Reader**:
   * Detail routes (`/books/:id`, `/series/:id`, `/authors/:id`) and catalog roots (`/books`, `/series`, `/authors`) must retain persistent desktop sidebar navigation (`AppShell`).
   * Reader routes (`/books/:id/read` and `/books/:id/read/:chapter`) must remain outside `AppShell` for fullscreen, distraction-free reading.

---

## Live Stack Execution Guide

### 1. Ensure Database Container is Active
Verify PostgreSQL is running on `127.0.0.1:5432`:
```bash
docker ps --filter "name=shelfd-postgres"
```
If not running:
```bash
docker start shelfd-postgres || docker compose up -d postgres
```

### 2. Launch Go Backend in Watch Mode
Run in `apps/server` as a background process or daemon:
```bash
cd apps/server
SHELFD_DATABASE_TYPE=postgres \
SHELFD_DATABASE_POSTGRES_DSN="postgres://shelfd:shelfd_password@127.0.0.1:5432/shelfd?sslmode=disable" \
SHELFD_STORAGE_LIBRARY_DIR=/Users/bforce/books \
SHELFD_STORAGE_DATA_DIR=/Users/bforce/repos/shelved/data \
SHELFD_SERVER_PORT=8080 \
SHELFD_JWT_SECRET=shelfd-homelab-jwt-secret-key-32bytes \
mise exec -- air
```
* `air` watches for Go file changes (`.go`, `.html`, etc.) and automatically recompiles and reloads the server within hundreds of milliseconds.

### 3. Launch Flutter Web Dev Server
Run in `apps/app` as a background process or daemon:
```bash
cd apps/app
mise exec -- flutter run -d web-server --web-port 3000
```
* Binds to `http://localhost:3000`.
* Allows triggering hot reload (`r`) or hot restart (`R`) when modifying Dart files.

---

## Chrome DevTools MCP Verification Workflow

### 1. Locate Browser Page
```json
{
  "ServerName": "chrome-devtools-mcp",
  "ToolName": "list_pages",
  "Arguments": {}
}
```
Identify the `pageId` corresponding to `http://localhost:3000`.

### 2. Seed LocalStorage Authentication (If Needed)
To bypass the login screen and authenticate directly against local development backend:
```json
{
  "ServerName": "chrome-devtools-mcp",
  "ToolName": "evaluate_script",
  "Arguments": {
    "pageId": 1,
    "script": "localStorage.setItem('flutter.shelfd_server_url', JSON.stringify('http://localhost:8080')); localStorage.setItem('flutter.shelfd_saved_username', JSON.stringify('admin')); localStorage.setItem('flutter.shelfd_auth_token', JSON.stringify('<JWT_TOKEN>'));"
  }
}
```

### 3. Navigate to Target Views
```json
{
  "ServerName": "chrome-devtools-mcp",
  "ToolName": "navigate_page",
  "Arguments": {
    "pageId": 1,
    "type": "url",
    "url": "http://localhost:3000/books/573be0ee-ab96-4849-89e8-a7596968c3df"
  }
}
```

### 4. Audit Errors & DOM Tree
* Inspect console for unhandled exceptions or CORS preflight failures:
  ```json
  {
    "ServerName": "chrome-devtools-mcp",
    "ToolName": "list_console_messages",
    "Arguments": { "pageId": 1 }
  }
  ```
* Verify accessibility tree and rendered widget labels:
  ```json
  {
    "ServerName": "chrome-devtools-mcp",
    "ToolName": "take_snapshot",
    "Arguments": { "pageId": 1 }
  }
  ```

### 5. Capture & Embed Screenshots
1. Take screenshot without `filePath` parameter (or save to workspace):
   ```json
   {
     "ServerName": "chrome-devtools-mcp",
     "ToolName": "take_screenshot",
     "Arguments": { "pageId": 1 }
   }
   ```
2. The MCP server offloads the image to `.system_generated/steps/<step>/media_0.png`.
3. Copy the image into the brain artifact directory:
   ```bash
   cp <appDataDir>/brain/<conversation-id>/.system_generated/steps/<step>/media_0.png <appDataDir>/brain/<conversation-id>/screenshot_<name>.png
   ```
4. Embed in `walkthrough.md`:
   ```markdown
   ![Caption](<appDataDir>/brain/<conversation-id>/screenshot_<name>.png)
   ```
