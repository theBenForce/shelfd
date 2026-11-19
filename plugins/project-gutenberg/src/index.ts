import {
  createPoliteFetcher,
  definePlugin,
  truncateForMCP,
  type CatalogGroup,
  type CatalogItem,
  type CatalogPage,
  type CatalogStagedEntry,
} from "@shelfd/plugin-sdk";

export interface GutenbergConfig extends Record<string, unknown> {
  default_language?: string;
  prefer_images?: boolean;
  base_url?: string;
}

interface GutendexPerson {
  name: string;
  birth_year?: number | null;
  death_year?: number | null;
}

interface GutendexBook {
  id: number;
  title: string;
  authors?: GutendexPerson[];
  summaries?: string[];
  subjects?: string[];
  bookshelves?: string[];
  languages?: string[];
  download_count?: number;
  formats?: Record<string, string>;
}

interface GutendexListResponse {
  count?: number;
  next?: string | null;
  results?: GutendexBook[];
}

const BROWSE_GROUPS: CatalogGroup[] = [
  { id: "popular", label: "Most Popular" },
  { id: "science fiction", label: "Science Fiction" },
  { id: "mystery", label: "Mystery & Detective" },
  { id: "fantasy", label: "Fantasy" },
  { id: "adventure", label: "Adventure" },
  { id: "historical fiction", label: "Historical Fiction" },
  { id: "philosophy", label: "Philosophy" },
  { id: "children", label: "Children's Classics" },
];

const politeFetch = createPoliteFetcher({
  minIntervalMs: 200,
  maxRetries: 2,
  userAgent: "Shelfd-ProjectGutenberg-Plugin/0.1 (+https://github.com/theBenForce/shelfd)",
});

/**
 * Converts Gutendex "Last, First" author strings into natural "First Last" order
 * so Audiobookshelf /library/<Author>/<Title> paths stay consistent.
 */
export function normalizeGutenbergAuthor(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed.includes(",")) {
    return trimmed;
  }
  const parts = trimmed.split(",").map((s) => s.trim()).filter(Boolean);
  if (parts.length === 2) {
    return `${parts[1]} ${parts[0]}`;
  }
  return trimmed;
}

export function extractCleanGenres(book: GutendexBook): string[] {
  const seen = new Set<string>();
  const out: string[] = [];

  for (const shelf of book.bookshelves ?? []) {
    const cleaned = shelf.replace(/^Browsing:\s*/i, "").trim();
    if (cleaned && !seen.has(cleaned.toLowerCase())) {
      seen.add(cleaned.toLowerCase());
      out.push(cleaned);
    }
    if (out.length >= 5) return out;
  }

  for (const subj of book.subjects ?? []) {
    const firstSegment = subj.split("--")[0]?.trim() ?? "";
    if (firstSegment && firstSegment.length <= 40 && !seen.has(firstSegment.toLowerCase())) {
      seen.add(firstSegment.toLowerCase());
      out.push(firstSegment);
    }
    if (out.length >= 5) break;
  }
  return out;
}

export function selectEpubUrl(formats: Record<string, string> | undefined, preferImages: boolean): string | undefined {
  if (!formats) return undefined;
  const entries = Object.entries(formats).filter(
    ([mime, url]) => mime.toLowerCase().startsWith("application/epub") && Boolean(url)
  );
  if (entries.length === 0) return undefined;

  if (preferImages) {
    const withImages = entries.find(([, url]) => url.includes(".images") || !url.includes("noimages"));
    if (withImages) return withImages[1];
  } else {
    const noImages = entries.find(([, url]) => url.includes("noimages"));
    if (noImages) return noImages[1];
  }
  return entries[0][1];
}

function selectCoverUrl(formats: Record<string, string> | undefined): string | undefined {
  if (!formats) return undefined;
  if (formats["image/jpeg"]) return formats["image/jpeg"];
  const imgEntry = Object.entries(formats).find(([mime]) => mime.toLowerCase().startsWith("image/"));
  return imgEntry?.[1];
}

function resolveBaseUrl(cfg: GutenbergConfig): string {
  const raw = typeof cfg.base_url === "string" && cfg.base_url.trim() ? cfg.base_url.trim() : "https://gutendex.com";
  return raw.replace(/\/+$/, "");
}

function resolveLanguage(cfg: GutenbergConfig): string {
  if (typeof cfg.default_language === "string") {
    return cfg.default_language.trim().toLowerCase();
  }
  return "en";
}

function mapBookToCatalogItem(book: GutendexBook, preferImages: boolean): CatalogItem | null {
  const epubUrl = selectEpubUrl(book.formats, preferImages);
  if (!epubUrl) {
    return null;
  }
  const authors = (book.authors ?? []).map((a) => normalizeGutenbergAuthor(a.name)).filter(Boolean);
  const summary = book.summaries?.[0]?.trim();
  const genres = extractCleanGenres(book);

  return {
    external_id: String(book.id),
    title: book.title,
    authors: authors.length > 0 ? authors : ["Unknown"],
    summary,
    cover_url: selectCoverUrl(book.formats),
    publisher: "Project Gutenberg",
    language: book.languages?.[0] ?? "en",
    genres,
    formats: [{ format: "epub", label: "EPUB" }],
  };
}

function encodeNextCursor(nextUrl: string | null | undefined): string | undefined {
  if (!nextUrl) return undefined;
  try {
    const parsed = new URL(nextUrl);
    const page = parsed.searchParams.get("page");
    return page || undefined;
  } catch {
    return undefined;
  }
}

async function queryGutendexBooks(
  cfg: GutenbergConfig,
  params: { search?: string; topic?: string; cursor?: string }
): Promise<CatalogPage> {
  const baseUrl = resolveBaseUrl(cfg);
  const preferImages = cfg.prefer_images !== false;
  const lang = resolveLanguage(cfg);

  const url = new URL(`${baseUrl}/books`);
  url.searchParams.set("mime_type", "application/epub");
  if (lang) {
    url.searchParams.set("languages", lang);
  }
  if (params.search?.trim()) {
    url.searchParams.set("search", params.search.trim());
  }
  if (params.topic && params.topic !== "popular") {
    url.searchParams.set("topic", params.topic);
  }
  if (params.cursor?.trim()) {
    url.searchParams.set("page", params.cursor.trim());
  }

  const res = await politeFetch(url.toString(), {
    headers: { Accept: "application/json" },
  });
  if (!res.ok) {
    throw new Error(`Gutendex returned HTTP ${res.status}`);
  }
  const payload = (await res.json()) as GutendexListResponse;
  const items: CatalogItem[] = [];
  for (const b of payload.results ?? []) {
    const mapped = mapBookToCatalogItem(b, preferImages);
    if (mapped) {
      items.push(mapped);
    }
  }

  return {
    items,
    groups: BROWSE_GROUPS,
    next_cursor: encodeNextCursor(payload.next),
    total_count: payload.count,
  };
}

function sanitizeFilename(title: string, externalId: string): string {
  const slug = title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 60);
  return `${slug || "gutenberg"}-pg${externalId}.epub`;
}

definePlugin<GutenbergConfig>({
  name: "project-gutenberg",
  version: "0.1.0",

  tools: {
    search_books: {
      description:
        "Search free public-domain ebooks on Project Gutenberg by title, author, or subject. Returns concise book IDs, titles, and authors.",
      inputSchema: {
        type: "object",
        required: ["query"],
        properties: {
          query: {
            type: "string",
            description: "Title, author, or topic search terms (e.g. 'Sherlock Holmes' or 'Mary Shelley')",
          },
          topic: {
            type: "string",
            description: "Optional subject/bookshelf filter (e.g. 'science fiction')",
          },
        },
      },
      async execute(args, ctx) {
        const query = String(args.query ?? "").trim();
        const topic = typeof args.topic === "string" ? args.topic.trim() : undefined;
        const page = await queryGutendexBooks(ctx.config, { search: query, topic });
        if (page.items.length === 0) {
          return `No Project Gutenberg EPUBs found matching "${query}".`;
        }
        const lines = page.items.slice(0, 12).map((item) => {
          const authorStr = (item.authors ?? ["Unknown"]).join(", ");
          const genreStr = item.genres && item.genres.length > 0 ? ` [${item.genres.slice(0, 2).join(", ")}]` : "";
          return `- [pg:${item.external_id}] "${item.title}" by ${authorStr} (${item.language ?? "en"})${genreStr}`;
        });
        return truncateForMCP(
          `Found ${page.total_count ?? page.items.length} Project Gutenberg result(s) (showing top ${lines.length}):\n${lines.join("\n")}`,
          3500
        );
      },
    },
  },

  catalog: {
    descriptor: {
      modes: ["browse", "search"],
      supported_formats: ["epub"],
      search_hint: "Search Project Gutenberg titles, authors, or subjects...",
      requires_auth: false,
    },

    async browse(req, ctx) {
      return await queryGutendexBooks(ctx.config, {
        topic: req.group || "popular",
        cursor: req.cursor,
        search: req.filter,
      });
    },

    async search(req, ctx) {
      const topic = typeof req.filters?.group === "string" ? req.filters.group : undefined;
      return await queryGutendexBooks(ctx.config, {
        search: req.query,
        topic,
        cursor: req.cursor,
      });
    },

    async stage(req, ctx) {
      const baseUrl = resolveBaseUrl(ctx.config);
      const preferImages = ctx.config.prefer_images !== false;
      const batchId = `pg-${Date.now()}`;
      const results: CatalogStagedEntry[] = [];
      const total = req.items.length;

      for (let i = 0; i < total; i += 1) {
        const item = req.items[i];
        const extId = item.external_id.trim();
        ctx.reportProgress({
          batchId,
          current: i + 1,
          total,
          externalId: extId,
          status: "downloading",
          message: `Fetching Project Gutenberg #${extId}`,
        });

        try {
          const metaRes = await politeFetch(`${baseUrl}/books/${encodeURIComponent(extId)}`, {
            headers: { Accept: "application/json" },
          });
          if (!metaRes.ok) {
            throw new Error(`Metadata lookup failed (HTTP ${metaRes.status})`);
          }
          const book = (await metaRes.json()) as GutendexBook;
          const epubUrl = selectEpubUrl(book.formats, preferImages);
          if (!epubUrl) {
            throw new Error(`No downloadable EPUB format available for #${extId}`);
          }

          const epubRes = await politeFetch(epubUrl);
          if (!epubRes.ok) {
            throw new Error(`EPUB download failed (HTTP ${epubRes.status})`);
          }
          const epubBytes = new Uint8Array(await epubRes.arrayBuffer());

          let coverPayload: { data: Uint8Array; filename: string; mimeType: string } | undefined;
          const coverUrl = selectCoverUrl(book.formats);
          if (coverUrl) {
            try {
              const covRes = await politeFetch(coverUrl);
              if (covRes.ok) {
                const covMime = covRes.headers.get("content-type") ?? "image/jpeg";
                coverPayload = {
                  data: new Uint8Array(await covRes.arrayBuffer()),
                  filename: covMime.includes("png") ? "cover.png" : "cover.jpg",
                  mimeType: covMime,
                };
              }
            } catch {
              // Cover is optional; proceed with EPUB staging even if external cover fetch fails
            }
          }

          const authors = (book.authors ?? []).map((a) => normalizeGutenbergAuthor(a.name)).filter(Boolean);
          const summary = book.summaries?.[0]?.trim();
          const genres = extractCleanGenres(book);

          const staged = await ctx.stageBook({
            data: epubBytes,
            filename: sanitizeFilename(book.title, extId),
            mimeType: "application/epub+zip",
            externalId: extId,
            cover: coverPayload,
            metadataDefaults: {
              title: book.title,
              authors: authors.length > 0 ? authors : ["Unknown"],
              description: summary,
              publisher: "Project Gutenberg",
              language: book.languages?.[0] ?? "en",
              genres,
            },
          });

          results.push({
            external_id: extId,
            title: book.title,
            upload_job_id: staged.job_id,
            status: "staged",
          });

          ctx.reportProgress({
            batchId,
            current: i + 1,
            total,
            externalId: extId,
            title: book.title,
            status: "staged",
            message: `Staged "${book.title}" for review`,
          });
        } catch (err) {
          const errMsg = err instanceof Error ? err.message : String(err);
          results.push({
            external_id: extId,
            status: "failed",
            error: errMsg,
          });
          ctx.reportProgress({
            batchId,
            current: i + 1,
            total,
            externalId: extId,
            status: "failed",
            message: errMsg,
          });
        }
      }

      return {
        batch_id: batchId,
        async: false,
        results,
      };
    },
  },
});
