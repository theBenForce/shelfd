import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const pluginRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

test("project-gutenberg bundle is a standalone single-file ESM artifact", () => {
  const distPath = path.join(pluginRoot, "dist", "index.mjs");
  assert.ok(fs.existsSync(distPath), "dist/index.mjs must exist after build");
  const content = fs.readFileSync(distPath, "utf8");
  assert.doesNotMatch(content, /from\s+["']@shelfd\/plugin-sdk["']/, "dist/index.mjs must inline @shelfd/plugin-sdk so no node_modules is needed at runtime");
});
