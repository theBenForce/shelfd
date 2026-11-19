import { spawn } from "node:child_process";
import fs from "node:fs";
import http from "node:http";
import { createRequire, stripTypeScriptTypes } from "node:module";
import os from "node:os";
import path from "node:path";
import readline from "node:readline";
import { fileURLToPath } from "node:url";
import zlib from "node:zlib";
import type { PluginManifest } from "./types.ts";

const PLUGIN_NAME_RE = /^[a-z][a-z0-9_-]{1,32}$/;
const SEMVER_RE = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;
const VALID_CAPABILITIES = new Set(["tools", "commands", "events", "catalog"]);
const VALID_SCOPES = new Set(["uploads:stage", "uploads:write", "books:read"]);

export function validateManifest(manifest: PluginManifest): string[] {
  const errors: string[] = [];
  if (!manifest || typeof manifest !== "object") {
    return ["plugin.json must be a JSON object"];
  }
  if (typeof manifest.name !== "string" || !PLUGIN_NAME_RE.test(manifest.name)) {
    errors.push(`Invalid "name" (${String(manifest.name)}): must match ^[a-z][a-z0-9_-]{1,32}$`);
  }
  if (typeof manifest.version !== "string" || !SEMVER_RE.test(manifest.version)) {
    errors.push(`Invalid "version" (${String(manifest.version)}): must be valid semver`);
  }
  if (!manifest.entrypoint || typeof manifest.entrypoint.command !== "string" || !manifest.entrypoint.command.trim()) {
    errors.push(`"entrypoint.command" is required (e.g. "node")`);
  } else if (manifest.entrypoint.command.includes("..")) {
    errors.push(`"entrypoint.command" must not contain ".."`);
  }
  for (const cap of manifest.capabilities ?? []) {
    if (!VALID_CAPABILITIES.has(cap)) {
      errors.push(`Unsupported capability "${cap}" (allowed: tools, commands, events, catalog)`);
    }
  }
  for (const scope of manifest.api_scopes ?? []) {
    if (!VALID_SCOPES.has(scope)) {
      errors.push(`Unsupported api_scope "${scope}" (allowed: uploads:stage, uploads:write, books:read)`);
    }
  }
  return errors;
}

function resolveLocalModule(fromDir: string, spec: string): string {
  const base = path.resolve(fromDir, spec);
  const candidates = [
    base,
    `${base}.ts`,
    `${base}.mts`,
    `${base}.js`,
    `${base}.mjs`,
    path.join(base, "index.ts"),
    path.join(base, "index.mjs"),
    path.join(base, "index.js"),
  ];
  // Support TypeScript's ESM convention where `./foo.js` imports `./foo.ts`
  if (spec.endsWith(".js") || spec.endsWith(".mjs")) {
    const stem = base.replace(/\.(m?js)$/, "");
    candidates.unshift(`${stem}.ts`, `${stem}.mts`);
  }
  for (const candidate of candidates) {
    if (fs.existsSync(candidate) && fs.statSync(candidate).isFile()) {
      return candidate;
    }
  }
  throw new Error(`Cannot resolve module "${spec}" from ${fromDir}`);
}

function stripTS(code: string, filePath: string): string {
  if (/\.(m?ts)$/.test(filePath)) {
    return stripTypeScriptTypes(code, { mode: "strip" });
  }
  return code;
}

interface ParsedImport {
  fullMatch: string;
  clause: string;
  specifier: string;
}

function parseStaticImports(code: string): ParsedImport[] {
  const results: ParsedImport[] = [];
  const re = /^\s*import\s+(?!type\s)([\s\S]*?)\s+from\s+["']([^"']+)["']\s*;?/gm;
  let m: RegExpExecArray | null;
  while ((m = re.exec(code)) !== null) {
    results.push({
      fullMatch: m[0],
      clause: m[1].trim(),
      specifier: m[2].trim(),
    });
  }
  return results;
}

function rewriteClauseToDestructure(clause: string, nsVar: string): string {
  if (clause.startsWith("* as ")) {
    const alias = clause.slice("* as ".length).trim();
    return `const ${alias} = ${nsVar};`;
  }
  if (clause.startsWith("{") && clause.endsWith("}")) {
    const inner = clause.slice(1, -1).replace(/\bas\b/g, ":");
    return `const { ${inner} } = ${nsVar};`;
  }
  if (clause.includes(",")) {
    const commaIdx = clause.indexOf(",");
    const defName = clause.slice(0, commaIdx).trim();
    const namedPart = clause.slice(commaIdx + 1).trim();
    const extra = rewriteClauseToDestructure(namedPart, nsVar);
    return `const ${defName} = ${nsVar}.default;\n${extra}`;
  }
  return `const ${clause} = ${nsVar}.default ?? ${nsVar};`;
}

function transformExportsToObject(code: string, exportObjName: string): string {
  const exportedNames: string[] = [];
  let transformed = code;

  // Remove pure type exports left after stripping or `export * from` handled separately
  transformed = transformed.replace(/^\s*export\s+\{\s*\}\s*;?\s*$/gm, "");

  transformed = transformed.replace(
    /^\s*export\s+(async\s+function|function|class|const|let|var)\s+([A-Za-z0-9_$]+)/gm,
    (_match, keyword: string, ident: string) => {
      exportedNames.push(ident);
      return `${keyword} ${ident}`;
    }
  );

  transformed = transformed.replace(/^\s*export\s+\{([^}]+)\}\s*;?/gm, (_match, list: string) => {
    const lines: string[] = [];
    for (const part of list.split(",")) {
      const trimmed = part.trim();
      if (!trimmed) continue;
      const [local, alias] = trimmed.split(/\s+as\s+/).map((s) => s.trim());
      lines.push(`${exportObjName}.${alias || local} = ${local};`);
    }
    return lines.join("\n");
  });

  transformed = transformed.replace(/^\s*export\s+default\s+/gm, `${exportObjName}.default = `);

  const footer = exportedNames.map((n) => `${exportObjName}.${n} = ${n};`).join("\n");
  return `const ${exportObjName} = {};\n${transformed}\n${footer}`;
}

/**
 * Bundles a TypeScript/ESM Shelfd plugin into a standalone single-file `dist/index.mjs`.
 * Uses `esbuild` automatically if installed in the plugin workspace, otherwise uses the
 * built-in zero-dependency bundler backed by Node's native `stripTypeScriptTypes`.
 */
export async function buildPlugin(pluginDir: string): Promise<{ outFile: string; manifest: PluginManifest }> {
  const absDir = path.resolve(pluginDir);
  const manifestPath = path.join(absDir, "plugin.json");
  if (!fs.existsSync(manifestPath)) {
    throw new Error(`Missing plugin.json in ${absDir}`);
  }
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8")) as PluginManifest;
  const errs = validateManifest(manifest);
  if (errs.length > 0) {
    throw new Error(`Invalid plugin.json:\n  - ${errs.join("\n  - ")}`);
  }

  const entryCandidates = ["src/index.ts", "src/index.mts", "src/index.mjs", "src/index.js", "index.ts", "index.mjs"];
  const entryRel = entryCandidates.find((rel) => fs.existsSync(path.join(absDir, rel)));
  if (!entryRel) {
    throw new Error(`Could not find plugin source entrypoint (looked for ${entryCandidates.join(", ")})`);
  }
  const entryFile = path.join(absDir, entryRel);
  const outFile = path.join(absDir, "dist", "index.mjs");
  fs.mkdirSync(path.dirname(outFile), { recursive: true });

  // Try esbuild first if present in external repositories with third-party npm packages
  try {
    const req = createRequire(path.join(absDir, "package.json"));
    const esbuild = req("esbuild") as {
      build: (opts: Record<string, unknown>) => Promise<unknown>;
    };
    await esbuild.build({
      entryPoints: [entryFile],
      outfile: outFile,
      bundle: true,
      platform: "node",
      format: "esm",
      target: "node22",
      logLevel: "silent",
    });
    return { outFile, manifest };
  } catch {
    // Fall back to built-in zero-dependency ESM + stripTypeScriptTypes bundler
  }

  const sdkDistIndex = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "index.mjs");
  const nodeBuiltinImports = new Map<string, Set<string>>();
  const nodeNamespaceImports = new Map<string, string>();
  const visited = new Map<string, string>();
  const chunks: string[] = [];
  let modCounter = 0;

  const bundleModule = (filePath: string, isEntry: boolean): string => {
    const real = fs.realpathSync(filePath);
    const existingVar = visited.get(real);
    if (existingVar) {
      return existingVar;
    }

    modCounter += 1;
    const modVar = `__shelfd_mod_${modCounter}`;
    visited.set(real, modVar);

    let code = stripTS(fs.readFileSync(real, "utf8"), real);
    // Strip re-exports of pure types or empty type modules
    code = code.replace(/^\s*export\s+\*\s+from\s+["']\.\/types(?:\.ts|\.js)?["']\s*;?\s*$/gm, "");

    const imports = parseStaticImports(code);
    for (const imp of imports) {
      if (imp.specifier.startsWith("node:") || ["fs", "path", "os", "http", "https", "url", "util", "readline", "stream", "crypto", "zlib"].includes(imp.specifier)) {
        const normSpec = imp.specifier.startsWith("node:") ? imp.specifier : `node:${imp.specifier}`;
        const aliasVar = `__node_${normSpec.replace(/[^a-zA-Z0-9]/g, "_")}`;
        if (!nodeNamespaceImports.has(normSpec)) {
          nodeNamespaceImports.set(normSpec, aliasVar);
        }
        if (!nodeBuiltinImports.has(normSpec)) {
          nodeBuiltinImports.set(normSpec, new Set());
        }
        code = code.replace(imp.fullMatch, rewriteClauseToDestructure(imp.clause, aliasVar));
        continue;
      }

      let depPath: string;
      if (imp.specifier === "@shelfd/plugin-sdk") {
        depPath = sdkDistIndex;
      } else if (imp.specifier.startsWith("./") || imp.specifier.startsWith("../")) {
        depPath = resolveLocalModule(path.dirname(real), imp.specifier);
      } else {
        throw new Error(
          `External package import "${imp.specifier}" in ${real} requires "esbuild" in devDependencies to bundle into single-file dist/index.mjs.`
        );
      }

      const depVar = bundleModule(depPath, false);
      code = code.replace(imp.fullMatch, rewriteClauseToDestructure(imp.clause, depVar));
    }

    if (isEntry) {
      chunks.push(`// --- entry: ${path.relative(absDir, real)} ---\n${code}`);
    } else {
      const wrapped = transformExportsToObject(code, modVar);
      chunks.push(`// --- module: ${path.basename(real)} ---\nconst ${modVar} = (() => {\n${wrapped}\nreturn ${modVar};\n})();`);
    }

    return modVar;
  };

  bundleModule(entryFile, true);

  const headerLines = [
    "// Generated by @shelfd/plugin-sdk (standalone zero-dependency ESM bundle)",
    ...[...nodeNamespaceImports.entries()].map(([spec, alias]) => `import * as ${alias} from "${spec}";`),
    "",
  ];

  fs.writeFileSync(outFile, `${headerLines.join("\n")}${chunks.join("\n\n")}\n`, "utf8");
  return { outFile, manifest };
}

// CRC-32 lookup table for pure-Node ZIP creation
const CRC32_TABLE = (() => {
  const table = new Uint32Array(256);
  for (let i = 0; i < 256; i++) {
    let c = i;
    for (let k = 0; k < 8; k++) {
      c = (c & 1) ? (0xedb88320 ^ (c >>> 1)) : (c >>> 1);
    }
    table[i] = c >>> 0;
  }
  return table;
})();

function crc32(buf: Buffer): number {
  let crc = 0xffffffff;
  for (let i = 0; i < buf.length; i++) {
    crc = CRC32_TABLE[(crc ^ buf[i]) & 0xff] ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}

export function createZipArchive(entries: Array<{ name: string; data: Buffer }>): Buffer {
  const localParts: Buffer[] = [];
  const centralParts: Buffer[] = [];
  let offset = 0;

  for (const entry of entries) {
    const nameBuf = Buffer.from(entry.name.replace(/\\/g, "/"), "utf8");
    const uncompressed = entry.data;
    const compressed = zlib.deflateRawSync(uncompressed);
    const crc = crc32(uncompressed);

    const localHeader = Buffer.alloc(30 + nameBuf.length);
    localHeader.writeUInt32LE(0x04034b50, 0);
    localHeader.writeUInt16LE(20, 4);
    localHeader.writeUInt16LE(0, 6);
    localHeader.writeUInt16LE(8, 8); // deflate
    localHeader.writeUInt16LE(0, 10);
    localHeader.writeUInt16LE(0, 12);
    localHeader.writeUInt32LE(crc, 14);
    localHeader.writeUInt32LE(compressed.length, 18);
    localHeader.writeUInt32LE(uncompressed.length, 22);
    localHeader.writeUInt16LE(nameBuf.length, 26);
    localHeader.writeUInt16LE(0, 28);
    nameBuf.copy(localHeader, 30);

    localParts.push(localHeader, compressed);

    const centralHeader = Buffer.alloc(46 + nameBuf.length);
    centralHeader.writeUInt32LE(0x02014b50, 0);
    centralHeader.writeUInt16LE(20, 4);
    centralHeader.writeUInt16LE(20, 6);
    centralHeader.writeUInt16LE(0, 8);
    centralHeader.writeUInt16LE(8, 10);
    centralHeader.writeUInt16LE(0, 12);
    centralHeader.writeUInt16LE(0, 14);
    centralHeader.writeUInt32LE(crc, 16);
    centralHeader.writeUInt32LE(compressed.length, 20);
    centralHeader.writeUInt32LE(uncompressed.length, 24);
    centralHeader.writeUInt16LE(nameBuf.length, 28);
    centralHeader.writeUInt16LE(0, 30);
    centralHeader.writeUInt16LE(0, 32);
    centralHeader.writeUInt16LE(0, 34);
    centralHeader.writeUInt16LE(0, 36);
    centralHeader.writeUInt32LE(0, 38);
    centralHeader.writeUInt32LE(offset, 42);
    nameBuf.copy(centralHeader, 46);

    centralParts.push(centralHeader);
    offset += localHeader.length + compressed.length;
  }

  const centralSize = centralParts.reduce((acc, b) => acc + b.length, 0);
  const eocd = Buffer.alloc(22);
  eocd.writeUInt32LE(0x06054b50, 0);
  eocd.writeUInt16LE(0, 4);
  eocd.writeUInt16LE(0, 6);
  eocd.writeUInt16LE(entries.length, 8);
  eocd.writeUInt16LE(entries.length, 10);
  eocd.writeUInt32LE(centralSize, 12);
  eocd.writeUInt32LE(offset, 16);
  eocd.writeUInt16LE(0, 20);

  return Buffer.concat([...localParts, ...centralParts, eocd]);
}

export async function packPlugin(pluginDir: string): Promise<string> {
  const absDir = path.resolve(pluginDir);
  const { manifest, outFile } = await buildPlugin(absDir);
  const zipPath = path.join(absDir, "dist", `${manifest.name}-${manifest.version}.zip`);

  const zipBytes = createZipArchive([
    { name: "plugin.json", data: fs.readFileSync(path.join(absDir, "plugin.json")) },
    { name: "dist/index.mjs", data: fs.readFileSync(outFile) },
  ]);
  fs.writeFileSync(zipPath, zipBytes);
  return zipPath;
}

export interface VerifyCheckResult {
  name: string;
  passed: boolean;
  detail?: string;
}

export interface VerifyReport {
  passed: boolean;
  pluginName: string;
  checks: VerifyCheckResult[];
}

/**
 * Standalone contract, protocol, and secret-hygiene verification harness for Shelfd plugins.
 */
export async function verifyPlugin(pluginDir: string): Promise<VerifyReport> {
  const absDir = path.resolve(pluginDir);
  const checks: VerifyCheckResult[] = [];
  const manifestPath = path.join(absDir, "plugin.json");

  if (!fs.existsSync(manifestPath)) {
    return {
      passed: false,
      pluginName: path.basename(absDir),
      checks: [{ name: "manifest_exists", passed: false, detail: `Missing ${manifestPath}` }],
    };
  }

  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8")) as PluginManifest;
  const manifestErrs = validateManifest(manifest);
  checks.push({
    name: "manifest_valid",
    passed: manifestErrs.length === 0,
    detail: manifestErrs.length === 0 ? `${manifest.name}@${manifest.version}` : manifestErrs.join("; "),
  });
  if (manifestErrs.length > 0) {
    return { passed: false, pluginName: manifest.name || "unknown", checks };
  }

  // Mock Shelfd API + deterministic upstream mock catalog for offline verification
  let stagedUploads = 0;
  const mockServer = http.createServer((req, res) => {
    const url = req.url ?? "/";
    if (req.method === "POST" && url === "/api/v1/books/upload/stage") {
      const auth = req.headers.authorization ?? "";
      if (!auth.startsWith("Bearer verify-scoped-token")) {
        res.writeHead(401, { "content-type": "application/json" });
        res.end(JSON.stringify({ error: "Missing or invalid scoped invocation token" }));
        return;
      }
      const chunks: Buffer[] = [];
      req.on("data", (c: Buffer) => chunks.push(c));
      req.on("end", () => {
        stagedUploads += 1;
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            job_id: `verify-job-${stagedUploads}`,
            status: "staged",
            filename: "verified.epub",
            has_cover: true,
            metadata: { title: "Verified Book", authors: ["Test Author"] },
          })
        );
      });
      return;
    }

    if (url.startsWith("/mock-gutendex/books")) {
      const addr = mockServer.address();
      const port = typeof addr === "object" && addr ? addr.port : 0;
      const base = `http://127.0.0.1:${port}`;
      if (url.match(/^\/mock-gutendex\/books\/\d+/)) {
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            id: 84,
            title: "Frankenstein; Or, The Modern Prometheus",
            authors: [{ name: "Shelley, Mary Wollstonecraft" }],
            subjects: ["Science fiction", "Monsters -- Fiction"],
            bookshelves: ["Gothic Fiction", "Science Fiction"],
            languages: ["en"],
            formats: {
              "application/epub+zip": `${base}/mock-gutendex/files/84.epub`,
              "image/jpeg": `${base}/mock-gutendex/files/84.jpg`,
            },
          })
        );
        return;
      }
      res.writeHead(200, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          count: 1,
          next: null,
          results: [
            {
              id: 84,
              title: "Frankenstein; Or, The Modern Prometheus",
              authors: [{ name: "Shelley, Mary Wollstonecraft" }],
              subjects: ["Science fiction"],
              bookshelves: ["Science Fiction"],
              languages: ["en"],
              formats: {
                "application/epub+zip": `${base}/mock-gutendex/files/84.epub`,
                "image/jpeg": `${base}/mock-gutendex/files/84.jpg`,
              },
            },
          ],
        })
      );
      return;
    }

    if (url.startsWith("/mock-gutendex/files/84.epub")) {
      res.writeHead(200, { "content-type": "application/epub+zip" });
      res.end(Buffer.from("PK\x03\x04mock-epub-bytes"));
      return;
    }
    if (url.startsWith("/mock-gutendex/files/84.jpg")) {
      res.writeHead(200, { "content-type": "image/jpeg" });
      res.end(Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10]));
      return;
    }

    res.writeHead(404);
    res.end("not found");
  });

  await new Promise<void>((resolve) => mockServer.listen(0, "127.0.0.1", () => resolve()));
  const serverAddr = mockServer.address();
  const port = typeof serverAddr === "object" && serverAddr ? serverAddr.port : 0;
  const mockApiBase = `http://127.0.0.1:${port}`;

  const scratchDir = fs.mkdtempSync(path.join(os.tmpdir(), "shelfd-verify-"));
  const canarySecret = "SHELFD_VERIFY_CANARY_SECRET_991122";
  const stderrLines: string[] = [];
  let malformedStdoutFrames = 0;

  try {
    const cmd = manifest.entrypoint!.command;
    const args = manifest.entrypoint!.args ?? [];

    const child = spawn(cmd, args, {
      cwd: absDir,
      shell: false,
      stdio: ["pipe", "pipe", "pipe"],
      env: {
        PATH: process.env.PATH ?? "",
        HOME: process.env.HOME ?? scratchDir,
        TMPDIR: os.tmpdir(),
        NO_PROXY: "127.0.0.1,localhost",
        no_proxy: "127.0.0.1,localhost",
        NODE_NO_WARNINGS: "1",
        SHELFD_DATA_DIR: scratchDir,
      },
    });

    const pending = new Map<string, { resolve: (msg: Record<string, unknown>) => void; reject: (err: Error) => void }>();
    let reqCounter = 0;

    const stdoutRl = readline.createInterface({ input: child.stdout, crlfDelay: Infinity });
    stdoutRl.on("line", (line) => {
      if (!line.trim()) return;
      try {
        const parsed = JSON.parse(line) as Record<string, unknown>;
        if (parsed.jsonrpc !== "2.0") {
          malformedStdoutFrames += 1;
          return;
        }
        if (parsed.id !== undefined && !parsed.method) {
          const waiter = pending.get(String(parsed.id));
          if (waiter) {
            pending.delete(String(parsed.id));
            waiter.resolve(parsed);
          }
        } else if (parsed.method === "config/patch" && parsed.id !== undefined) {
          child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id: parsed.id, result: { ok: true } })}\n`);
        }
      } catch {
        malformedStdoutFrames += 1;
      }
    });

    const stderrRl = readline.createInterface({ input: child.stderr, crlfDelay: Infinity });
    stderrRl.on("line", (line) => {
      stderrLines.push(line);
    });

    const callRpc = (method: string, params: Record<string, unknown>, timeoutMs = 10_000): Promise<Record<string, unknown>> => {
      reqCounter += 1;
      const id = String(reqCounter);
      return new Promise((resolve, reject) => {
        const t = setTimeout(() => {
          pending.delete(id);
          reject(new Error(`RPC ${method} timed out after ${timeoutMs}ms`));
        }, timeoutMs);
        pending.set(id, {
          resolve: (msg) => {
            clearTimeout(t);
            resolve(msg);
          },
          reject: (err) => {
            clearTimeout(t);
            reject(err);
          },
        });
        child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
      });
    };

    // 1. Handshake check
    const initResp = await callRpc("initialize", {
      protocol_version: "2026-11-01",
      shelfd_version: "0.1.0-verify",
      api_base: mockApiBase,
      data_dir: scratchDir,
    });
    const initResult = (initResp.result ?? {}) as Record<string, unknown>;
    const handshakeOk = !initResp.error && initResult.name === manifest.name && initResult.version === manifest.version;
    checks.push({
      name: "handshake_initialize",
      passed: handshakeOk,
      detail: handshakeOk ? `Initialized ${String(initResult.name)}@${String(initResult.version)}` : JSON.stringify(initResp.error ?? initResult),
    });

    // Build mock config injecting canary secret into secret fields + mock base_url if supported
    const testConfig: Record<string, unknown> = {};
    const props = (manifest.config_schema?.properties ?? {}) as Record<string, Record<string, unknown>>;
    for (const [key, prop] of Object.entries(props)) {
      if (prop["x-shelfd-secret"] === true || prop.format === "password") {
        testConfig[key] = canarySecret;
      } else if (key === "base_url" || key === "api_url") {
        testConfig[key] = `${mockApiBase}/mock-gutendex`;
      }
    }

    const authEnvelope = {
      user_id: "verify-user-1",
      invocation_token: "verify-scoped-token",
      api_base: mockApiBase,
      config: testConfig,
    };

    // 2. If catalog plugin supports mock base_url (e.g. project-gutenberg), run full browse/search/stage E2E!
    if ((manifest.capabilities ?? []).includes("catalog") && ("base_url" in props || "api_url" in props)) {
      const searchResp = await callRpc("catalog/search", {
        query: "frankenstein",
        auth: authEnvelope,
      });
      const searchRes = (searchResp.result ?? {}) as { items?: Array<{ external_id: string }> };
      const items = searchRes.items ?? [];
      checks.push({
        name: "catalog_search",
        passed: !searchResp.error && items.length > 0 && Boolean(items[0].external_id),
        detail: searchResp.error ? JSON.stringify(searchResp.error) : `Returned ${items.length} item(s)`,
      });

      if (items.length > 0) {
        const stageResp = await callRpc("catalog/stage", {
          items: [{ external_id: items[0].external_id }],
          auth: authEnvelope,
        });
        const stageRes = (stageResp.result ?? {}) as { results?: Array<{ status: string; upload_job_id?: string }> };
        const firstStaged = stageRes.results?.[0];
        const stageOk = !stageResp.error && firstStaged?.status === "staged" && stagedUploads > 0;
        checks.push({
          name: "catalog_stage_loopback",
          passed: Boolean(stageOk),
          detail: stageOk ? `Staged upload_job_id=${String(firstStaged?.upload_job_id)}` : JSON.stringify(stageResp.error ?? stageRes),
        });
      }
    }

    // 3. Shutdown check
    const exitPromise = new Promise<number | null>((resolve) => {
      child.once("exit", (code) => resolve(code));
    });
    const shutResp = await callRpc("shutdown", {});
    const exitCode = await Promise.race([
      exitPromise,
      new Promise<number | null>((resolve) => setTimeout(() => resolve(-1), 4000)),
    ]);
    if (exitCode === -1) {
      child.kill("SIGKILL");
    }
    checks.push({
      name: "graceful_shutdown",
      passed: !shutResp.error && exitCode === 0,
      detail: `exitCode=${String(exitCode)}`,
    });

    // 4. Stdout NDJSON frame integrity check
    checks.push({
      name: "stdout_ndjson_purity",
      passed: malformedStdoutFrames === 0,
      detail: malformedStdoutFrames === 0 ? "All stdout lines were valid JSON-RPC 2.0 frames" : `${malformedStdoutFrames} non-JSON-RPC line(s) written to stdout`,
    });

    // 5. Secret non-leakage on stderr check
    const leakedInStderr = stderrLines.some((l) => l.includes(canarySecret));
    checks.push({
      name: "stderr_secret_hygiene",
      passed: !leakedInStderr,
      detail: leakedInStderr ? "CRITICAL: Secret config value leaked to stderr!" : "No secret values found in stderr",
    });
  } finally {
    mockServer.close();
    fs.rmSync(scratchDir, { recursive: true, force: true });
  }

  return {
    passed: checks.every((c) => c.passed),
    pluginName: manifest.name,
    checks,
  };
}

export async function runCli(argv: string[] = process.argv.slice(2)): Promise<void> {
  const [subcommand, targetArg] = argv;
  const targetDir = path.resolve(targetArg || ".");

  if (subcommand === "build") {
    const { outFile, manifest } = await buildPlugin(targetDir);
    process.stdout.write(`Built ${manifest.name}@${manifest.version} -> ${path.relative(process.cwd(), outFile)}\n`);
    return;
  }

  if (subcommand === "pack") {
    const zipFile = await packPlugin(targetDir);
    process.stdout.write(`Packaged plugin archive -> ${path.relative(process.cwd(), zipFile)}\n`);
    return;
  }

  if (subcommand === "verify") {
    const report = await verifyPlugin(targetDir);
    for (const check of report.checks) {
      const badge = check.passed ? "PASS" : "FAIL";
      process.stdout.write(`[${badge}] ${check.name}${check.detail ? ` — ${check.detail}` : ""}\n`);
    }
    if (!report.passed) {
      process.stderr.write(`Verification FAILED for plugin "${report.pluginName}"\n`);
      process.exit(1);
    }
    process.stdout.write(`Verification PASSED for plugin "${report.pluginName}"\n`);
    return;
  }

  process.stderr.write(
    "Usage: shelfd-plugin <command> [plugin-dir]\n\nCommands:\n  build [dir]   Bundle TypeScript plugin into single-file dist/index.mjs\n  verify [dir]  Run protocol, loopback staging, and secret-hygiene checks\n  pack [dir]    Build and package plugin into dist/<name>-<version>.zip\n"
  );
  process.exit(subcommand === "--help" || subcommand === "-h" ? 0 : 1);
}

const isMain = process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url));
if (isMain) {
  runCli().catch((err: unknown) => {
    process.stderr.write(`${err instanceof Error ? err.message : String(err)}\n`);
    process.exit(1);
  });
}
