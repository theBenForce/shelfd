import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { buildPlugin, packPlugin, validateManifest, verifyPlugin } from "../dist/cli.mjs";
import { AuthExpiredError, createPoliteFetcher, truncateForMCP } from "../dist/index.mjs";

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

