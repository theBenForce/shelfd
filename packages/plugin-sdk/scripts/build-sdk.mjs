import fs from "node:fs";
import { stripTypeScriptTypes } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const pkgRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const checkOnly = process.argv.includes("--check");

const typesSrc = fs.readFileSync(path.join(pkgRoot, "src", "types.ts"), "utf8");
const indexSrc = fs.readFileSync(path.join(pkgRoot, "src", "index.ts"), "utf8");
const cliSrc = fs.readFileSync(path.join(pkgRoot, "src", "cli.ts"), "utf8");

const strippedTypes = stripTypeScriptTypes(typesSrc, { mode: "strip" });
const strippedIndex = stripTypeScriptTypes(indexSrc, { mode: "strip" })
  .replace(/^\s*import\s+type\s+[\s\S]*?from\s+["']\.\/types\.ts["'];?\s*$/gm, "")
  .replace(/^\s*export\s+\*\s+from\s+["']\.\/types\.ts["'];?\s*$/gm, strippedTypes.trim());

const strippedCli = stripTypeScriptTypes(cliSrc, { mode: "strip" })
  .replace(/^\s*import\s+type\s+[\s\S]*?from\s+["']\.\/types\.ts["'];?\s*$/gm, "");

if (checkOnly) {
  process.stdout.write("@shelfd/plugin-sdk syntax & type-strip check OK\n");
  process.exit(0);
}

const distDir = path.join(pkgRoot, "dist");
fs.mkdirSync(distDir, { recursive: true });

function writeFileAtomic(destPath, content) {
  const tmpPath = `${destPath}.tmp.${process.pid}.${Math.random().toString(36).slice(2)}`;
  fs.writeFileSync(tmpPath, content, "utf8");
  fs.renameSync(tmpPath, destPath);
}

writeFileAtomic(path.join(distDir, "index.mjs"), `${strippedIndex.trim()}\n`);
writeFileAtomic(path.join(distDir, "cli.mjs"), `${strippedCli.trim()}\n`);
writeFileAtomic(
  path.join(distDir, "index.d.ts"),
  `${typesSrc}\nexport declare class AuthExpiredError extends Error {\n  readonly code = -32001;\n  constructor(message?: string);\n}\nexport declare class PluginRPCError extends Error {\n  readonly code: number;\n  readonly data?: unknown;\n  constructor(code: number, message: string, data?: unknown);\n}\nexport declare function truncateForMCP(text: string, maxChars?: number): string;\nexport declare function createPoliteFetcher(options?: PoliteFetcherOptions): typeof fetch;\nexport declare function readResponseBytesWithProgress(response: Response, onProgress?: (info: DownloadProgressInfo) => void, options?: DownloadProgressOptions): Promise<Uint8Array>;\nexport declare function definePlugin<TConfig extends Record<string, unknown> = Record<string, unknown>>(definition: PluginDefinition<TConfig>): PluginDefinition<TConfig>;\n`
);
fs.chmodSync(path.join(pkgRoot, "cli", "shelfd-plugin.mjs"), 0o755);
