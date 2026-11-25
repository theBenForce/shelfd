import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { buildPlugin, packPlugin, validateManifest, verifyPlugin } from "../dist/cli.mjs";
import { AuthExpiredError, createPoliteFetcher, readResponseBytesWithProgress, truncateForMCP } from "../dist/index.mjs";

test("SDK utilities: AuthExpiredError, truncateForMCP, and createPoliteFetcher", async () => {
  const authErr = new AuthExpiredError("Cookie expired");
  assert.equal(authErr.code, -32001);
  assert.equal(authErr.message, "Cookie expired");

  assert.equal(truncateForMCP("hello", 10), "hello");
  assert.match(truncateForMCP("abcdefghij", 5), /^abcde\n\.\.\.\[truncated\]$/);

  let calls = 0;
  const timestamps = [];
  const politeFetch = createPoliteFetcher({
    minIntervalMs: 40,
    maxRetries: 2,
    fetchImpl: async (_url, init) => {
      calls += 1;
      timestamps.push(Date.now());
      const headers = new Headers(init?.headers);
      assert.match(headers.get("user-agent") ?? "", /Shelfd-Plugin/);
      if (calls === 1) {
        return new Response("rate limited", { status: 429, headers: { "retry-after": "0.02" } });
      }
      return new Response(JSON.stringify({ ok: true }), { status: 200 });
    },
  });

  const res1 = await politeFetch("https://example.com/a");
  assert.equal(res1.status, 200);
  assert.equal(calls, 2);

  const res2 = await politeFetch("https://example.com/b");
  assert.equal(res2.status, 200);
  assert.ok(timestamps[timestamps.length - 1] - timestamps[0] >= 30);
});

test("validateManifest catches invalid names, versions, scopes, auth, and actions", () => {
  assert.equal(
    validateManifest({
      name: "valid-plugin",
      version: "1.0.0",
      entrypoint: { command: "node", args: ["dist/index.mjs"] },
      capabilities: ["catalog", "tools", "actions"],
      api_scopes: ["uploads:stage", "books:read"],
      auth: {
        methods: ["oauth_pkce", "token"],
        title: "Link Account",
      },
      actions: [
        {
          name: "send_to_device",
          title: "Send to Device",
          targets: ["book", "series"],
          media_types: ["audiobook"],
          mode: "form",
          execution: "job",
        },
      ],
    }).length,
    0
  );

  const errs = validateManifest({
    name: "UPPERCASE_BAD",
    version: "not-semver",
    entrypoint: { command: "../bad" },
    capabilities: ["root"],
    api_scopes: ["admin:all"],
    auth: { methods: [] },
    actions: [{ name: "", title: "", targets: ["invalid_target"] }],
  });
  assert.equal(errs.length, 9);
});

test("buildPlugin, verifyPlugin, and packPlugin end-to-end on a TypeScript plugin", async () => {
  const tmpPluginDir = fs.mkdtempSync(path.join(os.tmpdir(), "sdk-test-plugin-"));
  try {
    fs.mkdirSync(path.join(tmpPluginDir, "src"), { recursive: true });
    fs.writeFileSync(
      path.join(tmpPluginDir, "plugin.json"),
      JSON.stringify(
        {
          name: "sample-ts-plugin",
          version: "0.2.0",
          display_name: "Sample TS Plugin",
          entrypoint: { command: "node", args: ["dist/index.mjs"] },
          capabilities: ["catalog", "tools"],
          catalog_modes: ["search"],
          api_scopes: ["uploads:stage"],
          config_schema: {
            type: "object",
            properties: {
              session_secret: { type: "string", "x-shelfd-secret": true },
              base_url: { type: "string" },
            },
          },
        },
        null,
        2
      )
    );

    fs.writeFileSync(
      path.join(tmpPluginDir, "src", "helper.ts"),
      `export function formatAuthor(raw: string): string {\n  return raw.trim();\n}\n`
    );

    fs.writeFileSync(
      path.join(tmpPluginDir, "src", "index.ts"),
      `import { definePlugin } from "@shelfd/plugin-sdk";
import { formatAuthor } from "./helper.ts";

interface SampleConfig extends Record<string, unknown> {
  session_secret?: string;
  base_url?: string;
}

definePlugin<SampleConfig>({
  name: "sample-ts-plugin",
  version: "0.2.0",
  onInitialize() {
    // Accidental console.log must be redirected safely to stderr so stdout NDJSON stays clean
    console.log("Initializing sample-ts-plugin safely");
  },
  tools: {
    ping: {
      description: "Test ping tool",
      inputSchema: { type: "object", properties: {} },
      execute: () => "pong",
    },
  },
  catalog: {
    async search(_req, _ctx) {
      return {
        items: [
          {
            external_id: "84",
            title: "Frankenstein",
            authors: [formatAuthor("Mary Shelley")],
            formats: [{ format: "epub" }],
          },
        ],
      };
    },
    async stage(req, ctx) {
      const staged = await ctx.stageBook({
        data: new TextEncoder().encode("PK-mock-epub"),
        filename: "frankenstein.epub",
        externalId: req.items[0].external_id,
        metadataDefaults: { title: "Frankenstein", authors: ["Mary Shelley"] },
      });
      return {
        results: [{ external_id: req.items[0].external_id, status: "staged", upload_job_id: staged.job_id }],
      };
    },
  },
});
`
    );

    const built = await buildPlugin(tmpPluginDir);
    assert.ok(fs.existsSync(built.outFile));

    const report = await verifyPlugin(tmpPluginDir);
    assert.equal(report.passed, true, JSON.stringify(report.checks, null, 2));

    const zipPath = await packPlugin(tmpPluginDir);
    assert.ok(fs.existsSync(zipPath));
    assert.ok(fs.statSync(zipPath).size > 100);
  } finally {
    fs.rmSync(tmpPluginDir, { recursive: true, force: true });
  }
});

test("definePlugin action & auth handlers, ctx.openLoopbackStream, host/refresh_token 401 retry, and verifyPlugin on action fixture", async () => {
  const { spawn } = await import("node:child_process");
  const http = await import("node:http");
  const readline = await import("node:readline");

  const tmpPluginDir = fs.mkdtempSync(path.join(os.tmpdir(), "sdk-action-plugin-"));
  let loopbackRequests = 0;
  let lastAuthHeader = "";

  const mockLoopbackServer = http.createServer((req, res) => {
    const url = req.url ?? "/";
    const auth = req.headers.authorization ?? "";
    lastAuthHeader = auth;
    loopbackRequests += 1;

    if (auth === "Bearer expired-token") {
      res.writeHead(401, { "content-type": "application/json" });
      res.end(JSON.stringify({ error: "token expired" }));
      return;
    }

    if (req.method === "GET" && url === "/api/v1/books/book-42/markdown") {
      res.writeHead(200, {
        "content-type": "text/markdown; charset=utf-8",
        "content-disposition": 'attachment; filename="Dune_42.md"',
      });
      res.end("# Dune\n\nArrakis awaits.\n");
      return;
    }

    if (req.method === "GET" && url === "/api/v1/books/book-42/files/f1/stream") {
      const payload = Buffer.from("streamed-audio-bytes-0123456789");
      res.writeHead(200, {
        "content-type": "audio/mpeg",
        "content-length": String(payload.length),
      });
      res.end(payload);
      return;
    }

    res.writeHead(404);
    res.end("not found");
  });

  await new Promise((resolve) => mockLoopbackServer.listen(0, "127.0.0.1", resolve));
  const addr = mockLoopbackServer.address();
  const apiBase = `http://127.0.0.1:${typeof addr === "object" && addr ? addr.port : 0}`;

  try {
    fs.mkdirSync(path.join(tmpPluginDir, "src"), { recursive: true });
    fs.writeFileSync(
      path.join(tmpPluginDir, "plugin.json"),
      JSON.stringify(
        {
          name: "action-fixture",
          version: "1.0.0",
          display_name: "Action Fixture",
          entrypoint: { command: "node", args: ["dist/index.mjs"] },
          capabilities: ["actions", "events"],
          api_scopes: ["books:read"],
          auth: {
            methods: ["oauth_pkce", "token"],
            title: "Link Action Fixture",
          },
          actions: [
            {
              name: "demo_form",
              title: "Demo Form Action",
              targets: ["book", "series"],
              media_types: ["audiobook", "ebook"],
              mode: "form",
              execution: "job",
            },
          ],
          config_schema: {
            type: "object",
            properties: {
              access_token: { type: "string", "x-shelfd-secret": true },
            },
          },
        },
        null,
        2
      )
    );

    fs.writeFileSync(
      path.join(tmpPluginDir, "src", "index.ts"),
      `import { definePlugin } from "@shelfd/plugin-sdk";

interface ActionConfig extends Record<string, unknown> {
  access_token?: string;
}

definePlugin<ActionConfig>({
  name: "action-fixture",
  version: "1.0.0",
  auth: {
    spec: {
      methods: ["oauth_pkce", "token"],
      title: "Link Action Fixture",
    },
    status(ctx) {
      const linked = Boolean(ctx.config.access_token);
      return {
        connected: linked,
        configured: linked,
        account_label: linked ? "linked-user" : undefined,
      };
    },
    begin(req) {
      return {
        method: req.method ?? "oauth_pkce",
        authorize_url: \`https://auth.example.com/authorize?state=\${req.state ?? ""}&challenge=\${req.code_challenge ?? ""}\`,
      };
    },
    async complete(req, ctx) {
      const tok = req.code ? \`pkce-\${req.code}\` : (req.access_token || req.credentials?.token || req.values?.access_token || "manual-tok");
      await ctx.patchConfig({ access_token: tok });
      const label = \`\${req.access_token ?? "-"}|\${req.refresh_token ?? "-"}|\${req.client_id ?? "-"}|\${req.credentials?.patronId ?? "-"}|\${req.credentials?.count ?? "-"}\`;
      return { connected: true, configured: true, account_label: \`[\${label}]\` };
    },
    async disconnect(ctx) {
      await ctx.patchConfig({ access_token: "" });
      return { connected: false, configured: false };
    },
  },
  actions: {
    demo_form: {
      title: "Demo Form Action",
      targets: ["book", "series"],
      mediaTypes: ["audiobook", "ebook"],
      mode: "form",
      execution: "job",
      prepare(req) {
        return {
          title: \`Send \${req.target.title ?? req.target.id}\`,
          submit_label: "Start Transfer",
          entries: [
            { id: "ch-1", index: 1, title: "Chapter 1" },
          ],
          fields: [
            {
              key: "target_mode",
              kind: "segmented",
              title: "Target Mode",
              default_value: "create",
              options: [
                { value: "create", label: "Create New" },
                { value: "existing", label: "Existing" },
              ],
            },
            {
              key: "playlist_id",
              kind: "remote_select",
              title: "Playlist",
              options_source: "playlists",
              visible_when: { field: "target_mode", equals: "existing" },
            },
            {
              key: "icon",
              kind: "asset_picker",
              title: "Chapter Icon",
              options_source: "icons",
              asset_shape: "square_pixel",
              allow_upload: true,
              per_entry_override: true,
            },
          ],
        };
      },
      options(req) {
        return {
          items: [
            {
              value: \`\${req.source}-1\`,
              label: \`Option for \${req.source} (\${req.query ?? ""})\`,
            },
          ],
        };
      },
      uploadAsset(req) {
        const raw = Buffer.from(req.data_base64, "base64");
        return {
          item: {
            value: \`asset://\${req.filename}\`,
            label: \`\${req.filename} (\${raw.length}B)\`,
            icon_uri: \`asset://\${req.filename}\`,
          },
        };
      },
      async invoke(req, ctx) {
        ctx.reportProgress({
          jobId: req.job_id,
          current: 1,
          total: 2,
          percent: 50,
          title: "Chapter 1",
          message: "Reading markdown and stream...",
        });
        const md = await ctx.getBookMarkdown(req.target.id);
        const streamInfo = await ctx.openLoopbackStream(\`/api/v1/books/\${req.target.id}/files/f1/stream\`);
        const reader = streamInfo.stream.getReader();
        let totalBytes = 0;
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          totalBytes += value.byteLength;
        }
        return {
          status: "completed",
          message: \`Transferred \${md.fileName} (\${totalBytes} stream bytes)\`,
          data: {
            fileName: md.fileName,
            streamBytes: totalBytes,
            contentLength: streamInfo.contentLength,
            contentType: streamInfo.contentType,
            overrideCh1: req.overrides?.["ch-1"] ?? null,
          },
        };
      },
    },
  },
});
`
    );

    const built = await buildPlugin(tmpPluginDir);
    assert.ok(fs.existsSync(built.outFile));

    const verifyReport = await verifyPlugin(tmpPluginDir);
    assert.equal(verifyReport.passed, true, JSON.stringify(verifyReport.checks, null, 2));
    assert.ok(verifyReport.checks.some((c) => c.name === "auth_status_contract" && c.passed));
    assert.ok(verifyReport.checks.some((c) => c.name === "actions_prepare_contract" && c.passed));

    const child = spawn("node", [built.outFile], {
      cwd: tmpPluginDir,
      stdio: ["pipe", "pipe", "pipe"],
    });

    const pending = new Map();
    const notifications = [];
    let refreshTokenCalls = 0;
    let patchedConfigs = [];
    let reqSeq = 0;

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
      } else if (msg.method === "host/refresh_token") {
        refreshTokenCalls += 1;
        child.stdin.write(
          `${JSON.stringify({
            jsonrpc: "2.0",
            id: msg.id,
            result: { invocation_token: "fresh-valid-token-123" },
          })}\n`
        );
      } else if (msg.method === "config/patch") {
        patchedConfigs.push(msg.params);
        child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id: msg.id, result: { ok: true } })}\n`);
      } else if (msg.method) {
        notifications.push(msg);
      }
    });

    const callRpc = (method, params) => {
      reqSeq += 1;
      const id = String(reqSeq);
      return new Promise((resolve) => {
        pending.set(id, resolve);
        child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
      });
    };

    const initRes = await callRpc("initialize", {
      protocol_version: "2026-11-01",
      shelfd_version: "0.1.0-test",
      api_base: apiBase,
      data_dir: tmpPluginDir,
    });
    assert.equal(initRes.result.name, "action-fixture");
    assert.equal(initRes.result.actions.length, 1);
    assert.equal(initRes.result.actions[0].name, "demo_form");

    const authEnv = {
      user_id: "u-1",
      invocation_token: "expired-token",
      api_base: apiBase,
      config: {},
    };

    const beginRes = await callRpc("auth/begin", {
      method: "oauth_pkce",
      state: "st-1",
      code_challenge: "ch-1",
      auth: authEnv,
    });
    assert.match(beginRes.result.authorize_url, /state=st-1&challenge=ch-1/);

    const completeRes = await callRpc("auth/complete", {
      method: "oauth_pkce",
      code: "oauth-code-99",
      auth: authEnv,
    });
    assert.equal(completeRes.result.connected, true);
    assert.equal(patchedConfigs.length, 1);
    assert.equal(patchedConfigs[0].values.access_token, "pkce-oauth-code-99");

    const tokenCompleteRes = await callRpc("auth/complete", {
      method: "token",
      access_token: "hoopla-token-xyz",
      refresh_token: "hoopla-refresh-1",
      client_id: "www-client",
      credentials: { token: "cookie-token-abc", patronId: "patron-42", count: 7 },
      auth: authEnv,
    });
    assert.equal(tokenCompleteRes.result.connected, true);
    assert.equal(
      tokenCompleteRes.result.account_label,
      "[hoopla-token-xyz|hoopla-refresh-1|www-client|patron-42|7]"
    );
    assert.equal(patchedConfigs[1].values.access_token, "hoopla-token-xyz");

    const disconnectRes = await callRpc("auth/disconnect", { auth: authEnv });
    assert.equal(disconnectRes.result.connected, false);

    const optRes = await callRpc("actions/options", {
      action: "demo_form",
      source: "playlists",
      query: "bedtime",
      auth: authEnv,
    });
    assert.equal(optRes.result.items[0].value, "playlists-1");
    assert.match(optRes.result.items[0].label, /bedtime/);

    const uploadRes = await callRpc("actions/upload_asset", {
      action: "demo_form",
      field: "icon",
      filename: "icon.png",
      mime_type: "image/png",
      data_base64: Buffer.from("12345678").toString("base64"),
      auth: authEnv,
    });
    assert.equal(uploadRes.result.item.value, "asset://icon.png");

    const invokeRes = await callRpc("actions/invoke", {
      action: "demo_form",
      job_id: "job-77",
      target: { type: "book", id: "book-42", title: "Dune" },
      values: { target_mode: "create" },
      overrides: { "ch-1": "asset://icon.png" },
      auth: authEnv,
    });
    assert.equal(invokeRes.result.status, "completed");
    assert.equal(refreshTokenCalls, 1);
    assert.equal(lastAuthHeader, "Bearer fresh-valid-token-123");
    assert.ok(loopbackRequests >= 3);
    assert.equal(invokeRes.result.data.fileName, "Dune_42.md");
    assert.equal(invokeRes.result.data.streamBytes, 31);
    assert.equal(invokeRes.result.data.contentLength, 31);
    assert.equal(invokeRes.result.data.contentType, "audio/mpeg");
    assert.equal(invokeRes.result.data.overrideCh1, "asset://icon.png");
    assert.ok(notifications.some((n) => n.method === "progress" && n.params.job_id === "job-77"));

    await callRpc("shutdown", {});
  } finally {
    mockLoopbackServer.close();
    fs.rmSync(tmpPluginDir, { recursive: true, force: true });
  }
});

test("readResponseBytesWithProgress reports percentage with Content-Length and byte keepalives without Content-Length", async () => {
  // 1. Known Content-Length
  const chunksKnown = [
    new Uint8Array([1, 2, 3, 4]),
    new Uint8Array([5, 6, 7, 8]),
    new Uint8Array([9, 10]),
  ];
  const streamKnown = new ReadableStream({
    start(controller) {
      for (const ch of chunksKnown) controller.enqueue(ch);
      controller.close();
    },
  });
  const resKnown = new Response(streamKnown, {
    headers: { "content-length": "10" },
  });
  const eventsKnown = [];
  const outKnown = await readResponseBytesWithProgress(
    resKnown,
    (info) => eventsKnown.push(info),
    { minPercentDelta: 20, minIntervalMs: 0 }
  );
  assert.deepEqual(Array.from(outKnown), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
  assert.ok(eventsKnown.length >= 2);
  assert.equal(eventsKnown[0].fraction, 0.4);
  assert.equal(eventsKnown[eventsKnown.length - 1].fraction, 1);

  // 2. Chunked (no Content-Length) emits keepalive with loadedBytes and final fraction=1
  const streamChunked = new ReadableStream({
    async start(controller) {
      controller.enqueue(new Uint8Array([11, 12, 13]));
      await new Promise((r) => setTimeout(r, 15));
      controller.enqueue(new Uint8Array([14, 15]));
      controller.close();
    },
  });
  const resChunked = new Response(streamChunked);
  const eventsChunked = [];
  const outChunked = await readResponseBytesWithProgress(
    resChunked,
    (info) => eventsChunked.push(info),
    { keepaliveIntervalMs: 5 }
  );
  assert.deepEqual(Array.from(outChunked), [11, 12, 13, 14, 15]);
  assert.ok(eventsChunked.some((e) => e.fraction === undefined && e.loadedBytes >= 3));
  assert.equal(eventsChunked[eventsChunked.length - 1].fraction, 1);
  assert.equal(eventsChunked[eventsChunked.length - 1].loadedBytes, 5);
});

test("catalog/stage auto-injects job_id into reportProgress and rebuilds fresh FormData on 401 token refresh", async () => {
  const { spawn } = await import("node:child_process");
  const http = await import("node:http");
  const readline = await import("node:readline");

  const tmpPluginDir = fs.mkdtempSync(path.join(os.tmpdir(), "sdk-stage-refresh-"));
  const stageBodies = [];

  const mockStageServer = http.createServer((req, res) => {
    const auth = req.headers.authorization ?? "";
    const chunks = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => {
      const rawBody = Buffer.concat(chunks).toString("utf8");
      stageBodies.push({ auth, bodyLen: rawBody.length, hasEpubBytes: rawBody.includes("PK-fresh-epub-payload") });
      if (auth === "Bearer expired-stage-tok") {
        res.writeHead(401, { "content-type": "application/json" });
        res.end(JSON.stringify({ error: "expired token" }));
        return;
      }
      res.writeHead(201, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          job_id: "upload-job-999",
          status: "staged",
          filename: "test.epub",
          has_cover: false,
          metadata: { title: "Test Book", authors: ["Author"] },
        })
      );
    });
  });

  await new Promise((resolve) => mockStageServer.listen(0, "127.0.0.1", resolve));
  const addr = mockStageServer.address();
  const apiBase = `http://127.0.0.1:${typeof addr === "object" && addr ? addr.port : 0}`;

  try {
    fs.mkdirSync(path.join(tmpPluginDir, "src"), { recursive: true });
    fs.writeFileSync(
      path.join(tmpPluginDir, "plugin.json"),
      JSON.stringify({
        name: "stage-refresh-fixture",
        version: "1.0.0",
        entrypoint: { command: "node", args: ["dist/index.mjs"] },
        capabilities: ["catalog"],
        catalog_modes: ["browse"],
        api_scopes: ["uploads:stage"],
      })
    );
    fs.writeFileSync(
      path.join(tmpPluginDir, "src", "index.ts"),
      `import { definePlugin } from "@shelfd/plugin-sdk";
definePlugin({
  name: "stage-refresh-fixture",
  version: "1.0.0",
  catalog: {
    async browse() {
      return { items: [] };
    },
    async stage(req, ctx) {
      // Intentionally omit jobId in reportProgress to verify SDK auto-injection from req.job_id
      ctx.reportProgress({
        current: 1,
        total: req.items.length,
        percent: 50,
        title: req.items[0].title,
        message: "Downloading...",
      });
      const staged = await ctx.stageBook({
        data: new TextEncoder().encode("PK-fresh-epub-payload"),
        filename: "book.epub",
        externalId: req.items[0].external_id,
      });
      return {
        results: [{ external_id: req.items[0].external_id, title: req.items[0].title, upload_job_id: staged.job_id, status: "staged" }],
      };
    },
  },
});
`
    );

    const built = await buildPlugin(tmpPluginDir);
    const child = spawn(process.execPath, [built.outFile], {
      cwd: tmpPluginDir,
      stdio: ["pipe", "pipe", "pipe"],
    });

    const pending = new Map();
    const progressFrames = [];
    let refreshCount = 0;

    const rl = readline.createInterface({ input: child.stdout, crlfDelay: Infinity });
    rl.on("line", (line) => {
      if (!line.trim()) return;
      const msg = JSON.parse(line);
      if (msg.method === "host/refresh_token" && msg.id !== undefined) {
        refreshCount += 1;
        child.stdin.write(
          `${JSON.stringify({ jsonrpc: "2.0", id: msg.id, result: { invocation_token: "fresh-stage-tok" } })}\n`
        );
        return;
      }
      if (msg.method === "progress") {
        progressFrames.push(msg.params);
        return;
      }
      if (msg.id !== undefined && pending.has(String(msg.id))) {
        const resolve = pending.get(String(msg.id));
        pending.delete(String(msg.id));
        resolve(msg);
      }
    });

    let seq = 0;
    const callRpc = (method, params) => {
      seq += 1;
      const id = String(seq);
      return new Promise((resolve) => {
        pending.set(id, resolve);
        child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
      });
    };

    await callRpc("initialize", {
      protocol_version: "2026-11-01",
      shelfd_version: "0.1.0",
      api_base: apiBase,
      data_dir: tmpPluginDir,
    });

    const stageRes = await callRpc("catalog/stage", {
      job_id: "pjob-stage-42",
      items: [{ external_id: "ext-1", title: "Twenty Thousand Leagues" }],
      auth: {
        user_id: "u-stage",
        invocation_token: "expired-stage-tok",
        api_base: apiBase,
        config: {},
      },
    });

    assert.equal(stageRes.result.results[0].upload_job_id, "upload-job-999");
    assert.equal(refreshCount, 1);
    assert.equal(stageBodies.length, 2);
    assert.equal(stageBodies[0].hasEpubBytes, true);
    assert.equal(stageBodies[1].hasEpubBytes, true);
    assert.equal(stageBodies[1].auth, "Bearer fresh-stage-tok");
    assert.equal(progressFrames.length, 1);
    assert.equal(progressFrames[0].job_id, "pjob-stage-42");
    assert.equal(progressFrames[0].title, "Twenty Thousand Leagues");

    await callRpc("shutdown", {});
  } finally {
    mockStageServer.close();
    fs.rmSync(tmpPluginDir, { recursive: true, force: true });
  }
});



test("stageBook refreshes the token when the server closes the socket mid-upload (early 401 regression)", async () => {
  const { spawn } = await import("node:child_process");
  const http = await import("node:http");
  const readline = await import("node:readline");

  const tmpPluginDir = fs.mkdtempSync(path.join(os.tmpdir(), "sdk-early-close-"));
  const seenAuths = [];
  let acceptedAuth = "";

  // Server behavior: on an expired token it rejects immediately by destroying the
  // socket without reading or draining the request body — the exact production
  // failure mode that made fetch() throw instead of returning a 401 Response.
  const mockStageServer = http.createServer((req, res) => {
    const auth = req.headers.authorization ?? "";
    seenAuths.push(auth);
    if (auth === "Bearer expired-early-close-tok") {
      req.socket.destroy();
      return;
    }
    const chunks = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => {
      acceptedAuth = auth;
      res.writeHead(201, { "content-type": "application/json" });
      res.end(JSON.stringify({ job_id: "upload-job-early-close", status: "staged", filename: "big.m4b" }));
    });
  });

  await new Promise((resolve) => mockStageServer.listen(0, "127.0.0.1", resolve));
  const addr = mockStageServer.address();
  const apiBase = `http://127.0.0.1:${typeof addr === "object" && addr ? addr.port : 0}`;

  try {
    fs.mkdirSync(path.join(tmpPluginDir, "src"), { recursive: true });
    fs.writeFileSync(
      path.join(tmpPluginDir, "plugin.json"),
      JSON.stringify({
        name: "early-close-fixture",
        version: "1.0.0",
        entrypoint: { command: "node", args: ["dist/index.mjs"] },
        capabilities: ["catalog"],
        catalog_modes: ["browse"],
        api_scopes: ["uploads:stage"],
      })
    );
    // Large payload so the client is still streaming the body when the socket dies.
    fs.writeFileSync(
      path.join(tmpPluginDir, "src", "index.ts"),
      `import { definePlugin } from "@shelfd/plugin-sdk";
definePlugin({
  name: "early-close-fixture",
  version: "1.0.0",
  catalog: {
    async browse() { return { items: [] }; },
    async stage(req, ctx) {
      const data = new Uint8Array(8 * 1024 * 1024);
      data.fill(7);
      const staged = await ctx.stageBook({
        data,
        filename: "big.m4b",
        externalId: req.items[0].external_id,
      });
      return { results: [{ external_id: req.items[0].external_id, upload_job_id: staged.job_id, status: "staged" }] };
    },
  },
});
`
    );

    const built = await buildPlugin(tmpPluginDir);
    const child = spawn(process.execPath, [built.outFile], {
      cwd: tmpPluginDir,
      stdio: ["pipe", "pipe", "pipe"],
    });

    const pending = new Map();
    let refreshCount = 0;

    const rl = readline.createInterface({ input: child.stdout, crlfDelay: Infinity });
    rl.on("line", (line) => {
      if (!line.trim()) return;
      const msg = JSON.parse(line);
      if (msg.method === "host/refresh_token" && msg.id !== undefined) {
        refreshCount += 1;
        child.stdin.write(
          `${JSON.stringify({ jsonrpc: "2.0", id: msg.id, result: { invocation_token: "fresh-early-close-tok" } })}\n`
        );
        return;
      }
      if (msg.id !== undefined && pending.has(String(msg.id))) {
        const resolve = pending.get(String(msg.id));
        pending.delete(String(msg.id));
        resolve(msg);
      }
    });

    let seq = 0;
    const callRpc = (method, params) => {
      seq += 1;
      const id = String(seq);
      return new Promise((resolve) => {
        pending.set(id, resolve);
        child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
      });
    };

    await callRpc("initialize", {
      protocol_version: "2026-11-01",
      shelfd_version: "0.1.0",
      api_base: apiBase,
      data_dir: tmpPluginDir,
    });

    const stageRes = await callRpc("catalog/stage", {
      job_id: "pjob-early-close",
      items: [{ external_id: "ext-9", title: "Odyssey" }],
      auth: {
        user_id: "u-stage",
        invocation_token: "expired-early-close-tok",
        api_base: apiBase,
        config: {},
      },
    });

    assert.equal(stageRes.result.results[0].status, "staged");
    assert.equal(stageRes.result.results[0].upload_job_id, "upload-job-early-close");
    assert.deepEqual(seenAuths, ["Bearer expired-early-close-tok", "Bearer fresh-early-close-tok"]);
    assert.equal(acceptedAuth, "Bearer fresh-early-close-tok");
    assert.equal(refreshCount, 1);

    await callRpc("shutdown", {});
  } finally {
    mockStageServer.close();
    fs.rmSync(tmpPluginDir, { recursive: true, force: true });
  }
});

test("stageBook({ filePath }) streams disk-backed files via openAsBlob and re-streams cleanly on 401 and early socket close", async () => {
  const { spawn } = await import("node:child_process");
  const crypto = await import("node:crypto");
  const http = await import("node:http");
  const readline = await import("node:readline");

  const tmpPluginDir = fs.mkdtempSync(path.join(os.tmpdir(), "sdk-filepath-stage-"));
  const audioFilePath = path.join(tmpPluginDir, "odyssey.m4b");
  // 8 MB payload so early socket destroy happens mid-stream
  const expectedAudioBuf = crypto.randomBytes(8 * 1024 * 1024);
  const expectedSha256 = crypto.createHash("sha256").update(expectedAudioBuf).digest("hex");
  fs.writeFileSync(audioFilePath, expectedAudioBuf);

  const seenRequests = [];

  const mockStageServer = http.createServer(async (req, res) => {
    const auth = req.headers.authorization ?? "";
    const contentType = req.headers["content-type"] ?? "";

    if (auth === "Bearer expired-socket-close-tok") {
      seenRequests.push({ auth, outcome: "socket_destroyed" });
      req.socket.destroy();
      return;
    }

    if (auth === "Bearer expired-401-tok") {
      // Drain body then return 401
      for await (const _chunk of req) {
        // drain
      }
      seenRequests.push({ auth, outcome: "http_401" });
      res.writeHead(401, { "content-type": "application/json" });
      res.end(JSON.stringify({ error: "token expired" }));
      return;
    }

    // Parse multipart request using Web Request + FormData to verify streamed contents
    const chunks = [];
    for await (const chunk of req) {
      chunks.push(chunk);
    }
    const rawBody = Buffer.concat(chunks);
    const webReq = new Request("http://127.0.0.1/api/v1/books/upload/stage", {
      method: "POST",
      headers: { "content-type": contentType },
      body: rawBody,
    });
    const form = await webReq.formData();
    const fileEntry = form.get("file");
    const coverEntry = form.get("cover");
    const fileBuf = Buffer.from(await fileEntry.arrayBuffer());
    const fileSha256 = crypto.createHash("sha256").update(fileBuf).digest("hex");

    seenRequests.push({
      auth,
      outcome: "accepted",
      fileName: fileEntry.name,
      fileType: fileEntry.type,
      fileBytes: fileBuf.byteLength,
      fileSha256,
      hasCover: coverEntry instanceof File && coverEntry.size > 0,
      sourcePlugin: form.get("source_plugin"),
      sourceExternalId: form.get("source_external_id"),
      metadataDefaults: JSON.parse(String(form.get("metadata_defaults") ?? "{}")),
    });

    res.writeHead(200, { "content-type": "application/json" });
    res.end(
      JSON.stringify({
        job_id: `job-${seenRequests.length}`,
        status: "staged",
        filename: fileEntry.name,
        has_cover: Boolean(coverEntry),
        metadata: { title: "The Odyssey", authors: ["Homer"] },
      })
    );
  });

  await new Promise((resolve) => mockStageServer.listen(0, "127.0.0.1", resolve));
  const addr = mockStageServer.address();
  const apiBase = `http://127.0.0.1:${typeof addr === "object" && addr ? addr.port : 0}`;

  try {
    fs.mkdirSync(path.join(tmpPluginDir, "src"), { recursive: true });
    fs.writeFileSync(
      path.join(tmpPluginDir, "plugin.json"),
      JSON.stringify({
        name: "filepath-stage-fixture",
        version: "1.0.0",
        entrypoint: { command: "node", args: ["dist/index.mjs"] },
        capabilities: ["catalog", "commands"],
        catalog_modes: ["browse"],
        api_scopes: ["uploads:stage"],
      })
    );

    fs.writeFileSync(
      path.join(tmpPluginDir, "src", "index.ts"),
      `import { definePlugin } from "@shelfd/plugin-sdk";
definePlugin({
  name: "filepath-stage-fixture",
  version: "1.0.0",
  commands: {
    validate_stage_errors: {
      async execute(args, ctx) {
        const filePath = String(args.file_path ?? "");
        const dirPath = String(args.dir_path ?? "");
        const errors: Record<string, string> = {};

        for (const [label, badInput] of [
          ["neither", { filename: "odyssey.m4b" }],
          ["blank_path", { filePath: "   ", filename: "odyssey.m4b" }],
          ["both", { filePath, data: new Uint8Array([1, 2, 3]), filename: "odyssey.m4b" }],
          ["missing_file", { filePath: filePath + ".does-not-exist", filename: "odyssey.m4b" }],
          ["directory_path", { filePath: dirPath, filename: "odyssey.m4b" }],
        ] as const) {
          try {
            await ctx.stageBook(badInput as never);
            errors[label] = "DID_NOT_THROW";
          } catch (err) {
            errors[label] = err instanceof Error ? err.message : String(err);
          }
        }
        return errors;
      },
    },
  },
  catalog: {
    async browse() {
      return { items: [] };
    },
    async stage(req, ctx) {
      const targetPath = String(ctx.config.audio_path ?? "");
      const staged = await ctx.stageBook({
        filePath: targetPath,
        filename: "The_Odyssey.m4b",
        mimeType: "audio/mp4",
        externalId: req.items[0].external_id,
        cover: {
          data: new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 1, 2, 3, 4]),
          filename: "cover.jpg",
          mimeType: "image/jpeg",
        },
        metadataDefaults: {
          title: "The Odyssey",
          authors: ["Homer"],
          narrator: "Claire Danes",
        },
      });
      return {
        results: [{ external_id: req.items[0].external_id, upload_job_id: staged.job_id, status: "staged" }],
      };
    },
  },
});
`
    );

    const built = await buildPlugin(tmpPluginDir);
    const child = spawn(process.execPath, [built.outFile], {
      cwd: tmpPluginDir,
      stdio: ["pipe", "pipe", "pipe"],
    });

    const pending = new Map();
    let refreshSeq = 0;

    const rl = readline.createInterface({ input: child.stdout, crlfDelay: Infinity });
    rl.on("line", (line) => {
      if (!line.trim()) return;
      const msg = JSON.parse(line);
      if (msg.method === "host/refresh_token" && msg.id !== undefined) {
        refreshSeq += 1;
        child.stdin.write(
          `${JSON.stringify({ jsonrpc: "2.0", id: msg.id, result: { invocation_token: `refreshed-tok-${refreshSeq}` } })}\n`
        );
        return;
      }
      if (msg.id !== undefined && pending.has(String(msg.id))) {
        const resolve = pending.get(String(msg.id));
        pending.delete(String(msg.id));
        resolve(msg);
      }
    });

    let seq = 0;
    const callRpc = (method, params) => {
      seq += 1;
      const id = String(seq);
      return new Promise((resolve) => {
        pending.set(id, resolve);
        child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
      });
    };

    await callRpc("initialize", {
      protocol_version: "2026-11-01",
      shelfd_version: "0.1.0",
      api_base: apiBase,
      data_dir: tmpPluginDir,
    });

    // 1. Input validation errors must fail synchronously/locally BEFORE sending any HTTP request
    const valRes = await callRpc("commands/invoke", {
      command: "validate_stage_errors",
      args: { file_path: audioFilePath, dir_path: tmpPluginDir },
      auth: {
        user_id: "u-stage",
        invocation_token: "valid-direct-tok",
        api_base: apiBase,
        config: {},
      },
    });
    assert.match(valRes.result.neither, /must provide either data or filePath/);
    assert.match(valRes.result.blank_path, /must provide either data or filePath/);
    assert.match(valRes.result.both, /provide either data or filePath, not both/);
    assert.match(valRes.result.missing_file, /is not readable/);
    assert.match(valRes.result.directory_path, /is not a regular file/);
    assert.equal(seenRequests.length, 0, "invalid stageBook inputs must not issue HTTP requests");

    // 2. Direct successful upload streaming filePath from disk
    const directRes = await callRpc("catalog/stage", {
      job_id: "pjob-direct",
      items: [{ external_id: "odyssey-direct", title: "The Odyssey" }],
      auth: {
        user_id: "u-stage",
        invocation_token: "valid-direct-tok",
        api_base: apiBase,
        config: { audio_path: audioFilePath },
      },
    });
    assert.equal(directRes.result.results[0].status, "staged");
    assert.equal(seenRequests.length, 1);
    assert.equal(seenRequests[0].outcome, "accepted");
    assert.equal(seenRequests[0].fileName, "The_Odyssey.m4b");
    assert.equal(seenRequests[0].fileType, "audio/mp4");
    assert.equal(seenRequests[0].fileBytes, expectedAudioBuf.byteLength);
    assert.equal(seenRequests[0].fileSha256, expectedSha256);
    assert.equal(seenRequests[0].hasCover, true);
    assert.equal(seenRequests[0].sourcePlugin, "filepath-stage-fixture");
    assert.equal(seenRequests[0].sourceExternalId, "odyssey-direct");
    assert.equal(seenRequests[0].metadataDefaults.narrator, "Claire Danes");

    // 3. Retry after HTTP 401 re-streams filePath cleanly from disk
    const retry401Res = await callRpc("catalog/stage", {
      job_id: "pjob-401",
      items: [{ external_id: "odyssey-401", title: "The Odyssey" }],
      auth: {
        user_id: "u-stage",
        invocation_token: "expired-401-tok",
        api_base: apiBase,
        config: { audio_path: audioFilePath },
      },
    });
    assert.equal(retry401Res.result.results[0].status, "staged");
    assert.equal(seenRequests.length, 3);
    assert.equal(seenRequests[1].outcome, "http_401");
    assert.equal(seenRequests[2].outcome, "accepted");
    assert.equal(seenRequests[2].auth, "Bearer refreshed-tok-1");
    assert.equal(seenRequests[2].fileBytes, expectedAudioBuf.byteLength);
    assert.equal(seenRequests[2].fileSha256, expectedSha256);

    // 4. Retry after early socket close mid-stream re-streams filePath cleanly from disk
    const retryCloseRes = await callRpc("catalog/stage", {
      job_id: "pjob-close",
      items: [{ external_id: "odyssey-close", title: "The Odyssey" }],
      auth: {
        user_id: "u-stage",
        invocation_token: "expired-socket-close-tok",
        api_base: apiBase,
        config: { audio_path: audioFilePath },
      },
    });
    assert.equal(retryCloseRes.result.results[0].status, "staged");
    assert.equal(seenRequests.length, 5);
    assert.equal(seenRequests[3].outcome, "socket_destroyed");
    assert.equal(seenRequests[4].outcome, "accepted");
    assert.equal(seenRequests[4].auth, "Bearer refreshed-tok-2");
    assert.equal(seenRequests[4].fileBytes, expectedAudioBuf.byteLength);
    assert.equal(seenRequests[4].fileSha256, expectedSha256);

    await callRpc("shutdown", {});
  } finally {
    mockStageServer.close();
    fs.rmSync(tmpPluginDir, { recursive: true, force: true });
  }
});

test("stageBook({ filePath }) enforces TCP backpressure and keeps child Node RSS below file size on a 128 MiB upload", async () => {
  const { spawn } = await import("node:child_process");
  const crypto = await import("node:crypto");
  const http = await import("node:http");
  const readline = await import("node:readline");

  const tmpPluginDir = fs.mkdtempSync(path.join(os.tmpdir(), "sdk-backpressure-rss-"));
  const audioFilePath = path.join(tmpPluginDir, "large-128mb.m4b");
  const fileSizeMB = 128;
  const chunk1MB = crypto.randomBytes(1024 * 1024);
  const expectedHash = crypto.createHash("sha256");
  const fd = fs.openSync(audioFilePath, "w");
  try {
    for (let i = 0; i < fileSizeMB; i++) {
      chunk1MB.writeUInt32BE(i, 0);
      fs.writeSync(fd, chunk1MB);
      expectedHash.update(chunk1MB);
    }
  } finally {
    fs.closeSync(fd);
  }
  const expectedTotalBytes = fileSizeMB * 1024 * 1024;

  let receivedMultipartBytes = 0;
  const mockStageServer = http.createServer(async (req, res) => {
    let bytesSinceYield = 0;
    for await (const chunk of req) {
      receivedMultipartBytes += chunk.length;
      bytesSinceYield += chunk.length;
      // Throttle receiver slightly (~120 MB/s) so an unbackpressured sender would buffer all 128 MiB in RAM
      if (bytesSinceYield >= 512 * 1024) {
        bytesSinceYield = 0;
        await new Promise((r) => setTimeout(r, 2));
      }
    }
    res.writeHead(200, { "content-type": "application/json" });
    res.end(
      JSON.stringify({
        job_id: "job-128mb",
        status: "staged",
        filename: "large-128mb.m4b",
        has_cover: false,
        metadata: { title: "Large Audiobook", authors: ["Homer"] },
      })
    );
  });

  await new Promise((resolve) => mockStageServer.listen(0, "127.0.0.1", resolve));
  const addr = mockStageServer.address();
  const apiBase = `http://127.0.0.1:${typeof addr === "object" && addr ? addr.port : 0}`;

  try {
    fs.mkdirSync(path.join(tmpPluginDir, "src"), { recursive: true });
    fs.writeFileSync(
      path.join(tmpPluginDir, "plugin.json"),
      JSON.stringify({
        name: "rss-backpressure-fixture",
        version: "1.0.0",
        entrypoint: { command: "node", args: ["dist/index.mjs"] },
        capabilities: ["catalog"],
        catalog_modes: ["browse"],
        api_scopes: ["uploads:stage"],
      })
    );
    fs.writeFileSync(
      path.join(tmpPluginDir, "src", "index.ts"),
      `import { definePlugin } from "@shelfd/plugin-sdk";
definePlugin({
  name: "rss-backpressure-fixture",
  version: "1.0.0",
  catalog: {
    async browse() { return { items: [] }; },
    async stage(req, ctx) {
      const staged = await ctx.stageBook({
        filePath: String(ctx.config.audio_path ?? ""),
        filename: "large-128mb.m4b",
        mimeType: "audio/mp4",
        externalId: req.items[0].external_id,
      });
      return { results: [{ external_id: req.items[0].external_id, upload_job_id: staged.job_id, status: "staged" }] };
    },
  },
});
`
    );

    const built = await buildPlugin(tmpPluginDir);
    const child = spawn(process.execPath, [built.outFile], {
      cwd: tmpPluginDir,
      stdio: ["pipe", "pipe", "pipe"],
    });

    const pending = new Map();
    const rl = readline.createInterface({ input: child.stdout, crlfDelay: Infinity });
    rl.on("line", (line) => {
      if (!line.trim()) return;
      const msg = JSON.parse(line);
      if (msg.id !== undefined && pending.has(String(msg.id))) {
        const resolve = pending.get(String(msg.id));
        pending.delete(String(msg.id));
        resolve(msg);
      }
    });

    let seq = 0;
    const callRpc = (method, params) => {
      seq += 1;
      const id = String(seq);
      return new Promise((resolve) => {
        pending.set(id, resolve);
        child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
      });
    };

    await callRpc("initialize", {
      protocol_version: "2026-11-01",
      shelfd_version: "0.1.0",
      api_base: apiBase,
      data_dir: tmpPluginDir,
    });

    const stageRes = await callRpc("catalog/stage", {
      job_id: "pjob-rss",
      items: [{ external_id: "ext-128mb", title: "Large Audiobook" }],
      auth: {
        user_id: "u-stage",
        invocation_token: "valid-rss-tok",
        api_base: apiBase,
        config: { audio_path: audioFilePath },
      },
    });

    assert.equal(stageRes.result.results[0].status, "staged");
    assert.ok(receivedMultipartBytes > expectedTotalBytes, "expected full 128 MiB payload received");

    if (process.platform === "linux" && child.pid) {
      const statusRaw = fs.readFileSync(`/proc/${child.pid}/status`, "utf8");
      const anonMatch = statusRaw.match(/^RssAnon:\s+(\d+)\s+kB/m);
      const hwmMatch = statusRaw.match(/^VmHWM:\s+(\d+)\s+kB/m);
      const rssAnonMB = anonMatch ? Number(anonMatch[1]) / 1024 : 0;
      const vmHwmMB = hwmMatch ? Number(hwmMatch[1]) / 1024 : 0;
      assert.ok(
        rssAnonMB > 0 && rssAnonMB < 96,
        `expected child Node RssAnon < 96 MiB during 128 MiB upload, got ${rssAnonMB.toFixed(1)} MiB`
      );
      assert.ok(
        vmHwmMB > 0 && vmHwmMB < 120,
        `expected child Node VmHWM < 120 MiB (< 128 MiB file size), got ${vmHwmMB.toFixed(1)} MiB`
      );
    }

    await callRpc("shutdown", {});
  } finally {
    mockStageServer.close();
    fs.rmSync(tmpPluginDir, { recursive: true, force: true });
  }
});


