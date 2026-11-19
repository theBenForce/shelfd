# Audiobook & Read-Along Playback

Shelfd streams companion audiobooks (`/books/:bookId/listen`) with chapter seeking, speed/sleep controls, a persistent `MiniPlayerBar` across catalog routes that preserves playback position when reopened, fixed-layout EPUB 3 SMIL media-overlay rendering (`/books/:bookId/read`), and live mode toggling between Audiobook and Read-Along views.

## Sub-features

- `audiobook-open` opens `/books/:bookId/listen` (`PLAYING AUDIOBOOK`), loads chapters and stream metadata, and seeks/plays without CORS or media errors.
- `mini-player-preserve-position` minimizes `/books/:bookId/listen` to the docked `MiniPlayerBar` and reopens the full player without resetting playback position to `00:00`.
- `mode-toggle-readalong` switches between `Audiobook` (`/books/:bookId/listen`) and `Read-Along` (`/books/:bookId/read`) from the full player, reader top bar (`Switch to Audiobook Player`), and `MiniPlayerBar` (`Switch to Read-Along` / `Switch to Audiobook`) while preserving audio state.
- `fixed-layout-iframe` renders pre-paginated XHTML pages inside normalized `<iframe>` elements (`bodyChildren > 0`, `nestedAbsoluteSpans === 0`) synchronized to SMIL segments.

## How to get to it (user POV)

- Open a dual-format or audiobook item (such as `The Tale of Peter Rabbit`) from `/books` and choose `Listen Audiobook` (or `Read EPUB`).
- Inside `/books/:bookId/listen`, choose `Read-Along` in the mode pill to switch to synchronized page reading, or choose `Minimize Player` to dock playback into `MiniPlayerBar`.
- Inside `/books/:bookId/read` on a read-along book, choose `Switch to Audiobook Player` in the top bar to return to the full audiobook player.
- On `MiniPlayerBar`, tap the cover/title area to reopen the active playback mode view, or tap `Switch to Read-Along` / `Switch to Audiobook` to jump directly to the opposite mode.

## Driving it with control-shelfd

Preconditions:

- Authenticated browser session on `/books`.
- `The Tale of Peter Rabbit` (`hasReadalong: true`) is seeded in the isolated library.

- **Open Book Detail & launch Audiobook Player.** Choose `The Tale of Peter Rabbit`, then `Listen Audiobook`. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser goto "/books"`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "The Tale of Peter Rabbit" --wait-ms 1200`, and `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Listen Audiobook" --wait-ms 1800`. ARIA shows `heading "PLAYING AUDIOBOOK The Tale of Peter Rabbit"` and `button "Read-Along"`.
- **Advance position & verify MiniPlayerBar reopen preserves position.** Skip forward 30 seconds, minimize the player, tap the mini-player title group (`--role group --name "The Tale of Peter Rabbit"`) to reopen the full player and confirm elapsed time remains non-zero (`00:09` rather than `00:00`), then minimize again and click `Switch to Read-Along`. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Skip forward 30 seconds" --wait-ms 800`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Minimize Player" --wait-ms 1000`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role group --name "The Tale of Peter Rabbit" --wait-ms 1200`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser snapshot --path /tmp/shelfd-verify-evidence/audiobook-and-readalong-playback/reopened-from-miniplayer.aria.txt`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Minimize Player" --wait-ms 1000`, and `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Switch to Read-Along" --wait-ms 1800`.
- **Verify Fixed-Layout Read-Along iframe & toggle back to Audiobook.** On the Read-Along view, assert XHTML iframe normalization over CDP, capture visual proof, then switch back to the Audiobook Player via `Switch to Audiobook Player`. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser iframes --assert-normalized --path /tmp/shelfd-verify-evidence/audiobook-and-readalong-playback/iframes.json`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser screenshot --path /tmp/shelfd-verify-evidence/audiobook-and-readalong-playback/readalong.png`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Switch to Audiobook Player" --wait-ms 1200`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser snapshot --path /tmp/shelfd-verify-evidence/audiobook-and-readalong-playback/audiobook.aria.txt`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser screenshot --path /tmp/shelfd-verify-evidence/audiobook-and-readalong-playback/audiobook.png`, and `.agents/skills/verify-shelfd/scripts/control-shelfd browser console --assert-clean --path /tmp/shelfd-verify-evidence/audiobook-and-readalong-playback/console.json`.

## Gotchas

- Headless Linux containers without ALSA/PulseAudio hardware must pass `--mute-audio` and `--autoplay-policy=no-user-gesture-required` to Chromium (`control-shelfd launch` configures both automatically) or HTML5 `<audio>` initialization can crash the renderer.
- Outer ARIA snapshots and same-origin DOM scripts cannot traverse cross-origin fixed-layout `<iframe>` documents (`:13000` embedding `:18080`); `control-shelfd browser iframes --assert-normalized` inspects child OOPIF targets and isolated worlds directly over CDP to prove `bodyChildren > 0` and `nestedAbsoluteSpans === 0`.
- In `MiniPlayerBar`, the cover/title tap target is exposed in Flutter semantics as `role="group"` (`--role group --name "The Tale of Peter Rabbit"`), while `Switch to Read-Along`, `Switch to Audiobook`, and `Close player` are `role="button"`.
