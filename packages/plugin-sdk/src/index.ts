import readline from "node:readline";
import util from "node:util";
import type {
  CatalogBrowseRequest,
  CatalogDescriptor,
  CatalogSearchRequest,
  CatalogStageRequest,
  CommandDef,
  PluginContext,
  PluginDefinition,
  PoliteFetcherOptions,
  ProgressNotification,
  StageBookInput,
  StageBookResponse,
  ToolDef,
} from "./types.ts";

export * from "./types.ts";

const MAX_NOTIFICATION_BYTES = 62 * 1024;

/**
 * Thrown by plugin handlers when storefront credentials are missing, expired, or rejected (HTTP 401/403).
 * Maps to JSON-RPC error code -32001 so Shelfd surfaces an actionable re-authentication prompt.
 */
export class AuthExpiredError extends Error {
  readonly code = -32001;

  constructor(message = "Storefront authentication expired or missing. Please update plugin credentials in Settings -> Plugins.") {
    super(message);
    this.name = "AuthExpiredError";
  }
}

/**
 * Thrown by plugin handlers to return a structured JSON-RPC error code and message.
 */
export class PluginRPCError extends Error {
  readonly code: number;
  readonly data?: unknown;

  constructor(code: number, message: string, data?: unknown) {
    super(message);
    this.name = "PluginRPCError";
    this.code = code;
    this.data = data;
  }
}

/**
 * Truncates text cleanly so MCP tool outputs stay within Shelfd's Level-of-Detail (LOD) token budget.
 */
export function truncateForMCP(text: string, maxChars = 4000): string {
  if (text.length <= maxChars) {
    return text;
  }
  return `${text.slice(0, maxChars)}\n...[truncated]`;
}

/**
 * Wraps fetch() with a polite minimum interval between requests and automatic 429 Retry-After backoff.
 */
export function createPoliteFetcher(options: PoliteFetcherOptions = {}): typeof fetch {
  const minIntervalMs = options.minIntervalMs ?? 350;
  const maxRetries = options.maxRetries ?? 3;
  const userAgent = options.userAgent ?? "Shelfd-Plugin/0.1 (+https://github.com/theBenForce/shelfd)";
  const baseFetch = options.fetchImpl ?? globalThis.fetch;

  let lastRequestAt = 0;
  let queue: Promise<void> = Promise.resolve();

  const sleep = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));

  return async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    let releaseGate!: () => void;
    const prev = queue;
    queue = new Promise<void>((resolve) => {
      releaseGate = resolve;
    });

    await prev;
    try {
      const elapsed = Date.now() - lastRequestAt;
      if (elapsed < minIntervalMs) {
        await sleep(minIntervalMs - elapsed);
      }
      lastRequestAt = Date.now();
    } finally {
      releaseGate();
    }

    const headers = new Headers(init?.headers);
    if (!headers.has("user-agent")) {
      headers.set("user-agent", userAgent);
    }

    let attempt = 0;
    while (true) {
      const response = await baseFetch(input, { ...init, headers });
      if (response.status !== 429 || attempt >= maxRetries) {
        return response;
      }
      attempt += 1;
      const retryHeader = response.headers.get("retry-after");
      let delayMs = 1000 * Math.pow(2, attempt - 1);
      if (retryHeader) {
        const parsedSeconds = Number(retryHeader);
        if (!Number.isNaN(parsedSeconds) && parsedSeconds > 0) {
          delayMs = Math.min(parsedSeconds * 1000, 15_000);
        }
      }
      await sleep(delayMs);
    }
  };
}

interface WireMessage {
  jsonrpc: "2.0";
  id?: string | number;
  method?: string;
  params?: Record<string, unknown>;
  result?: unknown;
  error?: { code: number; message: string; data?: unknown };
}

interface InvocationAuthWire {
  user_id?: string;
  invocation_token?: string;
  api_base?: string;
  config?: Record<string, unknown>;
}

function toBlob(data: Uint8Array | ArrayBuffer | Blob, mimeType: string): Blob {
  if (data instanceof Blob) {
    return data;
  }
  if (data instanceof ArrayBuffer) {
    return new Blob([data], { type: mimeType });
  }
  return new Blob([new Uint8Array(data)], { type: mimeType });
}

/**
 * Starts the Shelfd NDJSON JSON-RPC 2.0 plugin runtime on process.stdin / process.stdout.
 * Automatically redirects console.log/info/warn to stderr so stdout remains strictly valid NDJSON.
 */
export function definePlugin<TConfig extends Record<string, unknown> = Record<string, unknown>>(
  definition: PluginDefinition<TConfig>
): PluginDefinition<TConfig> {
  const writeStdoutRaw = process.stdout.write.bind(process.stdout);

  // Redirect console stdout writers to stderr to prevent accidental NDJSON frame corruption
  const writeStderrLine = (...args: unknown[]): void => {
    process.stderr.write(`${util.format(...args)}\n`);
  };
  console.log = writeStderrLine;
  console.info = writeStderrLine;
  console.warn = writeStderrLine;
  console.debug = writeStderrLine;

  let hostApiBase = "";
  let hostDataDir = process.env.SHELFD_DATA_DIR ?? "";
  let outboundSeq = 0;
  const pendingHostCalls = new Map<string, { resolve: (val: unknown) => void; reject: (err: Error) => void }>();

  const sendFrame = (frame: WireMessage): void => {
    writeStdoutRaw(`${JSON.stringify(frame)}\n`);
  };

  const sendNotification = (method: string, params: Record<string, unknown>): void => {
    const encoded = JSON.stringify({ jsonrpc: "2.0", method, params });
    if (Buffer.byteLength(encoded, "utf8") > MAX_NOTIFICATION_BYTES) {
      process.stderr.write(`[plugin-sdk] dropping oversized ${method} notification (>64KB)\n`);
      return;
    }
    writeStdoutRaw(`${encoded}\n`);
  };

  const callHost = (method: string, params: Record<string, unknown>): Promise<unknown> => {
    outboundSeq += 1;
    const id = `plugin-${outboundSeq}`;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        pendingHostCalls.delete(id);
        reject(new Error(`Timed out waiting for host response to ${method}`));
      }, 15_000);

      pendingHostCalls.set(id, {
        resolve: (val) => {
          clearTimeout(timer);
          resolve(val);
        },
        reject: (err) => {
          clearTimeout(timer);
          reject(err);
        },
      });

      sendFrame({ jsonrpc: "2.0", id, method, params });
    });
  };

  const buildContext = (rawAuth?: InvocationAuthWire): PluginContext<TConfig> => {
    const userId = typeof rawAuth?.user_id === "string" ? rawAuth.user_id : "";
    const apiBase = (typeof rawAuth?.api_base === "string" && rawAuth.api_base ? rawAuth.api_base : hostApiBase).replace(/\/+$/, "");
    const invocationToken = typeof rawAuth?.invocation_token === "string" ? rawAuth.invocation_token : "";
    const config = ((rawAuth?.config && typeof rawAuth.config === "object") ? rawAuth.config : {}) as TConfig;

    return {
      userId,
      apiBase,
      invocationToken,
      dataDir: hostDataDir,
      config,

      async stageBook(input: StageBookInput): Promise<StageBookResponse> {
        if (!apiBase) {
          throw new Error("Missing Shelfd api_base in invocation context");
        }
        if (!invocationToken) {
          throw new Error("Missing scoped invocation_token; ensure \"uploads:stage\" is listed in plugin.json api_scopes");
        }

        const form = new FormData();
        const fileMime = input.mimeType ?? "application/epub+zip";
        form.append("file", toBlob(input.data, fileMime), input.filename);

        if (input.cover) {
          const coverMime = input.cover.mimeType ?? "image/jpeg";
          const coverName = input.cover.filename ?? "cover.jpg";
          form.append("cover", toBlob(input.cover.data, coverMime), coverName);
        }

        form.append("source_plugin", definition.name);
        if (input.externalId) {
          form.append("source_external_id", input.externalId);
        }
        if (input.metadataDefaults) {
          form.append("metadata_defaults", JSON.stringify(input.metadataDefaults));
        }

        const res = await fetch(`${apiBase}/api/v1/books/upload/stage`, {
          method: "POST",
          headers: {
            Authorization: `Bearer ${invocationToken}`,
          },
          body: form,
        });

        const bodyText = await res.text();
        let parsed: Record<string, unknown> = {};
        try {
          parsed = JSON.parse(bodyText) as Record<string, unknown>;
        } catch {
          // handled below
        }

        if (!res.ok) {
          const errMsg = typeof parsed.error === "string" ? parsed.error : `HTTP ${res.status}: ${bodyText.slice(0, 200)}`;
          throw new Error(`StageUploadBook failed: ${errMsg}`);
        }

        return parsed as unknown as StageBookResponse;
      },

      async patchConfig(values: Partial<TConfig>): Promise<void> {
        if (!userId) {
          throw new Error("Cannot patch config without an active user_id in invocation context");
        }
        await callHost("config/patch", {
          user_id: userId,
          values,
        });
      },

      emitEvent(event: string, data: Record<string, unknown> = {}): void {
        sendNotification("event", { event, ...data });
      },

      reportProgress(progress: ProgressNotification): void {
        sendNotification("progress", {
          batch_id: progress.batchId,
          current: progress.current,
          total: progress.total,
          external_id: progress.externalId,
          title: progress.title,
          status: progress.status ?? "staging",
          message: progress.message,
        });
      },

      log(level: "debug" | "info" | "warn" | "error", message: string): void {
        sendNotification("log", { level, message });
      },
    };
  };

  const handleRequest = async (msg: WireMessage): Promise<unknown> => {
    const method = msg.method ?? "";
    const params = (msg.params ?? {}) as Record<string, unknown>;

    switch (method) {
      case "initialize": {
        hostApiBase = typeof params.api_base === "string" ? params.api_base : "";
        if (typeof params.data_dir === "string" && params.data_dir) {
          hostDataDir = params.data_dir;
        }
        if (definition.onInitialize) {
          await definition.onInitialize({
            protocolVersion: String(params.protocol_version ?? ""),
            shelfdVersion: String(params.shelfd_version ?? ""),
            apiBase: hostApiBase,
            dataDir: hostDataDir,
          });
        }

        const tools: ToolDef[] = Object.entries(definition.tools ?? {}).map(([name, spec]) => ({
          name,
          description: spec.description,
          input_schema: spec.inputSchema,
        }));

        const commands: CommandDef[] = Object.entries(definition.commands ?? {}).map(([name, spec]) => ({
          name,
          title: spec.title,
          description: spec.description,
          input_schema: spec.inputSchema,
        }));

        let catalog: CatalogDescriptor | undefined = definition.catalog?.descriptor;
        if (definition.catalog && !catalog) {
          const modes: ("browse" | "search")[] = [];
          if (definition.catalog.browse) modes.push("browse");
          if (definition.catalog.search) modes.push("search");
          catalog = { modes };
        }

        return {
          name: definition.name,
          version: definition.version,
          tools: tools.length > 0 ? tools : undefined,
          commands: commands.length > 0 ? commands : undefined,
          catalog,
        };
      }

      case "tools/call": {
        const toolName = String(params.name ?? "");
        const spec = definition.tools?.[toolName];
        if (!spec) {
          throw new PluginRPCError(-32601, `Unknown tool: ${toolName}`);
        }
        const args = (params.arguments && typeof params.arguments === "object" ? params.arguments : {}) as Record<string, unknown>;
        const ctx = buildContext(params.auth as InvocationAuthWire | undefined);
        const out = await spec.execute(args, ctx);
        if (typeof out === "string") {
          return { content: out };
        }
        return { content: out.content, is_error: out.isError ?? false };
      }

      case "commands/invoke": {
        const cmdName = String(params.command ?? "");
        const spec = definition.commands?.[cmdName];
        if (!spec) {
          throw new PluginRPCError(-32601, `Unknown command: ${cmdName}`);
        }
        const args = (params.args && typeof params.args === "object" ? params.args : {}) as Record<string, unknown>;
        const ctx = buildContext(params.auth as InvocationAuthWire | undefined);
        return (await spec.execute(args, ctx)) ?? { ok: true };
      }

      case "catalog/browse": {
        if (!definition.catalog?.browse) {
          throw new PluginRPCError(-32601, `Plugin ${definition.name} does not implement catalog/browse`);
        }
        const ctx = buildContext(params.auth as InvocationAuthWire | undefined);
        const req: CatalogBrowseRequest = {
          group: typeof params.group === "string" ? params.group : undefined,
          cursor: typeof params.cursor === "string" ? params.cursor : undefined,
          filter: typeof params.filter === "string" ? params.filter : undefined,
        };
        return await definition.catalog.browse(req, ctx);
      }

      case "catalog/search": {
        if (!definition.catalog?.search) {
          throw new PluginRPCError(-32601, `Plugin ${definition.name} does not implement catalog/search`);
        }
        const ctx = buildContext(params.auth as InvocationAuthWire | undefined);
        const req: CatalogSearchRequest = {
          query: String(params.query ?? ""),
          cursor: typeof params.cursor === "string" ? params.cursor : undefined,
          filters: (params.filters && typeof params.filters === "object" ? params.filters : undefined) as Record<string, unknown> | undefined,
        };
        return await definition.catalog.search(req, ctx);
      }

      case "catalog/stage": {
        if (!definition.catalog?.stage) {
          throw new PluginRPCError(-32601, `Plugin ${definition.name} does not implement catalog/stage`);
        }
        const ctx = buildContext(params.auth as InvocationAuthWire | undefined);
        const items = Array.isArray(params.items) ? params.items : [];
        const req: CatalogStageRequest = {
          items: items.map((item: Record<string, unknown>) => ({
            external_id: String(item.external_id ?? ""),
            preferred_format: typeof item.preferred_format === "string" ? item.preferred_format : undefined,
          })),
        };
        return await definition.catalog.stage(req, ctx);
      }

      case "config/update": {
        if (definition.onConfigUpdate) {
          const userId = String(params.user_id ?? "");
          const cfg = ((params.config && typeof params.config === "object") ? params.config : {}) as TConfig;
          await definition.onConfigUpdate(userId, cfg);
        }
        return { ok: true };
      }

      case "shutdown": {
        if (definition.onShutdown) {
          await definition.onShutdown();
        }
        setImmediate(() => process.exit(0));
        return { ok: true };
      }

      default:
        throw new PluginRPCError(-32601, `Method not found: ${method}`);
    }
  };

  const rl = readline.createInterface({
    input: process.stdin,
    crlfDelay: Infinity,
  });

  rl.on("line", (line: string) => {
    const trimmed = line.trim();
    if (!trimmed) return;

    let msg: WireMessage;
    try {
      msg = JSON.parse(trimmed) as WireMessage;
    } catch (err) {
      process.stderr.write(`[plugin-sdk] invalid JSON frame on stdin: ${String(err)}\n`);
      return;
    }

    // Response to an outbound plugin -> host RPC (e.g. config/patch)
    if (msg.id !== undefined && !msg.method) {
      const key = String(msg.id);
      const pending = pendingHostCalls.get(key);
      if (pending) {
        pendingHostCalls.delete(key);
        if (msg.error) {
          pending.reject(new Error(msg.error.message));
        } else {
          pending.resolve(msg.result);
        }
      }
      return;
    }

    // Inbound request or notification from host
    void (async () => {
      try {
        const result = await handleRequest(msg);
        if (msg.id !== undefined) {
          sendFrame({ jsonrpc: "2.0", id: msg.id, result });
        }
      } catch (err) {
        if (msg.id === undefined) {
          process.stderr.write(`[plugin-sdk] unhandled notification error: ${String(err)}\n`);
          return;
        }
        if (err instanceof AuthExpiredError) {
          sendFrame({
            jsonrpc: "2.0",
            id: msg.id,
            error: { code: err.code, message: err.message },
          });
          return;
        }
        if (err instanceof PluginRPCError) {
          sendFrame({
            jsonrpc: "2.0",
            id: msg.id,
            error: { code: err.code, message: err.message, data: err.data },
          });
          return;
        }
        const message = err instanceof Error ? err.message : String(err);
        sendFrame({
          jsonrpc: "2.0",
          id: msg.id,
          error: { code: -32603, message },
        });
      }
    })();
  });

  rl.on("close", () => {
    process.exit(0);
  });

  return definition;
}
