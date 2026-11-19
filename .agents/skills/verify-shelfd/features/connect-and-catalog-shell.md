# Connect, Auth & Catalog Shell

Connect pairs the Shelf client with a self-hosted Shelfd server using manual credentials or QR discovery, lands on the persistent `AppShell` library catalog (`/books`), and lets the user browse Series, Authors, Genres, Topics, and Book Detail views without losing desktop navigation.

## Sub-features

- `connect-login` authenticates against `POST /api/v1/auth/login` from `/connect`, persists session state, and redirects to `/books`.
- `catalog-browse` displays book cards and collapsed series stacks on `/books`, and switches between `Series`, `Authors`, `Genres`, and `Topics` views inside `AppShell`.
- `book-detail` opens `/books/:bookId` showing metadata, reading progress, format badges, and `Listen Audiobook` / `Read EPUB` / `Start Reading` actions while keeping `AppShell` active.
- `disconnect-logout` signs out from Settings (`Disconnect / Sign Out`) and returns unauthenticated sessions to `/connect`.

## How to get to it (user POV)

- Open the web app unauthenticated (`/` or `/connect`), enter `Server URL`, `Username`, and `Password`, and choose `Connect & Sync Library`.
- On `/books`, choose filter pills (`All Books`, `Unread`, `Series`, `Authors`, `Genres`, `Topics`) or sidebar items (`Books`, `Series`, `Authors`, `Genres`, `Topics`) to switch catalog facets.
- Choose any book card (such as `The Tale of Peter Rabbit`) or series stack (`1 Book Dune Chronicles Series`) to inspect detail views.
- Open `Settings` and choose `Disconnect / Sign Out` to end the session.

## Driving it with control-shelfd

Preconditions:

- Isolated stack is running (`control-shelfd doctor` exits `0`).
- Browser is unauthenticated on `http://127.0.0.1:18080/connect`.

- **Fill server credentials.** Enter the isolated API URL, username, and password. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser fill --role textbox --name "Server URL" --value "http://127.0.0.1:18080"`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser fill --role textbox --name "Username" --value "admin"`, and `.agents/skills/verify-shelfd/scripts/control-shelfd browser fill --role textbox --name "Password" --value "shelfd-verify-pass-123"`. Each command reports `ok: true` on `/connect`.
- **Submit pairing form.** Choose `Connect & Sync Library`. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Connect & Sync Library" --wait-ms 2000`. The browser navigates to `http://127.0.0.1:18080/books` and renders `2 Books`, `1 Book Dune Chronicles Series`, and `The Tale of Peter Rabbit Beatrix Potter`.
- **Inspect Series & Book Detail inside AppShell.** Open the `Dune Chronicles` series card, then open `Dune`. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Dune Chronicles" --wait-ms 1200` and `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Dune" --wait-ms 1200`. The URL updates to `/books/<duneBookId>` with heading `Dune`, author link `Frank Herbert`, series pill `Dune Chronicles #1`, and `Start Reading`.
- **Capture proof.** Save ARIA, screenshot, and console audit. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser snapshot --path /tmp/shelfd-verify-evidence/connect-and-catalog-shell/book-detail.aria.txt`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser screenshot --path /tmp/shelfd-verify-evidence/connect-and-catalog-shell/book-detail.png`, and `.agents/skills/verify-shelfd/scripts/control-shelfd browser console --assert-clean --path /tmp/shelfd-verify-evidence/connect-and-catalog-shell/console.json`.

## Gotchas

- Books belonging to a series are grouped into a single series stack card (`1 Book Dune Chronicles Series`) on the default `All Books` view; click the series stack (or filter by title) to reach individual series installments.
- Filter pills (`All Books`, `Unread`, `Series`, `Authors`, `Genres`, `Topics`) on `/books` are Flutter `ChoiceChip` widgets and expose ARIA `role="checkbox"` (`--role checkbox --name "Series"`).
- Serving the SPA from `Shelfd`'s native `web_dir` on `http://127.0.0.1:18080` ensures `POST /api/v1/auth/login` sets the same-origin `HttpOnly` `shelfd_token` cookie required by fixed-layout EPUB `<iframe>` XHTML requests; if `SHELFD_VERIFY_WEB_URL` is overridden to a separate origin, explicitly fill `Server URL` with `http://127.0.0.1:18080`.
- Visiting `/connect` while already authenticated automatically redirects to `/books`.
