import {
  definePlugin,
  truncateForMCP,
  type ActionResult,
  type PluginContext,
} from "@thebenforce/shelfd-plugin-sdk";

export const DEFAULT_NOTEBOOKLM_URL = "https://notebooklm.google.com/";

export interface NotebookLMConfig extends Record<string, unknown> {
  notebook_url?: string;
  webhook_url?: string;
  endpoint?: string;
  project_id?: string;
  location?: string;
  api_key?: string;
  allow_local_endpoints?: boolean;
}

function sanitizeDownloadFilename(name: string): string {
  const trimmed = name.trim();
  const cleaned = trimmed
    .replace(/[\\/:*?"<>|\x00-\x1f]/g, "_")
    .trim()
    .replace(/^[._]+|[._]+$/g, "");
  return cleaned;
}

export function buildMarkdownFilename(bookTitle: string, bookId: string): string {
  let clean = sanitizeDownloadFilename(bookTitle);
  if (!clean) {
    clean = sanitizeDownloadFilename(bookId);
  }
  if (!clean) {
    clean = "book";
  }
  return `${clean}.md`;
}

function parseIPv4Octets(host: string): [number, number, number, number] | null {
  const m = host.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
  if (!m) return null;
  const octets = [Number(m[1]), Number(m[2]), Number(m[3]), Number(m[4])] as [number, number, number, number];
  if (octets.some((o) => Number.isNaN(o) || o < 0 || o > 255)) {
    return null;
  }
  return octets;
}

export function validateSSRF(rawUrl: string, allowLocalEndpoints = false): void {
  const trimmed = rawUrl.trim();
  if (!trimmed) {
    throw new Error("Endpoint URL cannot be empty");
  }

  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    throw new Error(`Invalid URL: ${trimmed}`);
  }

  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error(`Unsupported URL scheme ${parsed.protocol} (must be http: or https:)`);
  }

  const rawHost = parsed.hostname.replace(/^\[|\]$/g, "").toLowerCase().trim();
  if (!rawHost) {
    throw new Error(`Invalid URL hostname in ${trimmed}`);
  }

  if (
    rawHost === "metadata.google.internal" ||
    rawHost === "metadata" ||
    rawHost === "169.254.169.254" ||
    rawHost.startsWith("169.254.") ||
    rawHost === "fd00:ec2::254" ||
    rawHost.startsWith("fe80:") ||
    rawHost.startsWith("::ffff:169.254.") ||
    rawHost.startsWith("::ffff:a9fe:")
  ) {
    throw new Error(`SSRF blocked: prohibited cloud metadata or link-local address (${parsed.hostname})`);
  }

  const ipv4 = parseIPv4Octets(rawHost);
  if (ipv4) {
    const [a, b] = ipv4;
    if (a === 169 && b === 254) {
      throw new Error(`SSRF blocked: prohibited IPv4 link-local / metadata address (${parsed.hostname})`);
    }
    if (a === 0) {
      throw new Error(`SSRF blocked: prohibited unspecified IPv4 address (${parsed.hostname})`);
    }
    if (a === 127) {
      if (!allowLocalEndpoints) {
        throw new Error(`SSRF blocked: loopback address (${parsed.hostname}) is not allowed`);
      }
      return;
    }
    if (a === 10 || (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168)) {
      throw new Error(`SSRF blocked: private RFC1918 network address (${parsed.hostname}) is not allowed`);
    }
    return;
  }

  if (rawHost === "localhost" || rawHost.endsWith(".localhost") || rawHost === "::1" || rawHost === "0:0:0:0:0:0:0:1") {
    if (!allowLocalEndpoints) {
      throw new Error(`SSRF blocked: loopback host (${parsed.hostname}) is not allowed`);
    }
    return;
  }

  if (rawHost === "::" || rawHost.startsWith("fc") || rawHost.startsWith("fd")) {
    throw new Error(`SSRF blocked: private or unique-local IPv6 address (${parsed.hostname}) is not allowed`);
  }
}

function resolveNotebookUrl(cfg: NotebookLMConfig): string {
  const raw = typeof cfg.notebook_url === "string" ? cfg.notebook_url.trim() : "";
  return raw || DEFAULT_NOTEBOOKLM_URL;
}

function isConfiguredForPush(cfg: NotebookLMConfig): boolean {
  const webhookUrl = typeof cfg.webhook_url === "string" ? cfg.webhook_url.trim() : "";
  if (webhookUrl) return true;
  const projectId = typeof cfg.project_id === "string" ? cfg.project_id.trim() : "";
  const apiKey = typeof cfg.api_key === "string" ? cfg.api_key.trim() : "";
  return Boolean(projectId && apiKey);
}

async function resolveBookTitleAndFileName(
  ctx: PluginContext<NotebookLMConfig>,
  bookId: string,
  targetTitle?: string
): Promise<{ title: string; fileName: string }> {
  let title = (targetTitle ?? "").trim();
  try {
    const res = await ctx.fetchLoopback(`/api/v1/books/${encodeURIComponent(bookId)}`, {
      headers: { Accept: "application/json" },
    });
    if (res.ok) {
      const data = (await res.json()) as Record<string, unknown>;
      const bookObj = (data.book && typeof data.book === "object" ? data.book : data) as Record<string, unknown>;
      if (typeof bookObj.title === "string" && bookObj.title.trim()) {
        title = bookObj.title.trim();
      }
    }
  } catch {
    void 0;
  }
  if (!title) {
    title = bookId;
  }
  return {
    title,
    fileName: buildMarkdownFilename(title, bookId),
  };
}

export async function executeSendToNotebookLM(
  bookId: string,
  targetTitle: string | undefined,
  ctx: PluginContext<NotebookLMConfig>
): Promise<ActionResult> {
  const cleanBookId = bookId.trim();
  if (!cleanBookId) {
    throw new Error("Book ID is required");
  }

  const defaultNotebookUrl = resolveNotebookUrl(ctx.config);
  const downloadApiPath = `/api/v1/books/${encodeURIComponent(cleanBookId)}/markdown`;
  const allowLocal = Boolean(ctx.config.allow_local_endpoints);

  if (!isConfiguredForPush(ctx.config)) {
    const { fileName } = await resolveBookTitleAndFileName(ctx, cleanBookId, targetTitle);
    return {
      status: "client_handoff",
      open_url: defaultNotebookUrl,
      download_api_path: downloadApiPath,
      file_name: fileName,
      mime_type: "text/markdown; charset=utf-8",
      message: `Prepared "${fileName}" for NotebookLM. Upload the downloaded Markdown file as a source.`,
    };
  }

  const webhookUrl = typeof ctx.config.webhook_url === "string" ? ctx.config.webhook_url.trim() : "";
  const apiKey = typeof ctx.config.api_key === "string" ? ctx.config.api_key.trim() : "";

  if (webhookUrl) {
    validateSSRF(webhookUrl, allowLocal);
  } else {
    const location =
      typeof ctx.config.location === "string" && ctx.config.location.trim() ? ctx.config.location.trim() : "global";
    const rawEndpoint = typeof ctx.config.endpoint === "string" ? ctx.config.endpoint.trim().replace(/\/+$/, "") : "";
    const baseEndpoint = rawEndpoint || `https://${location}-discoveryengine.googleapis.com`;
    validateSSRF(baseEndpoint, allowLocal);
  }

  const { title } = await resolveBookTitleAndFileName(ctx, cleanBookId, targetTitle);
  const { markdown, fileName: mdHeaderFileName } = await ctx.getBookMarkdown(cleanBookId);
  const fileName = mdHeaderFileName || buildMarkdownFilename(title, cleanBookId);

  if (webhookUrl) {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (apiKey) {
      headers.Authorization = `Bearer ${apiKey}`;
    }

    const resp = await fetch(webhookUrl, {
      method: "POST",
      headers,
      body: JSON.stringify({
        book_id: cleanBookId,
        title,
        file_name: fileName,
        content_type: "text/markdown",
        content: markdown,
      }),
    });

    const respBody = await resp.text();
    if (!resp.ok) {
      throw new Error(`NotebookLM upstream returned HTTP ${resp.status}: ${respBody.slice(0, 400).trim()}`);
    }

    let resolvedNotebookUrl = defaultNotebookUrl;
    let message = `Sent "${title}" to NotebookLM.`;
    if (respBody.trim()) {
      try {
        const parsed = JSON.parse(respBody) as { notebook_url?: string; url?: string; message?: string };
        if (typeof parsed.notebook_url === "string" && parsed.notebook_url.trim()) {
          resolvedNotebookUrl = parsed.notebook_url.trim();
        } else if (typeof parsed.url === "string" && parsed.url.trim()) {
          resolvedNotebookUrl = parsed.url.trim();
        }
        if (typeof parsed.message === "string" && parsed.message.trim()) {
          message = parsed.message.trim();
        }
      } catch {
        void 0;
      }
    }

    return {
      status: "completed",
      open_url: resolvedNotebookUrl,
      external_url: resolvedNotebookUrl,
      download_api_path: downloadApiPath,
      file_name: fileName,
      message,
    };
  }

  const location =
    typeof ctx.config.location === "string" && ctx.config.location.trim() ? ctx.config.location.trim() : "global";
  const rawEndpoint = typeof ctx.config.endpoint === "string" ? ctx.config.endpoint.trim().replace(/\/+$/, "") : "";
  const baseEndpoint = rawEndpoint || `https://${location}-discoveryengine.googleapis.com`;
  const projectId = typeof ctx.config.project_id === "string" ? ctx.config.project_id.trim() : "";

  const createUrl = `${baseEndpoint}/v1alpha/projects/${encodeURIComponent(projectId)}/locations/${encodeURIComponent(location)}/notebooks`;
  const createResp = await fetch(createUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${apiKey}`,
    },
    body: JSON.stringify({ title }),
  });

  const createBody = await createResp.text();
  if (!createResp.ok) {
    throw new Error(`NotebookLM create notebook returned HTTP ${createResp.status}: ${createBody.slice(0, 400).trim()}`);
  }

  let notebookId = "";
  try {
    const parsedCreate = JSON.parse(createBody) as { name?: string; notebookId?: string };
    if (typeof parsedCreate.notebookId === "string" && parsedCreate.notebookId.trim()) {
      notebookId = parsedCreate.notebookId.trim();
    } else if (typeof parsedCreate.name === "string" && parsedCreate.name.trim()) {
      const parts = parsedCreate.name.trim().replace(/\/+$/, "").split("/");
      notebookId = parts[parts.length - 1] ?? "";
    }
  } catch {
    void 0;
  }

  if (notebookId) {
    const uploadUrl = `${baseEndpoint}/upload/v1alpha/projects/${encodeURIComponent(projectId)}/locations/${encodeURIComponent(location)}/notebooks/${encodeURIComponent(notebookId)}/sources:uploadFile`;
    const uploadResp = await fetch(uploadUrl, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${apiKey}`,
        "Content-Type": "text/markdown",
        "X-Goog-Upload-File-Name": fileName,
        "X-Goog-Upload-Protocol": "raw",
      },
      body: markdown,
    });
    const uploadBody = await uploadResp.text();
    if (!uploadResp.ok) {
      throw new Error(`NotebookLM source upload returned HTTP ${uploadResp.status}: ${uploadBody.slice(0, 400).trim()}`);
    }
  }

  let resolvedNotebookUrl = defaultNotebookUrl;
  if (notebookId && resolvedNotebookUrl === DEFAULT_NOTEBOOKLM_URL) {
    resolvedNotebookUrl = `https://notebooklm.google.com/notebook/${notebookId}`;
  }

  return {
    status: "completed",
    open_url: resolvedNotebookUrl,
    external_url: resolvedNotebookUrl,
    download_api_path: downloadApiPath,
    file_name: fileName,
    message: `Sent "${title}" to NotebookLM.`,
  };
}

definePlugin<NotebookLMConfig>({
  name: "notebooklm",
  version: "0.1.0",

  actions: {
    send_to_notebooklm: {
      title: "Send to NotebookLM",
      description: "Push compiled book Markdown with highlights and notes to NotebookLM or download for browser handoff.",
      icon: "auto_awesome",
      targets: ["book"],
      mediaTypes: ["ebook"],
      mode: "immediate",
      execution: "sync",

      async invoke(req, ctx) {
        return await executeSendToNotebookLM(req.target.id, req.target.title, ctx);
      },
    },
  },

  tools: {
    send_book_to_notebooklm: {
      description:
        "Send a Shelfd book's compiled Markdown (metadata, synopsis, highlights, notes, and chapters) to Google NotebookLM or prepare a browser handoff.",
      inputSchema: {
        type: "object",
        required: ["book_id"],
        properties: {
          book_id: {
            type: "string",
            description: "The Shelfd book ID to send to NotebookLM.",
          },
        },
      },
      async execute(args, ctx) {
        const bookId = String(args.book_id ?? "").trim();
        if (!bookId) {
          throw new Error("book_id is required");
        }
        const res = await executeSendToNotebookLM(bookId, undefined, ctx);
        const detail =
          res.status === "client_handoff"
            ? `${res.message} (Download: ${res.download_api_path}, Open: ${res.open_url})`
            : `${res.message} (URL: ${res.open_url})`;
        return truncateForMCP(detail, 3800);
      },
    },
  },
});
