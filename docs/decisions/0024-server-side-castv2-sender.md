# Server-Side CastV2 Sender for Audiobooks and Read-Along Books

* Status: accepted
* Deciders: @core, @reader, @security, @homelab
* Date: 2026-11-23

## Context and problem statement

Shelfd serves audiobooks and synchronized EPUB 3 read-along books over homelab LANs, often over plain HTTP or behind reverse proxies with self-signed TLS certificates. Listeners want to cast playback to Chromecast TVs and Google Nest speakers from Safari, Firefox, Chrome, and mobile clients while keeping read-along text highlights and playback progress in sync.

Google's browser Web Sender SDK (`chrome.cast`) works only in Chromium browsers and refuses to initialize on plain `http://192.168.x.x` origins. Out-of-process Shelfd plugins (`internal/plugins`) run as scale-to-zero Node.js stdio workers that are absent from `shelfd:minimal`, reap after 120 seconds of idle time, strip `SHELFD_JWT_SECRET`, and cannot write audiobook progress.

## Decision drivers

* Support every Shelfd client (Safari, Firefox, Chrome, iOS, Android, and plain HTTP LAN deployments) with one backend protocol implementation.
* Require zero Google Cast Developer Console registration or public HTTPS hosting in Phase 1.
* Keep audiobook progress synced in SQLite/PostgreSQL even when all browser tabs close.
* Avoid exposing full user session JWTs in Chromecast `MEDIA_STATUS` LAN broadcasts.
* Keep `apps/server/go.mod` free of unnecessary third-party dependencies.

## Considered options

* Option 1: Browser Web Sender SDK (`chrome.cast`) in Flutter Web.
* Option 2: Out-of-process TypeScript plugin under `plugins/chromecast`.
* Option 3: Custom Web Receiver (CAF v3) hosted over public HTTPS.
* Option 4: Pure-Go CastV2 sender in `apps/server/internal/cast` targeting Google's Default Media Receiver (`CC1AD845`) with scoped media JWTs, SSE status broadcasts, WebVTT TV captions, and a Flutter `CastAudioPlayerEngine`.

## Decision outcome

Chosen option: **Option 4 (Pure-Go CastV2 sender in `apps/server/internal/cast` targeting `CC1AD845`)**, because it works across every browser and mobile client on plain HTTP LANs, persists progress headlessly in the Go daemon, requires zero Google registration, and drives all existing Flutter read-along views (`ReaderView`, `FixedLayoutReader`, `ReadalongPipNotifier`) without modifying reader highlight code.

### Positive consequences

* Works from Safari, Firefox, Chrome, and mobile apps over plain LAN HTTP.
* Server-side `cast.Manager` owns SQLite/PostgreSQL progress writes during Cast sessions, preventing multi-tab write races.
* Book-scoped `cast:media:<bookID>` JWTs restrict LAN Chromecast URLs to read-only media, cover, and WebVTT routes for one book.
* Zero external Go dependencies added for CastV2 Protobuf framing or `_googlecast._tcp.local` mDNS discovery.

### Negative consequences

* Docker bridge deployments cannot receive LAN mDNS multicast packets without `network_mode: host` or explicit `SHELFD_CAST_STATIC_DEVICES` and `SHELFD_CAST_LAN_BASE_URL` configuration.
* On-TV visual read-along in Phase 1 uses WebVTT captions on `CC1AD845` rather than full EPUB page rendering on the TV (which can be added later via an optional Custom Web Receiver).
