#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const pkgRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const distCli = path.join(pkgRoot, "dist", "cli.mjs");

if (!fs.existsSync(distCli)) {
  await import(pathToFileURL(path.join(pkgRoot, "scripts", "build-sdk.mjs")).href);
}

const cliMod = await import(pathToFileURL(distCli).href);
await cliMod.runCli(process.argv.slice(2));
