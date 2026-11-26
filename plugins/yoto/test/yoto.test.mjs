import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import readline from "node:readline";
import test from "node:test";
import { fileURLToPath } from "node:url";
import zlib from "node:zlib";
import { normalizeIcon16x16, normalizeYotoIconUri } from "../src/image-16x16.ts";

const pluginRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function makeTestPNG(width, height, fillFn) {
  const sig = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8;
  ihdr[9] = 6;
  ihdr[10] = 0;
  ihdr[11] = 0;
  ihdr[12] = 0;

  const stride = width * 4;
  const raw = Buffer.alloc((1 + stride) * height);
  for (let y = 0; y < height; y++) {
    const rowStart = y * (1 + stride);
    raw[rowStart] = 0;
    for (let x = 0; x < width; x++) {
      const [r, g, b, a] = fillFn(x, y);
      const px = rowStart + 1 + x * 4;
      raw[px] = r;
      raw[px + 1] = g;
      raw[px + 2] = b;
      raw[px + 3] = a;
    }
  }

  const crcTable = new Uint32Array(256);
  for (let i = 0; i < 256; i++) {
    let c = i;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    crcTable[i] = c >>> 0;
  }
  const chunk = (type, data) => {
    const tBuf = Buffer.from(type, "ascii");
    const out = Buffer.alloc(12 + data.length);
    out.writeUInt32BE(data.length, 0);
    tBuf.copy(out, 4);
    data.copy(out, 8);
    let crc = 0xffffffff;
    for (const b of tBuf) crc = crcTable[(crc ^ b) & 0xff] ^ (crc >>> 8);
    for (const b of data) crc = crcTable[(crc ^ b) & 0xff] ^ (crc >>> 8);
    out.writeUInt32BE((crc ^ 0xffffffff) >>> 0, 8 + data.length);
    return out;
  };

  return new Uint8Array(
    Buffer.concat([
      sig,
      chunk("IHDR", ihdr),
      chunk("IDAT", zlib.deflateSync(raw)),
      chunk("IEND", Buffer.alloc(0)),
    ])
  );
}

test("yoto bundle is a standalone single-file ESM artifact", () => {
  const distPath = path.join(pluginRoot, "dist", "index.mjs");
  assert.ok(fs.existsSync(distPath), "dist/index.mjs must exist after build");
  const content = fs.readFileSync(distPath, "utf8");
  assert.doesNotMatch(
    content,
    /from\s+["']@thebenforce\/shelfd-plugin-sdk["']/,
    "dist/index.mjs must inline @thebenforce/shelfd-plugin-sdk so no node_modules is needed at runtime"
  );
});

test("TestNormalizeIcon16x16: 16x16 PNG passthrough, 64x64 PNG downscale to 16x16, invalid bytes error, and Yoto URI normalization", () => {
  const png16 = makeTestPNG(16, 16, (x, y) => [x * 16, y * 16, 128, 255]);
  const out16 = normalizeIcon16x16(png16);
  const view16 = new DataView(out16.buffer, out16.byteOffset, out16.byteLength);
  assert.equal(view16.getUint32(16, false), 16);
  assert.equal(view16.getUint32(20, false), 16);

  const png64 = makeTestPNG(64, 64, (x, y) => [x < 32 ? 255 : 0, y < 32 ? 255 : 0, 64, 255]);
  const outFrom64 = normalizeIcon16x16(png64);
  const view64 = new DataView(outFrom64.buffer, outFrom64.byteOffset, outFrom64.byteLength);
  assert.equal(view64.getUint32(16, false), 16, "64x64 PNG must normalize to 16px width");
  assert.equal(view64.getUint32(20, false), 16, "64x64 PNG must normalize to 16px height");

  assert.throws(() => normalizeIcon16x16(new Uint8Array([0x00, 0x01, 0x02, 0x03, 0x04])), /Unsupported image format/);
  assert.throws(() => normalizeIcon16x16(new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00])), /Invalid PNG/);

  const validMediaId = "AC6ySygbay79fIMHmMtYN9ekBo1kwMvRoi4DQW_CZW0";
  const expectedUri = `yoto:#${validMediaId}`;
  assert.equal(normalizeYotoIconUri(`yoto:#${validMediaId}`), expectedUri);
  assert.equal(normalizeYotoIconUri(`yoto:${validMediaId}`), expectedUri);
  assert.equal(normalizeYotoIconUri(validMediaId), expectedUri);
  assert.equal(normalizeYotoIconUri(`https://media-secure-sbc.yotoplay.com/icons/${validMediaId}`), expectedUri);
  assert.equal(normalizeYotoIconUri(`https://card-content.yotoplay.com/yoto/${validMediaId}.png?token=abc`), expectedUri);
  assert.equal(normalizeYotoIconUri(""), "");
  assert.equal(normalizeYotoIconUri("yoto:#short-id"), "");
  assert.equal(normalizeYotoIconUri("https://example.com/icons/book.png"), "");
});

test("yoto plugin RPC end-to-end: PKCE auth, token refresh, options, 16x16 icon upload, and create/append/replace card upload", async () => {
  const validIconId = "AC6ySygbay79fIMHmMtYN9ekBo1kwMvRoi4DQW_CZW0";
  const uploadedIconId = "UP6ySygbay79fIMHmMtYN9ekBo1kwMvRoi4DQW_CZW0";
  let s3UploadedBytes = 0;
  let lastSavedCards = [];
  let transcodePollCount = 0;
  let uploadedIconBytes = null;

  const existingCardStore = {
    cardId: "card-existing-1",
    title: "Bedtime Favorites",
    userId: "user-yoto-1",
    content: {
      chapters: [
        {
          key: "01",
          title: "Existing Story 1",
          overlayLabel: "1",
          duration: 60,
          fileSize: 1000,
          display: { icon16x16: `yoto:#${validIconId}` },
          tracks: [
            {
              key: "01",
              title: "Existing Story 1",
              trackUrl: "yoto:#sha-existing-1",
              duration: 60,
              fileSize: 1000,
              type: "audio",
              channels: 2,
              display: null,
            },
          ],
        },
      ],
    },
    metadata: {
      description: "Existing playlist",
      media: { duration: 60, fileSize: 1000 },
    },
  };

  const mockServer = http.createServer((req, res) => {
    const url = new URL(req.url ?? "/", `http://${req.headers.host}`);

    if (req.method === "POST" && url.pathname === "/oauth/token") {
      let body = "";
      req.on("data", (c) => (body += c));
      req.on("end", () => {
        const params = new URLSearchParams(body);
        const grantType = params.get("grant_type");
        if (grantType === "authorization_code") {
          if (params.get("code") !== "pkce-code-1" || params.get("code_verifier") !== "verifier-1") {
            res.writeHead(400, { "content-type": "application/json" });
            res.end(JSON.stringify({ error: "invalid_grant" }));
            return;
          }
          res.writeHead(200, { "content-type": "application/json" });
          res.end(
            JSON.stringify({
              access_token: "acc-pkce-1",
              refresh_token: "ref-pkce-1",
              token_type: "Bearer",
              expires_in: 3600,
            })
          );
          return;
        }
        if (grantType === "refresh_token") {
          res.writeHead(200, { "content-type": "application/json" });
          res.end(
            JSON.stringify({
              access_token: "acc-refreshed-2",
              refresh_token: "ref-rotated-2",
              token_type: "Bearer",
              expires_in: 3600,
            })
          );
          return;
        }
        res.writeHead(400);
        res.end("unsupported grant_type");
      });
      return;
    }

    if (req.method === "GET" && url.pathname === "/content/mine") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          cards: [{ cardId: "card-existing-1", title: "Bedtime Favorites" }],
        })
      );
      return;
    }

    if (req.method === "GET" && url.pathname === "/card/card-existing-1") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(JSON.stringify({ card: existingCardStore }));
      return;
    }

    if (req.method === "GET" && url.pathname === "/content/card-existing-1") {
      res.writeHead(403, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          error: {
            code: "forbidden",
            message: "User does not have required scope(s): 'user:content:view', 'family:library:view'",
          },
        })
      );
      return;
    }

    if (req.method === "GET" && url.pathname === "/media/displayIcons/user/yoto") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          displayIcons: [
            {
              displayIconId: "pub-1",
              mediaId: validIconId,
              title: "Rabbit Book",
              public: true,
              publicTags: ["rabbit", "book"],
              url: `https://cdn.yotoplay.com/icons/${validIconId}.png`,
            },
          ],
        })
      );
      return;
    }

    if (req.method === "POST" && url.pathname === "/media/displayIcons/user/me/upload") {
      const chunks = [];
      req.on("data", (c) => chunks.push(c));
      req.on("end", () => {
        uploadedIconBytes = Buffer.concat(chunks);
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            displayIcon: {
              displayIconId: "cust-1",
              mediaId: uploadedIconId,
              url: `https://cdn.yotoplay.com/icons/${uploadedIconId}.png`,
            },
          })
        );
      });
      return;
    }

    if (req.method === "POST" && url.pathname === "/media/coverImage/user/me/upload") {
      req.resume();
      req.on("end", () => {
        res.writeHead(200, { "content-type": "application/json" });
        res.end(JSON.stringify({ coverImage: { mediaUrl: "https://cdn.yotoplay.com/covers/cov-1.jpg" } }));
      });
      return;
    }

    if (req.method === "GET" && url.pathname === "/media/transcode/audio/uploadUrl") {
      const addr = mockServer.address();
      const base = `http://127.0.0.1:${addr.port}`;
      res.writeHead(200, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          upload: {
            uploadUrl: `${base}/s3-put-audio`,
            uploadId: "up-test-1",
          },
        })
      );
      return;
    }

    if (req.method === "PUT" && url.pathname === "/s3-put-audio") {
      const chunks = [];
      req.on("data", (c) => chunks.push(c));
      req.on("end", () => {
        s3UploadedBytes += Buffer.concat(chunks).length;
        res.writeHead(200);
        res.end();
      });
      return;
    }

    if (req.method === "GET" && url.pathname === "/media/upload/up-test-1/transcoded") {
      transcodePollCount += 1;
      res.writeHead(200, { "content-type": "application/json" });
      if (transcodePollCount === 1) {
        res.end(JSON.stringify({ transcode: {} }));
        return;
      }
      res.end(
        JSON.stringify({
          transcode: {
            transcodedSha256: "sha256-peter-rabbit",
            transcodedInfo: {
              duration: 142.5,
              fileSize: 4096,
              channels: "stereo",
              format: "aac",
            },
          },
        })
      );
      return;
    }

    if (req.method === "POST" && url.pathname === "/content") {
      let body = "";
      req.on("data", (c) => (body += c));
      req.on("end", () => {
        const parsed = JSON.parse(body);
        lastSavedCards.push(parsed);
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            card: {
              cardId: parsed.cardId || "card-new-99",
              title: parsed.title,
              content: parsed.content,
              metadata: parsed.metadata,
            },
          })
        );
      });
      return;
    }

    // Loopback Shelfd endpoints
    if (req.method === "GET" && url.pathname === "/api/v1/books/book-peter") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          id: "book-peter",
          title: "The Tale of Peter Rabbit",
          description: "A mischievous rabbit in Mr. McGregor's garden.",
          book_type: "audiobook",
          cover_path: "covers/peter.png",
          files: [
            {
              id: "file-peter-1",
              book_id: "book-peter",
              file_type: "audiobook",
              file_path: "Beatrix Potter/The Tale of Peter Rabbit/peter.mp3",
              mime_type: "audio/mpeg",
              file_size_bytes: 32,
              duration_seconds: 142.5,
            },
          ],
          audio_chapters: [
            { chapter_index: 1, title: "Into the Garden", duration_sec: 70 },
            { chapter_index: 2, title: "Escape Home", duration_sec: 72.5 },
          ],
        })
      );
      return;
    }

    if (req.method === "GET" && url.pathname === "/api/v1/books/book-peter/files/file-peter-1/stream") {
      const buf = Buffer.from("ID3-mock-mp3-audio-stream-bytes!");
      res.writeHead(200, {
        "content-type": "audio/mpeg",
        "content-length": String(buf.length),
      });
      res.end(buf);
      return;
    }

    if (req.method === "GET" && url.pathname === "/api/v1/books/book-peter/cover") {
      const cov = makeTestPNG(32, 32, () => [100, 150, 200, 255]);
      res.writeHead(200, { "content-type": "image/png" });
      res.end(Buffer.from(cov));
      return;
    }

    res.writeHead(404);
    res.end("not found");
  });

  await new Promise((r) => mockServer.listen(0, "127.0.0.1", r));
  const port = mockServer.address().port;
  const baseUrl = `http://127.0.0.1:${port}`;

  const distPath = path.join(pluginRoot, "dist", "index.mjs");
  const child = spawn("node", [distPath], {
    cwd: pluginRoot,
    stdio: ["pipe", "pipe", "pipe"],
    env: {
      ...process.env,
      NO_PROXY: "127.0.0.1,localhost",
      no_proxy: "127.0.0.1,localhost",
    },
  });

  const pending = new Map();
  const configPatches = [];
  const progressEvents = [];
  let seq = 0;

  const rl = readline.createInterface({ input: child.stdout, crlfDelay: Infinity });
  rl.on("line", (line) => {
    if (!line.trim()) return;
    const msg = JSON.parse(line);
    if (msg.id !== undefined && !msg.method) {
      const waiter = pending.get(String(msg.id));
      if (waiter) {
        pending.delete(String(msg.id));
        waiter(msg);
      }
    } else if (msg.method === "config/patch") {
      configPatches.push(msg.params.values);
      child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id: msg.id, result: { ok: true } })}\n`);
    } else if (msg.method === "progress") {
      progressEvents.push(msg.params);
    }
  });

  const callRpc = (method, params) => {
    seq += 1;
    const id = String(seq);
    return new Promise((resolve) => {
      pending.set(id, resolve);
      child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
    });
  };

  try {
    await callRpc("initialize", {
      protocol_version: "2026-11-01",
      shelfd_version: "0.1.0-test",
      api_base: baseUrl,
    });

    const baseConfig = {
      client_id: "yoto-test-client",
      api_base_url: baseUrl,
      auth_base_url: baseUrl,
      poll_interval_ms: 10,
    };

    // 1. Auth begin + PKCE complete
    const beginResp = await callRpc("auth/begin", {
      method: "oauth_pkce",
      code_challenge: "chal-123",
      state: "st-1",
      redirect_uri: "http://localhost:8080/settings",
      auth: { user_id: "u1", api_base: baseUrl, invocation_token: "tok-loop", config: baseConfig },
    });
    assert.ok(beginResp.result.authorize_url.includes("code_challenge=chal-123"));
    const authUrl = new URL(beginResp.result.authorize_url);
    assert.equal(authUrl.searchParams.get("scope"), "user:content:manage user:content:view user:icons:manage offline_access");

    const completeResp = await callRpc("auth/complete", {
      method: "oauth_pkce",
      code: "pkce-code-1",
      code_verifier: "verifier-1",
      redirect_uri: "http://localhost:8080/settings",
      auth: { user_id: "u1", api_base: baseUrl, invocation_token: "tok-loop", config: baseConfig },
    });
    assert.equal(completeResp.result.linked, true);
    assert.equal(configPatches.at(-1).access_token, "acc-pkce-1");

    // 2. Expired token auto-refresh during options query
    const expiredConfig = {
      ...baseConfig,
      access_token: "acc-expired",
      refresh_token: "ref-pkce-1",
      token_expires_at: new Date(Date.now() - 60_000).toISOString(),
    };
    const plOptResp = await callRpc("actions/options", {
      action: "send_to_yoto",
      source: "playlists",
      auth: { user_id: "u1", api_base: baseUrl, invocation_token: "tok-loop", config: expiredConfig },
    });
    assert.equal(plOptResp.result.items.length, 1);
    assert.equal(plOptResp.result.items[0].value, "card-existing-1");
    assert.equal(plOptResp.result.items[0].subtitle, "1 chapters");
    assert.equal(configPatches.at(-1).access_token, "acc-refreshed-2");

    const activeConfig = {
      ...baseConfig,
      access_token: "acc-refreshed-2",
      refresh_token: "ref-rotated-2",
      token_expires_at: new Date(Date.now() + 3600_000).toISOString(),
    };

    // 3. Public icon search + custom 64x64 asset upload normalized to 16x16 PNG
    const iconOptResp = await callRpc("actions/options", {
      action: "send_to_yoto",
      source: "icons",
      tab: "public",
      query: "rabbit",
      auth: { user_id: "u1", api_base: baseUrl, invocation_token: "tok-loop", config: activeConfig },
    });
    assert.equal(iconOptResp.result.items.length, 1);
    assert.equal(iconOptResp.result.items[0].icon_uri, `yoto:#${validIconId}`);

    const raw64Png = makeTestPNG(64, 64, () => [255, 100, 50, 255]);
    const uploadAssetResp = await callRpc("actions/upload_asset", {
      action: "send_to_yoto",
      filename: "custom64.png",
      mime_type: "image/png",
      data_base64: Buffer.from(raw64Png).toString("base64"),
      auth: { user_id: "u1", api_base: baseUrl, invocation_token: "tok-loop", config: activeConfig },
    });
    assert.equal(uploadAssetResp.result.item.icon_uri, `yoto:#${uploadedIconId}`);
    assert.ok(uploadedIconBytes, "mock server should receive uploaded icon bytes");
    assert.equal(uploadedIconBytes.readUInt32BE(16), 16, "uploaded icon must be normalized to 16px width");
    assert.equal(uploadedIconBytes.readUInt32BE(20), 16, "uploaded icon must be normalized to 16px height");

    // 4. Prepare action for book-peter
    const prepResp = await callRpc("actions/prepare", {
      action: "send_to_yoto",
      target: { type: "book", id: "book-peter" },
      auth: { user_id: "u1", api_base: baseUrl, invocation_token: "tok-loop", config: activeConfig },
    });
    assert.equal(prepResp.result.auth_required, false);
    assert.equal(prepResp.result.entries.length, 2);
    assert.equal(prepResp.result.entries[0].title, "Into the Garden");

    // 5. Invoke create, append, and replace
    const createResp = await callRpc("actions/invoke", {
      action: "send_to_yoto",
      job_id: "job-1",
      target: { type: "book", id: "book-peter" },
      values: {
        playlist_mode: "create",
        playlist_title: "Peter Rabbit MYO",
        include_cover: true,
        default_icon: `yoto:#${validIconId}`,
      },
      overrides: {
        "1": `yoto:#${uploadedIconId}`,
      },
      auth: { user_id: "u1", api_base: baseUrl, invocation_token: "tok-loop", config: activeConfig },
    });
    assert.equal(createResp.result.status, "completed");
    assert.equal(createResp.result.data.card_id, "card-new-99");
    assert.ok(s3UploadedBytes > 0, "audio stream must be uploaded to S3 PUT URL");
    assert.ok(progressEvents.length >= 2, "must emit progress events during upload and transcode");
    const createdCard = lastSavedCards.at(-1);
    assert.equal(createdCard.content.chapters.length, 2);
    assert.equal(createdCard.content.chapters[0].display.icon16x16, `yoto:#${validIconId}`);
    assert.equal(createdCard.content.chapters[1].display.icon16x16, `yoto:#${uploadedIconId}`);

    const appendResp = await callRpc("actions/invoke", {
      action: "send_to_yoto",
      job_id: "job-2",
      target: { type: "book", id: "book-peter" },
      values: {
        playlist_mode: "append",
        card_id: "card-existing-1",
      },
      auth: { user_id: "u1", api_base: baseUrl, invocation_token: "tok-loop", config: activeConfig },
    });
    assert.equal(appendResp.result.status, "completed");
    const appendedCard = lastSavedCards.at(-1);
    assert.equal(appendedCard.content.chapters.length, 3, "append must preserve existing 1 chapter + add 2 new chapters");
    assert.equal(appendedCard.content.chapters[0].title, "Existing Story 1");
    assert.equal(appendedCard.content.chapters[1].key, "02");

    const replaceResp = await callRpc("actions/invoke", {
      action: "send_to_yoto",
      job_id: "job-3",
      target: { type: "book", id: "book-peter" },
      values: {
        playlist_mode: "replace",
        card_id: "card-existing-1",
      },
      auth: { user_id: "u1", api_base: baseUrl, invocation_token: "tok-loop", config: activeConfig },
    });
    assert.equal(replaceResp.result.status, "completed");
    const replacedCard = lastSavedCards.at(-1);
    assert.equal(replacedCard.content.chapters.length, 2, "replace must overwrite chapters on existing card");

    await callRpc("shutdown", {});
  } finally {
    rl.close();
    child.kill("SIGKILL");
    mockServer.closeAllConnections?.();
    mockServer.close();
  }
});

function readProcVmRssMB(pid) {
  try {
    const status = fs.readFileSync(`/proc/${pid}/status`, "utf8");
    const m = status.match(/^VmRSS:\s+(\d+)\s+kB/m);
    if (m) {
      return Number(m[1]) / 1024;
    }
  } catch {
    void 0;
  }
  return 0;
}

test("yoto plugin streams 5x 64 MB audiobook uploads with child Node VmRSS under 96 MB", async () => {
  const STREAM_BYTES = 64 * 1024 * 1024;
  const CHUNK = Buffer.alloc(64 * 1024, 0x5a);
  let totalPutBytes = 0;

  const mockServer = http.createServer((req, res) => {
    const url = new URL(req.url ?? "/", `http://${req.headers.host}`);

    if (req.method === "GET" && url.pathname === "/api/v1/books/book-64mb") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          id: "book-64mb",
          title: "Large 64MB Audiobook",
          book_type: "audiobook",
          files: [
            {
              id: "file-64mb",
              book_id: "book-64mb",
              file_type: "audiobook",
              file_path: "Author/Large/large.m4b",
              mime_type: "audio/mp4",
              file_size_bytes: STREAM_BYTES,
              duration_seconds: 3600,
            },
          ],
          audio_chapters: [{ chapter_index: 1, title: "Part 1", duration_sec: 3600 }],
        })
      );
      return;
    }

    if (req.method === "GET" && url.pathname === "/api/v1/books/book-64mb/files/file-64mb/stream") {
      res.writeHead(200, {
        "content-type": "audio/mp4",
        "content-length": String(STREAM_BYTES),
      });
      let sent = 0;
      const pump = () => {
        while (sent < STREAM_BYTES) {
          const rem = Math.min(CHUNK.length, STREAM_BYTES - sent);
          const slice = rem === CHUNK.length ? CHUNK : CHUNK.subarray(0, rem);
          sent += rem;
          if (!res.write(slice)) {
            res.once("drain", pump);
            return;
          }
        }
        res.end();
      };
      pump();
      return;
    }

    if (req.method === "GET" && url.pathname === "/media/transcode/audio/uploadUrl") {
      const addr = mockServer.address();
      res.writeHead(200, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          upload: {
            uploadUrl: `http://127.0.0.1:${addr.port}/s3-put-64mb`,
            uploadId: "up-64mb",
          },
        })
      );
      return;
    }

    if (req.method === "PUT" && url.pathname === "/s3-put-64mb") {
      req.on("data", (c) => {
        totalPutBytes += c.length;
      });
      req.on("end", () => {
        res.writeHead(200);
        res.end();
      });
      return;
    }

    if (req.method === "GET" && url.pathname === "/media/upload/up-64mb/transcoded") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          transcode: {
            transcodedSha256: "sha256-64mb",
            transcodedInfo: { duration: 3600, fileSize: STREAM_BYTES, channels: "stereo", format: "aac" },
          },
        })
      );
      return;
    }

    if (req.method === "POST" && url.pathname === "/content") {
      req.resume();
      req.on("end", () => {
        res.writeHead(200, { "content-type": "application/json" });
        res.end(JSON.stringify({ card: { cardId: "card-64mb", title: "Large 64MB Audiobook" } }));
      });
      return;
    }

    res.writeHead(404);
    res.end();
  });

  await new Promise((r) => mockServer.listen(0, "127.0.0.1", r));
  const baseUrl = `http://127.0.0.1:${mockServer.address().port}`;
  const manifest = JSON.parse(fs.readFileSync(path.join(pluginRoot, "plugin.json"), "utf8"));
  const child = spawn(manifest.entrypoint.command, manifest.entrypoint.args, {
    cwd: pluginRoot,
    stdio: ["pipe", "pipe", "pipe"],
    env: {
      ...process.env,
      NO_PROXY: "127.0.0.1,localhost",
      no_proxy: "127.0.0.1,localhost",
    },
  });

  const pending = new Map();
  let seq = 0;
  let peakRssMB = 0;
  const sampleTimer = setInterval(() => {
    const rss = readProcVmRssMB(child.pid);
    if (rss > peakRssMB) peakRssMB = rss;
  }, 15);

  const rl = readline.createInterface({ input: child.stdout, crlfDelay: Infinity });
  rl.on("line", (line) => {
    if (!line.trim()) return;
    const msg = JSON.parse(line);
    if (msg.id !== undefined && !msg.method) {
      const waiter = pending.get(String(msg.id));
      if (waiter) {
        pending.delete(String(msg.id));
        waiter(msg);
      }
    }
  });

  const callRpc = (method, params) => {
    seq += 1;
    const id = String(seq);
    return new Promise((resolve) => {
      pending.set(id, resolve);
      child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
    });
  };

  try {
    await callRpc("initialize", {
      protocol_version: "2026-11-01",
      shelfd_version: "0.1.0-test",
      api_base: baseUrl,
    });

    const cfg = {
      client_id: "yoto-test-client",
      api_base_url: baseUrl,
      auth_base_url: baseUrl,
      access_token: "acc-valid",
      token_expires_at: new Date(Date.now() + 3600_000).toISOString(),
      poll_interval_ms: 5,
    };

    for (let i = 0; i < 5; i++) {
      const resp = await callRpc("actions/invoke", {
        action: "send_to_yoto",
        job_id: `job-64mb-${i}`,
        target: { type: "book", id: "book-64mb" },
        values: {
          playlist_mode: "create",
          playlist_title: "64MB Card",
          include_cover: false,
        },
        auth: { user_id: "u1", api_base: baseUrl, invocation_token: "tok-loop", config: cfg },
      });
      assert.equal(resp.result.status, "completed");
      const rss = readProcVmRssMB(child.pid);
      if (rss > peakRssMB) peakRssMB = rss;
    }

    assert.equal(totalPutBytes, 5 * STREAM_BYTES, "all 5x 64 MB streams must be received in full");
    assert.ok(peakRssMB < 96, `expected child Node peak VmRSS < 96 MB across 5x 64 MB uploads, got ${peakRssMB.toFixed(2)} MB`);

    await callRpc("shutdown", {});
  } finally {
    clearInterval(sampleTimer);
    rl.close();
    child.kill("SIGKILL");
    mockServer.closeAllConnections?.();
    mockServer.close();
  }
});

