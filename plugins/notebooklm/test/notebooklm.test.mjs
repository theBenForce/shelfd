import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import readline from "node:readline";
import test from "node:test";
import { fileURLToPath } from "node:url";

const pluginRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

let validateSSRF;
test.before(async () => {
  const mod = await import("../src/index.ts");
  validateSSRF = mod.validateSSRF;
  process.stdin.pause();
});

test("notebooklm bundle is a standalone single-file ESM artifact", () => {
  const distPath = path.join(pluginRoot, "dist", "index.mjs");
  assert.ok(fs.existsSync(distPath), "dist/index.mjs must exist after build");
  const content = fs.readFileSync(distPath, "utf8");
  assert.doesNotMatch(
    content,
    /from\s+["']@thebenforce\/shelfd-plugin-sdk["']/,
    "dist/index.mjs must inline @thebenforce/shelfd-plugin-sdk so no node_modules is needed at runtime"
  );
});

test("validateSSRF rejects metadata, link-local, RFC1918, and loopback IPs unless explicitly opted in for local mock tests", () => {
  for (const blocked of [
    "http://169.254.169.254/latest/meta-data",
    "http://169.254.10.20/token",
    "http://[fd00:ec2::254]/latest/meta-data",
    "http://[fe80::1]/hook",
    "http://[::ffff:169.254.169.254]/meta",
    "http://metadata.google.internal/computeMetadata/v1",
    "http://10.0.0.12/webhook",
    "http://172.16.5.4/webhook",
    "http://192.168.1.50/webhook",
    "ftp://notebooklm.google.com/file",
  ]) {
    assert.throws(() => validateSSRF(blocked, false), /SSRF blocked|Unsupported URL scheme/);
    assert.throws(
      () => validateSSRF(blocked, true),
      /SSRF blocked|Unsupported URL scheme/,
      `Metadata / link-local / private URL ${blocked} must be blocked even when allowLocalEndpoints is true`
    );
  }

  assert.throws(() => validateSSRF("http://127.0.0.1:9090/hook", false), /SSRF blocked/);
  assert.throws(() => validateSSRF("http://localhost:9090/hook", false), /SSRF blocked/);
  assert.doesNotThrow(() => validateSSRF("http://127.0.0.1:9090/hook", true));
  assert.doesNotThrow(() => validateSSRF("https://global-discoveryengine.googleapis.com", false));
});

test("notebooklm plugin RPC end-to-end: zero-config client_handoff, webhook push, Discovery Engine two-step push, and SSRF rejection", async () => {
  let webhookPayload = null;
  let webhookAuthHeader = "";
  let createdNotebookTitle = "";
  let uploadedSourceMarkdown = "";
  let uploadedSourceFileName = "";

  const mockServer = http.createServer((req, res) => {
    const url = new URL(req.url ?? "/", `http://${req.headers.host}`);

    if (req.method === "GET" && url.pathname === "/api/v1/books/book-dune") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          id: "book-dune",
          title: "Dune",
          book_type: "ebook",
        })
      );
      return;
    }

    if (req.method === "GET" && url.pathname === "/api/v1/books/book-dune/markdown") {
      res.writeHead(200, {
        "content-type": "text/markdown; charset=utf-8",
        "content-disposition": 'attachment; filename="Dune.md"',
      });
      res.end("# Dune\n\n## Highlights & Notes\n\n> Fear is the mind-killer.\n\n## Chapters\n\n### Chapter 1\n\nArrakis.\n");
      return;
    }

    if (req.method === "POST" && url.pathname === "/webhook-bridge") {
      webhookAuthHeader = req.headers.authorization ?? "";
      let body = "";
      req.on("data", (c) => (body += c));
      req.on("end", () => {
        webhookPayload = JSON.parse(body);
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            notebook_url: "https://notebooklm.google.com/notebook/nb-webhook-1",
            message: 'Pushed "Dune" via webhook bridge.',
          })
        );
      });
      return;
    }

    if (req.method === "POST" && url.pathname === "/v1alpha/projects/proj-1/locations/global/notebooks") {
      let body = "";
      req.on("data", (c) => (body += c));
      req.on("end", () => {
        const parsed = JSON.parse(body);
        createdNotebookTitle = parsed.title;
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            name: "projects/proj-1/locations/global/notebooks/nb-enterprise-42",
            notebookId: "nb-enterprise-42",
          })
        );
      });
      return;
    }

    if (
      req.method === "POST" &&
      url.pathname === "/upload/v1alpha/projects/proj-1/locations/global/notebooks/nb-enterprise-42/sources:uploadFile"
    ) {
      uploadedSourceFileName = String(req.headers["x-goog-upload-file-name"] ?? "");
      let body = "";
      req.on("data", (c) => (body += c));
      req.on("end", () => {
        uploadedSourceMarkdown = body;
        res.writeHead(200, { "content-type": "application/json" });
        res.end(JSON.stringify({ sourceId: "src-1" }));
      });
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

    // 1. Zero-config client_handoff
    const handoffResp = await callRpc("actions/invoke", {
      action: "send_to_notebooklm",
      target: { type: "book", id: "book-dune", title: "Dune" },
      auth: {
        user_id: "u1",
        api_base: baseUrl,
        invocation_token: "tok-1",
        config: {},
      },
    });
    assert.equal(handoffResp.result.status, "client_handoff");
    assert.equal(handoffResp.result.open_url, "https://notebooklm.google.com/");
    assert.equal(handoffResp.result.download_api_path, "/api/v1/books/book-dune/markdown");
    assert.equal(handoffResp.result.file_name, "Dune.md");

    // 2. Mode 1: Webhook push
    const webhookResp = await callRpc("actions/invoke", {
      action: "send_to_notebooklm",
      target: { type: "book", id: "book-dune", title: "Dune" },
      auth: {
        user_id: "u1",
        api_base: baseUrl,
        invocation_token: "tok-1",
        config: {
          webhook_url: `${baseUrl}/webhook-bridge`,
          api_key: "secret-bridge-key",
          allow_local_endpoints: true,
        },
      },
    });
    assert.equal(webhookResp.result.status, "completed");
    assert.equal(webhookResp.result.open_url, "https://notebooklm.google.com/notebook/nb-webhook-1");
    assert.equal(webhookAuthHeader, "Bearer secret-bridge-key");
    assert.equal(webhookPayload.book_id, "book-dune");
    assert.equal(webhookPayload.file_name, "Dune.md");
    assert.match(webhookPayload.content, /Fear is the mind-killer/);

    // 3. Mode 2: Discovery Engine two-step create + uploadFile
    const gcpResp = await callRpc("actions/invoke", {
      action: "send_to_notebooklm",
      target: { type: "book", id: "book-dune", title: "Dune" },
      auth: {
        user_id: "u1",
        api_base: baseUrl,
        invocation_token: "tok-1",
        config: {
          endpoint: baseUrl,
          project_id: "proj-1",
          location: "global",
          api_key: "gcp-oauth-token",
          allow_local_endpoints: true,
        },
      },
    });
    assert.equal(gcpResp.result.status, "completed");
    assert.equal(gcpResp.result.open_url, "https://notebooklm.google.com/notebook/nb-enterprise-42");
    assert.equal(createdNotebookTitle, "Dune");
    assert.equal(uploadedSourceFileName, "Dune.md");
    assert.match(uploadedSourceMarkdown, /Arrakis/);

    // 4. SSRF rejection on metadata IP 169.254.169.254 and [fd00:ec2::254]
    const ssrfResp1 = await callRpc("actions/invoke", {
      action: "send_to_notebooklm",
      target: { type: "book", id: "book-dune" },
      auth: {
        user_id: "u1",
        api_base: baseUrl,
        invocation_token: "tok-1",
        config: {
          webhook_url: "http://169.254.169.254/latest/meta-data",
          allow_local_endpoints: true,
        },
      },
    });
    assert.ok(ssrfResp1.error, "169.254.169.254 must return JSON-RPC error");
    assert.match(ssrfResp1.error.message, /SSRF blocked/);

    const ssrfResp2 = await callRpc("actions/invoke", {
      action: "send_to_notebooklm",
      target: { type: "book", id: "book-dune" },
      auth: {
        user_id: "u1",
        api_base: baseUrl,
        invocation_token: "tok-1",
        config: {
          webhook_url: "http://[fd00:ec2::254]/latest/meta-data",
          allow_local_endpoints: true,
        },
      },
    });
    assert.ok(ssrfResp2.error, "[fd00:ec2::254] must return JSON-RPC error");
    assert.match(ssrfResp2.error.message, /SSRF blocked/);

    // 5. MCP tool send_book_to_notebooklm
    const mcpResp = await callRpc("tools/call", {
      name: "send_book_to_notebooklm",
      arguments: { book_id: "book-dune" },
      auth: {
        user_id: "u1",
        api_base: baseUrl,
        invocation_token: "tok-1",
        config: {},
      },
    });
    assert.ok(!mcpResp.error);
    assert.match(mcpResp.result.content, /Prepared "Dune\.md" for NotebookLM/);

    await callRpc("shutdown", {});
  } finally {
    rl.close();
    child.kill("SIGKILL");
    mockServer.closeAllConnections?.();
    mockServer.close();
  }
});
