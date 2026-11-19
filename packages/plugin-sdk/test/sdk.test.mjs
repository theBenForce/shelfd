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

test("validateManifest catches invalid names, versions, and scopes", () => {
  assert.equal(
    validateManifest({
      name: "valid-plugin",
      version: "1.0.0",
      entrypoint: { command: "node", args: ["dist/index.mjs"] },
      capabilities: ["catalog", "tools"],
      api_scopes: ["uploads:stage"],
    }).length,
    0
  );

  const errs = validateManifest({
    name: "UPPERCASE_BAD",
    version: "not-semver",
    entrypoint: { command: "../bad" },
    capabilities: ["root"],
    api_scopes: ["admin:all"],
  });
  assert.equal(errs.length, 5);
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
