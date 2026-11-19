# Authenticated storefront recipes

Use these patterns when building Shelfd catalog plugins in external repositories for commercial ebook and audiobook storefronts.

## Recipe 1: Cookie-authenticated bundle storefront (Humble Bundle pattern)

Storefronts protected by Cloudflare or CAPTCHA login pages block headless password login, so authenticate with the user's session cookie (`_simpleauth_sess`) extracted via Smart cURL Paste.

### `plugin.json` `config_schema`

```json
{
  "type": "object",
  "required": ["session_cookie"],
  "properties": {
    "session_cookie": {
      "type": "string",
      "title": "Humble Bundle Session (_simpleauth_sess)",
      "description": "Open Humble Bundle -> Library in your browser, copy any network request as cURL, and paste it here.",
      "x-shelfd-secret": true,
      "x-shelfd-curl-paste": true,
      "x-shelfd-cookie-name": "_simpleauth_sess"
    },
    "preferred_format": {
      "type": "string",
      "title": "Preferred Format",
      "enum": ["epub", "cbz", "pdf"],
      "default": "epub"
    }
  }
}
```

### Handler guidelines

1. **Build Cookie header**: Format `Cookie: _simpleauth_sess=${quoteIfNeeded(ctx.config.session_cookie)}`. If `ctx.config.session_cookie` is empty, throw `new AuthExpiredError("Configure your _simpleauth_sess cookie first.")`.
2. **Detect expired sessions**: Check both `res.status === 401 || res.status === 403` and `res.headers.get("content-type")?.includes("text/html")` when calling JSON endpoints (`/api/v1/user/order`, `/api/v1/orders?all_tpkds=true`). Redirects to `/login` return HTTP 200 HTML; throw `new AuthExpiredError("Humble Bundle session expired. Paste a fresh cURL command.")`.
3. **Cache order metadata in memory or `ctx.dataDir`**: Humble Bundle returns order gamekeys first, then bulk order details. Cache normalized items keyed by `ctx.userId` for 5 minutes so group filtering (`group: order.product.human_name`) and `catalog/stage` do not re-fetch every order.
4. **Filter to supported Shelfd formats**: Inspect `subproduct.downloads[].download_struct` and include items with `epub`, `m4b`, `mp3`, `m4a`, or `flac` URLs (`url.web`). Map bundle names to `CatalogGroup[]` so users can filter by bundle in Shelfd.
5. **Fresh signed URLs on `catalog/stage`**: Signed download URLs (`dl.humble.com`) expire quickly. Re-fetch the specific order JSON right before downloading binary bytes if cached URLs are older than 2 minutes, then call `ctx.stageBook(...)` with `externalId: "${gamekey}:${machine_name}"` and `metadataDefaults: { title: subproduct.human_name, authors: [subproduct.payee.human_name], publisher: "Humble Bundle" }`.

```ts
if (res.status === 401 || res.status === 403 || !(res.headers.get("content-type") ?? "").includes("json")) {
  throw new AuthExpiredError("Session cookie rejected or expired. Paste a fresh cURL command in plugin settings.");
}
```

---

## Recipe 2: Bearer + refresh-token entitlement API (Packt Pub pattern)

Storefronts with JWT access tokens and refresh endpoints should rotate tokens automatically using `ctx.patchConfig()`.

### `plugin.json` `config_schema`

```json
{
  "type": "object",
  "required": ["access_token"],
  "properties": {
    "access_token": {
      "type": "string",
      "title": "Bearer Access Token",
      "description": "Paste your Bearer token or Copy-as-cURL from an authenticated API request.",
      "x-shelfd-secret": true,
      "x-shelfd-curl-paste": true
    },
    "refresh_token": {
      "type": "string",
      "title": "Refresh Token (Optional)",
      "description": "Enables automatic token renewal when the short-lived access token expires.",
      "x-shelfd-secret": true
    }
  }
}
```

### Token refresh wrapper

```ts
async function fetchWithRefresh(
  url: string,
  ctx: PluginContext<{ access_token?: string; refresh_token?: string }>,
  politeFetch: typeof fetch
): Promise<Response> {
  let token = ctx.config.access_token?.trim();
  if (!token) {
    throw new AuthExpiredError("Access token is required.");
  }

  let res = await politeFetch(url, {
    headers: { Authorization: `Bearer ${token}`, Accept: "application/json" },
  });

  if (res.status === 401 && ctx.config.refresh_token) {
    const refreshRes = await politeFetch("https://services.packtpub.com/auth-v1/users/me/tokens", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ refresh: ctx.config.refresh_token }),
    });

    if (!refreshRes.ok) {
      throw new AuthExpiredError("Refresh token expired. Paste a new Bearer token in plugin settings.");
    }

    const refreshed = (await refreshRes.json()) as { data?: { access?: string; refresh?: string } };
    const nextAccess = refreshed.data?.access;
    const nextRefresh = refreshed.data?.refresh ?? ctx.config.refresh_token;
    if (!nextAccess) {
      throw new AuthExpiredError("Token refresh response did not include an access token.");
    }

    // Persist encrypted rotated tokens in Shelfd before retrying
    await ctx.patchConfig({
      access_token: nextAccess,
      refresh_token: nextRefresh,
    });
    token = nextAccess;

    res = await politeFetch(url, {
      headers: { Authorization: `Bearer ${token}`, Accept: "application/json" },
    });
  }

  if (res.status === 401 || res.status === 403) {
    throw new AuthExpiredError("Access token expired. Update credentials in plugin settings.");
  }
  return res;
}
```

---

## Recipe 3: OPDS 1.2 / 2.0 catalog feed

For self-hosted or indie OPDS feeds (Standard Ebooks, Calibre-Web, Komga, Feedbooks):

1. Configure `feed_url` (`type: "string"`), optional `username`, and optional `password` (`"x-shelfd-secret": true`).
2. Map OPDS navigation links (`rel="subsection"` or `type="application/atom+xml;profile=opds-catalog;kind=navigation"`) to `CatalogGroup[]`, and acquisition entries (`rel="http://opds-spec.org/acquisition"`) with `application/epub+zip` or `audio/mp4` to `CatalogItem[]`.
3. Encode the next-page `<link rel="next" href="..."/>` URL as `next_cursor` so Shelfd's "Load More Books" button paginates cleanly through the feed.
