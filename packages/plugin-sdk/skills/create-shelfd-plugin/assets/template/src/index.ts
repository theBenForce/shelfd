import {
  AuthExpiredError,
  createPoliteFetcher,
  definePlugin,
  type CatalogItem,
  type CatalogPage,
  type PluginDefinition,
} from "@thebenforce/shelfd-plugin-sdk";

export interface StarterConfig extends Record<string, unknown> {
  api_token?: string;
}

export function createStarterPlugin(fetchImpl?: typeof fetch): PluginDefinition<StarterConfig> {
  const politeFetch = createPoliteFetcher({ minIntervalMs: 350, fetchImpl });

  function requireToken(config: StarterConfig): string {
    const token = typeof config.api_token === "string" ? config.api_token.trim() : "";
    if (!token) {
      throw new AuthExpiredError("Configure your API token in Settings -> Plugins & Storefronts.");
    }
    return token;
  }

  return {
    name: "starter-catalog",
    version: "0.1.0",

    commands: {
      verify_session: {
        title: "Verify Credentials",
        description: "Checks whether the configured credential is accepted by the remote service.",
        async execute(_args, ctx) {
          const token = requireToken(ctx.config);
          return { ok: true, message: `Credential present (${token.length} chars).` };
        },
      },
    },

    catalog: {
      descriptor: {
        modes: ["browse", "search"],
        supported_formats: ["epub"],
        search_hint: "Search books...",
        requires_auth: true,
      },

      async browse(req, ctx): Promise<CatalogPage> {
        const token = requireToken(ctx.config);
        const url = new URL("https://api.example.com/v1/library");
        if (req.cursor) url.searchParams.set("cursor", req.cursor);
        if (req.group) url.searchParams.set("group", req.group);

        const res = await politeFetch(url, {
          headers: { Authorization: `Bearer ${token}`, Accept: "application/json" },
        });
        if (res.status === 401 || res.status === 403) {
          throw new AuthExpiredError("Remote session expired. Update your token in plugin settings.");
        }
        if (!res.ok) {
          throw new Error(`Catalog browse failed with HTTP ${res.status}`);
        }

        const data = (await res.json()) as { items?: CatalogItem[]; next_cursor?: string };
        return {
          items: data.items ?? [],
          next_cursor: data.next_cursor,
        };
      },

      async search(req, ctx): Promise<CatalogPage> {
        const token = requireToken(ctx.config);
        const url = new URL("https://api.example.com/v1/library/search");
        url.searchParams.set("q", req.query);
        if (req.cursor) url.searchParams.set("cursor", req.cursor);

        const res = await politeFetch(url, {
          headers: { Authorization: `Bearer ${token}`, Accept: "application/json" },
        });
        if (res.status === 401 || res.status === 403) {
          throw new AuthExpiredError("Remote session expired. Update your token in plugin settings.");
        }
        if (!res.ok) {
          throw new Error(`Catalog search failed with HTTP ${res.status}`);
        }

        const data = (await res.json()) as { items?: CatalogItem[]; next_cursor?: string };
        return {
          items: data.items ?? [],
          next_cursor: data.next_cursor,
        };
      },

      async stage(req, ctx) {
        const token = requireToken(ctx.config);
        const results = [];

        for (let i = 0; i < req.items.length; i += 1) {
          const item = req.items[i];
          ctx.reportProgress({
            current: i + 1,
            total: req.items.length,
            externalId: item.external_id,
            message: `Downloading ${item.external_id} (${i + 1}/${req.items.length})`,
          });

          try {
            const dlRes = await politeFetch(
              `https://api.example.com/v1/library/${encodeURIComponent(item.external_id)}/download`,
              { headers: { Authorization: `Bearer ${token}` } }
            );
            if (dlRes.status === 401 || dlRes.status === 403) {
              throw new AuthExpiredError("Remote session expired during download.");
            }
            if (!dlRes.ok) {
              throw new Error(`Download returned HTTP ${dlRes.status}`);
            }

            const bytes = new Uint8Array(await dlRes.arrayBuffer());
            const staged = await ctx.stageBook({
              data: bytes,
              filename: `${item.external_id}.epub`,
              mimeType: "application/epub+zip",
              externalId: item.external_id,
              metadataDefaults: {
                title: item.external_id,
              },
            });

            results.push({
              external_id: item.external_id,
              title: staged.metadata.title,
              upload_job_id: staged.job_id,
              status: "staged" as const,
            });
          } catch (err) {
            if (err instanceof AuthExpiredError) {
              throw err;
            }
            results.push({
              external_id: item.external_id,
              status: "failed" as const,
              error: err instanceof Error ? err.message : String(err),
            });
          }
        }

        return { async: false, results };
      },
    },
  };
}

// Start the stdio JSON-RPC loop when invoked by Shelfd
if (import.meta.url === `file://${process.argv[1]}`) {
  definePlugin(createStarterPlugin());
}
