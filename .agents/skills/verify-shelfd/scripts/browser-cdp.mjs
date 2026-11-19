#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const CDP_PORT = process.env.SHELFD_VERIFY_CDP_PORT || "19222";
const WEB_URL = (process.env.SHELFD_VERIFY_WEB_URL || "http://127.0.0.1:18080").replace(/\/$/, "");

const CONSOLE_HOOK_SCRIPT = `
(() => {
  if (window.__shelfdConsoleInstalled) return;
  window.__shelfdConsoleInstalled = true;
  window.__shelfdConsole = window.__shelfdConsole || { errors: [], warnings: [], logs: [] };
  const serialize = (args) => args.map((a) => {
    if (a instanceof Error) return a.stack || a.message;
    if (typeof a === "object" && a !== null) {
      try { return JSON.stringify(a); } catch { return String(a); }
    }
    return String(a);
  }).join(" ");
  const origErr = console.error.bind(console);
  const origWarn = console.warn.bind(console);
  const origLog = console.log.bind(console);
  console.error = (...args) => {
    window.__shelfdConsole.errors.push({ ts: new Date().toISOString(), type: "console.error", text: serialize(args) });
    origErr(...args);
  };
  console.warn = (...args) => {
    window.__shelfdConsole.warnings.push({ ts: new Date().toISOString(), type: "console.warn", text: serialize(args) });
    origWarn(...args);
  };
  console.log = (...args) => {
    if (window.__shelfdConsole.logs.length < 200) {
      window.__shelfdConsole.logs.push({ ts: new Date().toISOString(), type: "console.log", text: serialize(args) });
    }
    origLog(...args);
  };
  window.addEventListener("error", (ev) => {
    window.__shelfdConsole.errors.push({
      ts: new Date().toISOString(),
      type: "window.error",
      text: ev.message || String(ev.error || ev),
    });
  });
  window.addEventListener("unhandledrejection", (ev) => {
    window.__shelfdConsole.errors.push({
      ts: new Date().toISOString(),
      type: "unhandledrejection",
      text: String(ev.reason?.stack || ev.reason || ev),
    });
  });
})();
`;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function parseArgs(argv) {
  const positional = [];
  const flags = {};
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg.startsWith("--")) {
      const eq = arg.indexOf("=");
      if (eq !== -1) {
        flags[arg.slice(2, eq)] = arg.slice(eq + 1);
      } else if (i + 1 < argv.length && !argv[i + 1].startsWith("--")) {
        flags[arg.slice(2)] = argv[++i];
      } else {
        flags[arg.slice(2)] = true;
      }
    } else {
      positional.push(arg);
    }
  }
  return { positional, flags };
}

async function openCdpSocket(wsUrl) {
  const ws = new WebSocket(wsUrl);
  await new Promise((resolve, reject) => {
    ws.addEventListener("open", resolve, { once: true });
    ws.addEventListener("error", reject, { once: true });
  });

  let nextId = 1;
  const pending = new Map();
  ws.addEventListener("message", (ev) => {
    const msg = JSON.parse(String(ev.data));
    if (msg.id && pending.has(msg.id)) {
      const { resolve, reject } = pending.get(msg.id);
      pending.delete(msg.id);
      if (msg.error) {
        reject(new Error(`${msg.error.message} (${msg.error.code})`));
      } else {
        resolve(msg.result);
      }
    }
  });

  const call = (method, params = {}) =>
    new Promise((resolve, reject) => {
      const id = nextId++;
      pending.set(id, { resolve, reject });
      ws.send(JSON.stringify({ id, method, params }));
    });

  return { ws, call, close: () => ws.close() };
}

async function listCdpTargets() {
  const listUrl = `http://127.0.0.1:${CDP_PORT}/json/list`;
  try {
    const res = await fetch(listUrl);
    return await res.json();
  } catch (err) {
    throw new Error(`Cannot reach Chromium CDP on 127.0.0.1:${CDP_PORT} (${err.message}). Run 'control-shelfd launch' first.`);
  }
}

async function connectPage() {
  const targets = await listCdpTargets();
  const page =
    targets.find((t) => t.type === "page" && t.url.startsWith(WEB_URL)) ||
    targets.find((t) => t.type === "page" && !t.url.startsWith("devtools://"));
  if (!page || !page.webSocketDebuggerUrl) {
    throw new Error(`No inspectable browser page target found on 127.0.0.1:${CDP_PORT}`);
  }

  const { ws, call, close } = await openCdpSocket(page.webSocketDebuggerUrl);
  await call("Page.enable");
  await call("Runtime.enable");
  await call("DOM.enable");
  await call("Accessibility.enable");
  await call("Runtime.evaluate", { expression: CONSOLE_HOOK_SCRIPT });

  return { ws, call, close };
}

async function ensureFlutterSemantics(call, timeoutMs = 8000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const check = await call("Runtime.evaluate", {
      expression: `(() => {
        const selector = "flt-semantics, input[data-semantics-role], [flt-semantics-identifier]";
        let semCount = document.querySelectorAll(selector).length;
        let placeholder = document.querySelector("flt-semantics-placeholder, [aria-label='Enable accessibility']");
        for (const h of document.querySelectorAll("flutter-view, flt-glass-pane")) {
          if (h.shadowRoot) {
            semCount += h.shadowRoot.querySelectorAll(selector).length;
            if (!placeholder) {
              placeholder = h.shadowRoot.querySelector("flt-semantics-placeholder, [aria-label='Enable accessibility']");
            }
          }
        }
        if (semCount === 0 && placeholder) {
          placeholder.click();
          return { semCount: 0, clickedPlaceholder: true };
        }
        return { semCount, clickedPlaceholder: false };
      })()`,
      returnByValue: true,
    });
    const val = check.result?.value || { semCount: 0 };
    if (val.semCount > 0) {
      return;
    }
    await sleep(300);
  }
}

function cleanAXNodes(nodes) {
  const skipRoles = new Set(["none", "generic", "InlineTextBox", "LineBreak", "Canvas"]);
  const out = [];
  for (const n of nodes) {
    if (n.ignored) continue;
    const role = n.role?.value || "";
    if (!role || skipRoles.has(role)) continue;
    const name = (n.name?.value ?? "").trim();
    const value = n.value?.value !== undefined ? String(n.value.value) : undefined;
    const props = {};
    for (const p of n.properties || []) {
      if (["checked", "selected", "disabled", "expanded", "focused", "level"].includes(p.name)) {
        props[p.name] = p.value?.value;
      }
    }
    if (!name && value === undefined && !["textbox", "slider", "progressbar", "checkbox", "switch"].includes(role)) {
      continue;
    }
    out.push({
      nodeId: n.nodeId,
      backendDOMNodeId: n.backendDOMNodeId,
      role,
      name,
      value,
      ...props,
    });
  }
  return out;
}

function formatAXSnapshot(url, nodes) {
  const lines = [`# ARIA Snapshot (${url})`];
  for (const n of nodes) {
    const parts = [`- ${n.role}`];
    if (n.name) parts.push(JSON.stringify(n.name));
    if (n.value !== undefined && n.value !== "") parts.push(`value=${JSON.stringify(n.value)}`);
    if (n.checked !== undefined) parts.push(`checked=${n.checked}`);
    if (n.selected) parts.push("selected");
    if (n.disabled) parts.push("disabled");
    if (n.expanded !== undefined) parts.push(`expanded=${n.expanded}`);
    lines.push(parts.join(" "));
  }
  return lines.join("\n") + "\n";
}

async function getCurrentUrl(call) {
  const r = await call("Runtime.evaluate", { expression: "window.location.href", returnByValue: true });
  return r.result?.value || "";
}

async function findMatchingNode(call, { role, name, exact = false, index = 0, timeoutMs = 8000 }) {
  const deadline = Date.now() + timeoutMs;
  const targetRole = role ? role.toLowerCase() : null;
  const targetName = name !== undefined ? String(name) : null;

  while (Date.now() < deadline) {
    await ensureFlutterSemantics(call, 1500);
    const ax = await call("Accessibility.getFullAXTree");
    const cleaned = cleanAXNodes(ax.nodes || []);
    const matches = cleaned.filter((n) => {
      if (n.disabled) return false;
      if (targetRole && n.role.toLowerCase() !== targetRole) return false;
      if (targetName !== null) {
        if (exact) {
          return n.name === targetName;
        }
        return n.name.toLowerCase().includes(targetName.toLowerCase());
      }
      return true;
    });
    if (matches.length > index) {
      return matches[index];
    }
    await sleep(300);
  }

  const ax = await call("Accessibility.getFullAXTree");
  const available = cleanAXNodes(ax.nodes || [])
    .slice(0, 40)
    .map((n) => `${n.role}(${JSON.stringify(n.name)})`)
    .join(", ");
  throw new Error(
    `Timed out after ${timeoutMs}ms waiting for AX node role=${role || "*"} name=${JSON.stringify(name)} (exact=${exact}). Visible nodes: ${available}`
  );
}

async function clickNode(call, node) {
  if (!node.backendDOMNodeId) {
    throw new Error(`Matched AX node ${node.role} ${JSON.stringify(node.name)} has no backendDOMNodeId`);
  }
  try {
    await call("DOM.scrollIntoViewIfNeeded", { backendNodeId: node.backendDOMNodeId });
  } catch {}

  let clickedViaMouse = false;
  try {
    const quads = await call("DOM.getContentQuads", { backendNodeId: node.backendDOMNodeId });
    if (quads.quads && quads.quads.length > 0) {
      const q = quads.quads[0];
      const x = (q[0] + q[2] + q[4] + q[6]) / 4;
      const y = (q[1] + q[3] + q[5] + q[7]) / 4;
      if (x > 0 && y > 0) {
        await call("Input.dispatchMouseEvent", { type: "mouseMoved", x, y });
        await call("Input.dispatchMouseEvent", { type: "mousePressed", x, y, button: "left", clickCount: 1 });
        await call("Input.dispatchMouseEvent", { type: "mouseReleased", x, y, button: "left", clickCount: 1 });
        clickedViaMouse = true;
      }
    }
  } catch {}

  if (!clickedViaMouse) {
    const resolved = await call("DOM.resolveNode", { backendNodeId: node.backendDOMNodeId });
    await call("Runtime.callFunctionOn", {
      objectId: resolved.object.objectId,
      functionDeclaration: "function() { this.click(); }",
    });
  }
}

const KEY_DEFS = {
  Enter: { key: "Enter", code: "Enter", windowsVirtualKeyCode: 13 },
  Escape: { key: "Escape", code: "Escape", windowsVirtualKeyCode: 27 },
  Tab: { key: "Tab", code: "Tab", windowsVirtualKeyCode: 9 },
  Backspace: { key: "Backspace", code: "Backspace", windowsVirtualKeyCode: 8 },
  ArrowRight: { key: "ArrowRight", code: "ArrowRight", windowsVirtualKeyCode: 39 },
  ArrowLeft: { key: "ArrowLeft", code: "ArrowLeft", windowsVirtualKeyCode: 37 },
  ArrowUp: { key: "ArrowUp", code: "ArrowUp", windowsVirtualKeyCode: 38 },
  ArrowDown: { key: "ArrowDown", code: "ArrowDown", windowsVirtualKeyCode: 40 },
  Space: { key: " ", code: "Space", windowsVirtualKeyCode: 32 },
};

async function pressKey(call, keyName) {
  const def = KEY_DEFS[keyName] || {
    key: keyName,
    code: keyName.length === 1 ? `Key${keyName.toUpperCase()}` : keyName,
    windowsVirtualKeyCode: keyName.length === 1 ? keyName.toUpperCase().charCodeAt(0) : 0,
  };
  await call("Input.dispatchKeyEvent", {
    type: "keyDown",
    key: def.key,
    code: def.code,
    windowsVirtualKeyCode: def.windowsVirtualKeyCode,
    nativeVirtualKeyCode: def.windowsVirtualKeyCode,
  });
  await call("Input.dispatchKeyEvent", {
    type: "keyUp",
    key: def.key,
    code: def.code,
    windowsVirtualKeyCode: def.windowsVirtualKeyCode,
    nativeVirtualKeyCode: def.windowsVirtualKeyCode,
  });
}

function writeArtifactIfRequested(filePath, content) {
  if (!filePath) return;
  fs.mkdirSync(path.dirname(path.resolve(filePath)), { recursive: true });
  fs.writeFileSync(filePath, content);
}

async function main() {
  const [subcmd, ...rest] = process.argv.slice(2);
  const { positional, flags } = parseArgs(rest);

  if (!subcmd || subcmd === "help" || flags.help) {
    console.log(`Usage: browser-cdp.mjs <command> [options]
Commands:
  goto <path-or-url> [--wait-ms 2500]
  snapshot [--path <file.aria.txt>]
  click --role <role> --name <label> [--exact] [--index 0] [--timeout 8000]
  fill --role <role> --name <label> --value <text> [--submit] [--exact]
  press <KeyName>
  screenshot --path <file.png>
  console [--path <file.json>] [--assert-clean]
  iframes [--path <file.json>] [--assert-normalized]
  eval <js-expression>`);
    return;
  }

  const session = await connectPage();
  const { call, close } = session;

  try {
    if (subcmd === "goto") {
      const rawTarget = positional[0] || "/";
      const targetUrl = rawTarget.startsWith("http://") || rawTarget.startsWith("https://")
        ? rawTarget
        : `${WEB_URL}${rawTarget.startsWith("/") ? "" : "/"}${rawTarget}`;
      await call("Page.addScriptToEvaluateOnNewDocument", { source: CONSOLE_HOOK_SCRIPT });
      await call("Page.navigate", { url: targetUrl });
      const waitMs = Number(flags["wait-ms"] || 2500);
      await sleep(waitMs);
      await ensureFlutterSemantics(call, 8000);
      await sleep(500);
      const url = await getCurrentUrl(call);
      console.log(JSON.stringify({ ok: true, action: "goto", requested: targetUrl, url }));
    } else if (subcmd === "snapshot") {
      await ensureFlutterSemantics(call, 8000);
      const url = await getCurrentUrl(call);
      const ax = await call("Accessibility.getFullAXTree");
      const nodes = cleanAXNodes(ax.nodes || []);
      const text = formatAXSnapshot(url, nodes);
      writeArtifactIfRequested(flags.path, text);
      process.stdout.write(text);
    } else if (subcmd === "click") {
      const timeoutMs = Number(flags.timeout || 8000);
      const index = Number(flags.index || 0);
      const node = await findMatchingNode(call, {
        role: flags.role,
        name: flags.name,
        exact: Boolean(flags.exact),
        index,
        timeoutMs,
      });
      await clickNode(call, node);
      await sleep(Number(flags["wait-ms"] || 600));
      await ensureFlutterSemantics(call, 3000);
      const url = await getCurrentUrl(call);
      console.log(JSON.stringify({ ok: true, action: "click", role: node.role, name: node.name, url }));
    } else if (subcmd === "fill") {
      const value = flags.value !== undefined ? String(flags.value) : "";
      const timeoutMs = Number(flags.timeout || 8000);
      const node = await findMatchingNode(call, {
        role: flags.role || "textbox",
        name: flags.name,
        exact: Boolean(flags.exact),
        index: Number(flags.index || 0),
        timeoutMs,
      });
      await clickNode(call, node);
      await sleep(250);
      await call("Runtime.evaluate", {
        expression: `(() => {
          const findActive = (root) => {
            let a = root.activeElement;
            while (a && a.shadowRoot && a.shadowRoot.activeElement) {
              a = a.shadowRoot.activeElement;
            }
            return a;
          };
          const el = findActive(document);
          if (el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA")) {
            el.select?.();
            el.value = "";
            el.dispatchEvent(new Event("input", { bubbles: true }));
            return el.tagName;
          }
          return null;
        })()`,
      });
      if (value.length > 0) {
        await call("Input.insertText", { text: value });
      }
      await sleep(250);
      if (flags.submit) {
        await pressKey(call, "Enter");
        await sleep(500);
      }
      const url = await getCurrentUrl(call);
      console.log(JSON.stringify({ ok: true, action: "fill", role: node.role, name: node.name, length: value.length, submitted: Boolean(flags.submit), url }));
    } else if (subcmd === "press") {
      const key = positional[0];
      if (!key) throw new Error("press requires a key name (e.g. Enter, Escape, ArrowRight)");
      await pressKey(call, key);
      await sleep(Number(flags["wait-ms"] || 400));
      const url = await getCurrentUrl(call);
      console.log(JSON.stringify({ ok: true, action: "press", key, url }));
    } else if (subcmd === "screenshot") {
      if (!flags.path) throw new Error("screenshot requires --path <file.png>");
      const shot = await call("Page.captureScreenshot", { format: "png" });
      const buf = Buffer.from(shot.data, "base64");
      writeArtifactIfRequested(flags.path, buf);
      const url = await getCurrentUrl(call);
      console.log(JSON.stringify({ ok: true, action: "screenshot", path: path.resolve(flags.path), bytes: buf.length, url }));
    } else if (subcmd === "console") {
      const res = await call("Runtime.evaluate", {
        expression: `(() => {
          const store = window.__shelfdConsole || { errors: [], warnings: [], logs: [] };
          return {
            url: window.location.href,
            errorCount: store.errors.length,
            warningCount: store.warnings.length,
            errors: store.errors,
            warnings: store.warnings,
            recentLogs: store.logs.slice(-30),
          };
        })()`,
        returnByValue: true,
      });
      const payload = res.result?.value || { errorCount: 0, errors: [] };
      const formatted = JSON.stringify(payload, null, 2) + "\n";
      writeArtifactIfRequested(flags.path, formatted);
      process.stdout.write(formatted);
      if (flags["assert-clean"] && payload.errorCount > 0) {
        process.exitCode = 1;
      }
    } else if (subcmd === "iframes") {
      const frameDocProbe = `(() => {
        if (!document || !document.body) {
          return { bodyChildren: -1, nestedAbsoluteSpans: -1, activeHighlights: [], textPreview: "" };
        }
        return {
          bodyChildren: document.body.children.length,
          nestedAbsoluteSpans: document.querySelectorAll('span[style*="position:absolute"] span[style*="position:absolute"]').length,
          activeHighlights: Array.from(document.querySelectorAll(".-epub-media-overlay-active, [class*='media-overlay-active']")).map((el) => ({
            id: el.id,
            text: (el.textContent || "").trim(),
          })),
          textPreview: (document.body.innerText || document.body.textContent || "").trim().slice(0, 160),
        };
      })()`;

      const collectFramesOnce = async () => {
        const res = await call("Runtime.evaluate", {
          expression: `(() => {
            const frames = [];
            const visit = (root) => {
              if (!root || !root.querySelectorAll) return;
              for (const f of root.querySelectorAll("iframe")) {
                let bodyChildren = -1;
                let nestedAbsoluteSpans = -1;
                let activeHighlights = [];
                let textPreview = "";
                try {
                  const doc = f.contentDocument;
                  if (doc && doc.body) {
                    bodyChildren = doc.body.children.length;
                    nestedAbsoluteSpans = doc.querySelectorAll('span[style*="position:absolute"] span[style*="position:absolute"]').length;
                    activeHighlights = Array.from(doc.querySelectorAll(".-epub-media-overlay-active, [class*='media-overlay-active']")).map((el) => ({
                      id: el.id,
                      text: (el.textContent || "").trim(),
                    }));
                    textPreview = (doc.body.innerText || doc.body.textContent || "").trim().slice(0, 160);
                  }
                } catch {}
                frames.push({ src: f.src, bodyChildren, nestedAbsoluteSpans, activeHighlights, textPreview });
              }
              for (const el of root.querySelectorAll("*")) {
                if (el.shadowRoot) visit(el.shadowRoot);
              }
            };
            visit(document);
            return { url: window.location.href, frameCount: frames.length, frames };
          })()`,
          returnByValue: true,
        });
        const payload = res.result?.value || { url: "", frameCount: 0, frames: [] };
        if (payload.frames.some((f) => f.bodyChildren < 0)) {
          // 1. Inspect out-of-process cross-origin iframe targets via /json/list
          const targets = await listCdpTargets();
          for (const f of payload.frames) {
            if (f.bodyChildren >= 0) continue;
            const oopif = targets.find((t) => t.type === "iframe" && t.url === f.src && t.webSocketDebuggerUrl);
            if (oopif) {
              const sub = await openCdpSocket(oopif.webSocketDebuggerUrl);
              try {
                await sub.call("Runtime.enable");
                const probe = await sub.call("Runtime.evaluate", { expression: frameDocProbe, returnByValue: true });
                if (probe.result?.value) Object.assign(f, probe.result.value);
              } finally {
                sub.close();
              }
            }
          }
          // 2. Fallback: inspect in-process child frames via Page.getFrameTree + Page.createIsolatedWorld
          if (payload.frames.some((f) => f.bodyChildren < 0)) {
            const tree = await call("Page.getFrameTree");
            const childFrames = [];
            const walkTree = (node) => {
              for (const cf of node?.childFrames || []) {
                childFrames.push(cf.frame);
                walkTree(cf);
              }
            };
            walkTree(tree.frameTree);
            for (const f of payload.frames) {
              if (f.bodyChildren >= 0) continue;
              const match = childFrames.find((cf) => cf.url === f.src);
              if (match) {
                try {
                  const world = await call("Page.createIsolatedWorld", { frameId: match.id, worldName: "shelfd-verify" });
                  const probe = await call("Runtime.evaluate", {
                    contextId: world.executionContextId,
                    expression: frameDocProbe,
                    returnByValue: true,
                  });
                  if (probe.result?.value) Object.assign(f, probe.result.value);
                } catch {}
              }
            }
          }
        }
        return payload;
      };

      const deadline = Date.now() + Number(flags["timeout-ms"] || 5000);
      let payload = await collectFramesOnce();
      while (
        flags["assert-normalized"] &&
        Date.now() < deadline &&
        (payload.frameCount === 0 || payload.frames.some((f) => f.bodyChildren <= 0))
      ) {
        await sleep(250);
        payload = await collectFramesOnce();
      }

      const formatted = JSON.stringify(payload, null, 2) + "\n";
      writeArtifactIfRequested(flags.path, formatted);
      process.stdout.write(formatted);
      if (flags["assert-normalized"]) {
        if (payload.frameCount === 0) {
          throw new Error("Expected at least 1 fixed-layout <iframe>, found 0");
        }
        for (const f of payload.frames) {
          if (f.bodyChildren <= 0 || f.nestedAbsoluteSpans !== 0 || /^\s*\{\s*"error"\s*:/.test(f.textPreview || "")) {
            throw new Error(`Iframe normalization check failed for ${f.src}: bodyChildren=${f.bodyChildren}, nestedAbsoluteSpans=${f.nestedAbsoluteSpans}, textPreview=${JSON.stringify(f.textPreview)}`);
          }
        }
      }
    } else if (subcmd === "eval") {
      const expr = positional.join(" ");
      if (!expr) throw new Error("eval requires a JavaScript expression");
      const res = await call("Runtime.evaluate", {
        expression: expr,
        awaitPromise: true,
        returnByValue: true,
      });
      if (res.exceptionDetails) {
        throw new Error(res.exceptionDetails.text || "JS evaluation failed");
      }
      console.log(JSON.stringify(res.result?.value ?? null, null, 2));
    } else {
      throw new Error(`Unknown browser subcommand: ${subcmd}`);
    }
  } finally {
    close();
  }
}

main().catch((err) => {
  console.error(`ERROR: ${err.message}`);
  process.exit(1);
});
