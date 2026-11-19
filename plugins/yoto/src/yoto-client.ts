import { AuthExpiredError, type PluginContext } from "@shelfd/plugin-sdk";
import { normalizeIcon16x16, normalizeYotoIconUri } from "./image-16x16.ts";

export const DEFAULT_YOTO_API_BASE_URL = "https://api.yotoplay.com";
export const DEFAULT_YOTO_AUTH_BASE_URL = "https://login.yotoplay.com";
export const DEFAULT_BOOK_ICON_URI = "yoto:#gTMbacpoeSMYqc9fNLJnxPjylraNG6jIrYEWevyzYbA";

export interface YotoPluginConfig extends Record<string, unknown> {
  client_id?: string;
  client_secret?: string;
  redirect_uri?: string;
  api_base_url?: string;
  auth_base_url?: string;
  access_token?: string;
  refresh_token?: string;
  token_expires_at?: string;
  poll_interval_ms?: number;
}

export interface TokenResponse {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
  expires_at: string;
}

export interface YotoDisplayFields {
  icon16x16: string | null;
}

export interface YotoTrack {
  key: string;
  title: string;
  trackUrl: string;
  duration?: number;
  fileSize?: number;
  channels?: string;
  format?: string;
  type: string;
  uid?: string;
  overlayLabel?: string;
  display: YotoDisplayFields;
}

export interface YotoChapter {
  key: string;
  title: string;
  overlayLabel?: string;
  duration?: number;
  fileSize?: number;
  tracks: YotoTrack[];
  display: YotoDisplayFields;
}

export interface YotoCardCover {
  imageL?: string;
}

export interface YotoCardMediaMetadata {
  duration?: number;
  fileSize?: number;
  readableFileSize?: number;
}

export interface YotoCardMetadata {
  author?: string;
  category?: string;
  description?: string;
  cover?: YotoCardCover;
  media?: YotoCardMediaMetadata;
  creators?: Record<string, unknown>;
}

export interface YotoCardContentBody {
  chapters: YotoChapter[];
  config?: Record<string, unknown>;
  playbackType?: string;
  activity?: string;
  version?: string;
  cover?: YotoCardCover;
}

export interface YotoCard {
  cardId: string;
  title: string;
  userId?: string;
  slug?: string;
  deleted?: boolean;
  createdAt?: string;
  updatedAt?: string;
  content: YotoCardContentBody;
  metadata: YotoCardMetadata;
}

export interface YotoPlaylistSummary {
  cardId: string;
  title: string;
  description: string;
  coverImageUrl: string;
  chapterCount: number;
  updatedAt?: string;
}

export interface DisplayIcon {
  displayIconId: string;
  mediaId: string;
  title?: string;
  url?: string;
  public: boolean;
  publicTags?: string[];
  userId?: string;
  createdAt?: string;
  yotoUri: string;
}

export interface UploadURLResponse {
  uploadUrl: string;
  uploadId: string;
}

export interface TranscodedAudio {
  transcodedSha256: string;
  transcodedInfo: {
    duration?: number;
    fileSize?: number;
    channels?: string;
    format?: string;
    metadata?: {
      title?: string;
    };
  };
}

export function resolveApiBaseUrl(cfg: YotoPluginConfig): string {
  const raw = typeof cfg.api_base_url === "string" && cfg.api_base_url.trim() ? cfg.api_base_url.trim() : DEFAULT_YOTO_API_BASE_URL;
  return raw.replace(/\/+$/, "");
}

export function resolveAuthBaseUrl(cfg: YotoPluginConfig): string {
  const raw = typeof cfg.auth_base_url === "string" && cfg.auth_base_url.trim() ? cfg.auth_base_url.trim() : DEFAULT_YOTO_AUTH_BASE_URL;
  return raw.replace(/\/+$/, "");
}

export function parseJwtExpiry(accessToken: string, expiresInSeconds?: number): string {
  if (typeof expiresInSeconds === "number" && expiresInSeconds > 0) {
    return new Date(Date.now() + expiresInSeconds * 1000).toISOString();
  }
  const parts = accessToken.split(".");
  if (parts.length === 3 && parts[1]) {
    try {
      const jsonStr = Buffer.from(parts[1], "base64url").toString("utf8");
      const parsed = JSON.parse(jsonStr) as { exp?: number };
      if (typeof parsed.exp === "number" && parsed.exp > 0) {
        return new Date(parsed.exp * 1000).toISOString();
      }
    } catch {
      return new Date(Date.now() + 3600 * 1000).toISOString();
    }
  }
  return new Date(Date.now() + 3600 * 1000).toISOString();
}

export async function exchangeCode(params: {
  authBaseUrl: string;
  clientId: string;
  clientSecret?: string;
  code: string;
  codeVerifier?: string;
  redirectUri?: string;
}): Promise<TokenResponse> {
  const form = new URLSearchParams();
  form.set("grant_type", "authorization_code");
  form.set("client_id", params.clientId);
  form.set("code", params.code);
  if (params.clientSecret) {
    form.set("client_secret", params.clientSecret);
  }
  if (params.codeVerifier) {
    form.set("code_verifier", params.codeVerifier);
  }
  if (params.redirectUri) {
    form.set("redirect_uri", params.redirectUri);
  }

  const res = await fetch(`${params.authBaseUrl.replace(/\/+$/, "")}/oauth/token`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Accept: "application/json",
    },
    body: form.toString(),
  });

  const bodyText = await res.text();
  if (!res.ok) {
    throw new Error(`token exchange failed with status ${res.status}: ${bodyText.slice(0, 500).trim()}`);
  }

  const raw = JSON.parse(bodyText) as Record<string, unknown>;
  const accessToken = String(raw.access_token ?? "");
  const refreshToken = String(raw.refresh_token ?? "");
  const expiresIn = typeof raw.expires_in === "number" ? raw.expires_in : Number(raw.expires_in ?? 0);
  const expiresAt = parseJwtExpiry(accessToken, expiresIn);

  return {
    access_token: accessToken,
    refresh_token: refreshToken,
    token_type: String(raw.token_type ?? "Bearer"),
    expires_in: expiresIn,
    expires_at: expiresAt,
  };
}

export async function refreshToken(params: {
  authBaseUrl: string;
  clientId: string;
  clientSecret?: string;
  refreshToken: string;
}): Promise<TokenResponse> {
  const form = new URLSearchParams();
  form.set("grant_type", "refresh_token");
  form.set("client_id", params.clientId);
  form.set("refresh_token", params.refreshToken);
  if (params.clientSecret) {
    form.set("client_secret", params.clientSecret);
  }

  const res = await fetch(`${params.authBaseUrl.replace(/\/+$/, "")}/oauth/token`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Accept: "application/json",
    },
    body: form.toString(),
  });

  const bodyText = await res.text();
  if (res.status === 401 || res.status === 403) {
    throw new AuthExpiredError("Yoto account is not linked or session expired. Configure Yoto in Settings -> Plugins.");
  }
  if (!res.ok) {
    throw new Error(`token refresh failed with status ${res.status}: ${bodyText.slice(0, 500).trim()}`);
  }

  const raw = JSON.parse(bodyText) as Record<string, unknown>;
  const accessToken = String(raw.access_token ?? "");
  const nextRefresh = typeof raw.refresh_token === "string" && raw.refresh_token ? raw.refresh_token : params.refreshToken;
  const expiresIn = typeof raw.expires_in === "number" ? raw.expires_in : Number(raw.expires_in ?? 0);
  const expiresAt = parseJwtExpiry(accessToken, expiresIn);

  return {
    access_token: accessToken,
    refresh_token: nextRefresh,
    token_type: String(raw.token_type ?? "Bearer"),
    expires_in: expiresIn,
    expires_at: expiresAt,
  };
}

export async function ensureValidAccessToken(ctx: PluginContext<YotoPluginConfig>): Promise<string> {
  const currentAccess = typeof ctx.config.access_token === "string" ? ctx.config.access_token.trim() : "";
  const currentRefresh = typeof ctx.config.refresh_token === "string" ? ctx.config.refresh_token.trim() : "";
  const clientId = typeof ctx.config.client_id === "string" && ctx.config.client_id.trim() ? ctx.config.client_id.trim() : "shelfd-yoto";
  const clientSecret = typeof ctx.config.client_secret === "string" ? ctx.config.client_secret.trim() : undefined;
  const expiresAtRaw = typeof ctx.config.token_expires_at === "string" ? ctx.config.token_expires_at.trim() : "";

  if (!currentAccess && !currentRefresh) {
    throw new AuthExpiredError("Yoto account is not linked or session expired. Configure Yoto in Settings -> Plugins.");
  }

  let isExpiring = !currentAccess;
  if (expiresAtRaw) {
    const expMs = Date.parse(expiresAtRaw);
    if (!Number.isNaN(expMs) && Date.now() + 2 * 60 * 1000 >= expMs) {
      isExpiring = true;
    }
  }

  if (isExpiring && currentRefresh && clientId !== "manual-token") {
    try {
      const refreshed = await refreshToken({
        authBaseUrl: resolveAuthBaseUrl(ctx.config),
        clientId,
        clientSecret,
        refreshToken: currentRefresh,
      });
      if (refreshed.access_token) {
        ctx.config.access_token = refreshed.access_token;
        ctx.config.refresh_token = refreshed.refresh_token;
        ctx.config.token_expires_at = refreshed.expires_at;
        await ctx.patchConfig({
          access_token: refreshed.access_token,
          refresh_token: refreshed.refresh_token,
          token_expires_at: refreshed.expires_at,
        });
        return refreshed.access_token;
      }
    } catch (err) {
      if (err instanceof AuthExpiredError || !currentAccess) {
        throw new AuthExpiredError("Yoto account is not linked or session expired. Configure Yoto in Settings -> Plugins.");
      }
    }
  }

  if (!currentAccess) {
    throw new AuthExpiredError("Yoto account is not linked or session expired. Configure Yoto in Settings -> Plugins.");
  }

  return currentAccess;
}

function normalizeDisplay(rawDisplay: unknown): YotoDisplayFields {
  if (!rawDisplay || typeof rawDisplay !== "object") {
    return { icon16x16: null };
  }
  const obj = rawDisplay as Record<string, unknown>;
  const norm = normalizeYotoIconUri(typeof obj.icon16x16 === "string" ? obj.icon16x16 : "");
  return { icon16x16: norm || null };
}

function normalizeChannels(rawChannels: unknown): string | undefined {
  if (typeof rawChannels === "string" && rawChannels.trim()) {
    return rawChannels.trim();
  }
  if (rawChannels === 1) return "mono";
  if (rawChannels === 2) return "stereo";
  return undefined;
}

export function decodeYotoCard(rawBody: unknown, fallbackCardId = ""): YotoCard {
  const root = (rawBody && typeof rawBody === "object" ? rawBody : {}) as Record<string, unknown>;
  const candidate =
    root.card && typeof root.card === "object" ? (root.card as Record<string, unknown>) : root;

  const cardId = String(candidate.cardId ?? fallbackCardId ?? "");
  const title = String(candidate.title ?? "");
  const rawContent = (candidate.content && typeof candidate.content === "object" ? candidate.content : {}) as Record<string, unknown>;
  const rawMetadata = (candidate.metadata && typeof candidate.metadata === "object" ? candidate.metadata : {}) as Record<string, unknown>;

  const rawChapters = Array.isArray(rawContent.chapters) ? rawContent.chapters : [];
  const chapters: YotoChapter[] = rawChapters.map((chRaw, idx) => {
    const ch = (chRaw && typeof chRaw === "object" ? chRaw : {}) as Record<string, unknown>;
    const rawTracks = Array.isArray(ch.tracks) ? ch.tracks : [];
    const tracks: YotoTrack[] = rawTracks.map((trRaw, tIdx) => {
      const tr = (trRaw && typeof trRaw === "object" ? trRaw : {}) as Record<string, unknown>;
      return {
        key: String(tr.key ?? String(tIdx + 1).padStart(2, "0")),
        title: String(tr.title ?? ""),
        trackUrl: String(tr.trackUrl ?? ""),
        duration: typeof tr.duration === "number" ? tr.duration : undefined,
        fileSize: typeof tr.fileSize === "number" ? tr.fileSize : undefined,
        channels: normalizeChannels(tr.channels),
        format: typeof tr.format === "string" ? tr.format : undefined,
        type: String(tr.type ?? "audio"),
        uid: typeof tr.uid === "string" ? tr.uid : undefined,
        overlayLabel: typeof tr.overlayLabel === "string" ? tr.overlayLabel : undefined,
        display: normalizeDisplay(tr.display),
      };
    });

    return {
      key: String(ch.key ?? String(idx + 1).padStart(2, "0")),
      title: String(ch.title ?? ""),
      overlayLabel: typeof ch.overlayLabel === "string" ? ch.overlayLabel : undefined,
      duration: typeof ch.duration === "number" ? ch.duration : undefined,
      fileSize: typeof ch.fileSize === "number" ? ch.fileSize : undefined,
      tracks,
      display: normalizeDisplay(ch.display),
    };
  });

  let cover: YotoCardCover | undefined;
  if (rawMetadata.cover && typeof rawMetadata.cover === "object") {
    const cov = rawMetadata.cover as Record<string, unknown>;
    if (typeof cov.imageL === "string" && cov.imageL) {
      cover = { imageL: cov.imageL };
    }
  } else if (rawContent.cover && typeof rawContent.cover === "object") {
    const cov = rawContent.cover as Record<string, unknown>;
    if (typeof cov.imageL === "string" && cov.imageL) {
      cover = { imageL: cov.imageL };
    }
  }

  let media: YotoCardMediaMetadata | undefined;
  if (rawMetadata.media && typeof rawMetadata.media === "object") {
    const m = rawMetadata.media as Record<string, unknown>;
    media = {
      duration: typeof m.duration === "number" ? m.duration : undefined,
      fileSize: typeof m.fileSize === "number" ? m.fileSize : undefined,
      readableFileSize: typeof m.readableFileSize === "number" ? m.readableFileSize : undefined,
    };
  }

  return {
    cardId,
    title,
    userId: typeof candidate.userId === "string" ? candidate.userId : undefined,
    slug: typeof candidate.slug === "string" ? candidate.slug : undefined,
    deleted: Boolean(candidate.deleted),
    createdAt: typeof candidate.createdAt === "string" ? candidate.createdAt : undefined,
    updatedAt: typeof candidate.updatedAt === "string" ? candidate.updatedAt : undefined,
    content: {
      chapters,
      config: rawContent.config && typeof rawContent.config === "object" ? (rawContent.config as Record<string, unknown>) : undefined,
      playbackType: typeof rawContent.playbackType === "string" ? rawContent.playbackType : undefined,
      activity: typeof rawContent.activity === "string" ? rawContent.activity : undefined,
      version: typeof rawContent.version === "string" ? rawContent.version : undefined,
    },
    metadata: {
      author: typeof rawMetadata.author === "string" ? rawMetadata.author : undefined,
      category: typeof rawMetadata.category === "string" ? rawMetadata.category : undefined,
      description: typeof rawMetadata.description === "string" ? rawMetadata.description : undefined,
      cover,
      media,
      creators: rawMetadata.creators && typeof rawMetadata.creators === "object" ? (rawMetadata.creators as Record<string, unknown>) : undefined,
    },
  };
}

export async function getCard(apiBaseUrl: string, accessToken: string, cardId: string): Promise<YotoCard> {
  const base = apiBaseUrl.replace(/\/+$/, "");
  const encodedId = encodeURIComponent(cardId);
  const endpoints = [`${base}/content/${encodedId}`, `${base}/card/${encodedId}`];

  let lastErr: Error | null = null;
  let bestCard: YotoCard | null = null;

  for (const endpoint of endpoints) {
    try {
      const res = await fetch(endpoint, {
        method: "GET",
        headers: {
          Authorization: `Bearer ${accessToken}`,
          Accept: "application/json",
        },
      });
      if (res.status === 401 || res.status === 403) {
        throw new AuthExpiredError("Yoto session expired. Please re-link your Yoto account.");
      }
      const text = await res.text();
      if (!res.ok) {
        lastErr = new Error(`fetching card ${cardId} failed with status ${res.status}: ${text.slice(0, 300).trim()}`);
        continue;
      }
      const card = decodeYotoCard(JSON.parse(text), cardId);
      if (card.content.chapters.length > 0) {
        return card;
      }
      if (!bestCard) {
        bestCard = card;
      }
    } catch (err) {
      if (err instanceof AuthExpiredError) throw err;
      lastErr = err instanceof Error ? err : new Error(String(err));
    }
  }

  if (bestCard) return bestCard;
  throw lastErr ?? new Error(`Failed to fetch card ${cardId}`);
}

export async function getMyPlaylists(apiBaseUrl: string, accessToken: string): Promise<YotoPlaylistSummary[]> {
  const base = apiBaseUrl.replace(/\/+$/, "");
  const res = await fetch(`${base}/content/mine`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      Accept: "application/json",
    },
  });
  if (res.status === 401 || res.status === 403) {
    throw new AuthExpiredError("Yoto session expired. Please re-link your Yoto account.");
  }
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`fetching playlists failed with status ${res.status}: ${text.slice(0, 300).trim()}`);
  }

  const parsed = JSON.parse(text) as { cards?: unknown[] };
  const rawCards = Array.isArray(parsed.cards) ? parsed.cards : [];
  const activeCards: YotoCard[] = [];

  for (const raw of rawCards) {
    const card = decodeYotoCard(raw);
    if (!card.deleted && card.cardId) {
      activeCards.push(card);
    }
  }

  await Promise.all(
    activeCards.map(async (card, idx) => {
      if (card.content.chapters.length === 0 && card.cardId) {
        try {
          const detail = await getCard(base, accessToken, card.cardId);
          if (detail.content.chapters.length > 0) {
            activeCards[idx].content = detail.content;
          }
          if (!activeCards[idx].metadata.cover && detail.metadata.cover) {
            activeCards[idx].metadata.cover = detail.metadata.cover;
          }
          if (!activeCards[idx].metadata.description && detail.metadata.description) {
            activeCards[idx].metadata.description = detail.metadata.description;
          }
        } catch {
          void 0;
        }
      }
    })
  );

  return activeCards.map((card) => ({
    cardId: card.cardId,
    title: card.title || card.cardId,
    description: card.metadata.description ?? "",
    coverImageUrl: card.metadata.cover?.imageL ?? "",
    chapterCount: card.content.chapters.length,
    updatedAt: card.updatedAt,
  }));
}

export async function createOrUpdateContent(apiBaseUrl: string, accessToken: string, content: Partial<YotoCard>): Promise<YotoCard> {
  const base = apiBaseUrl.replace(/\/+$/, "");
  const normalizedChapters = (content.content?.chapters ?? []).map((ch) => ({
    ...ch,
    display: {
      icon16x16: normalizeYotoIconUri(ch.display?.icon16x16) || null,
    },
    tracks: (ch.tracks ?? []).map((tr) => ({
      ...tr,
      display: {
        icon16x16: normalizeYotoIconUri(tr.display?.icon16x16) || null,
      },
    })),
  }));

  const payload = {
    ...content,
    content: {
      ...content.content,
      chapters: normalizedChapters,
    },
  };

  const res = await fetch(`${base}/content`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    body: JSON.stringify(payload),
  });
  if (res.status === 401 || res.status === 403) {
    throw new AuthExpiredError("Yoto session expired. Please re-link your Yoto account.");
  }
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`create/update content failed with status ${res.status}: ${text.slice(0, 400).trim()}`);
  }
  const saved = decodeYotoCard(JSON.parse(text), content.cardId ?? "");
  if (!saved.cardId && content.cardId) {
    saved.cardId = content.cardId;
  }
  if (!saved.title && content.title) {
    saved.title = content.title;
  }
  return saved;
}

function cleanDisplayIconMediaId(rawMediaId: string, rawUrl: string): string {
  const normMedia = normalizeYotoIconUri(rawMediaId);
  if (normMedia) {
    return normMedia.replace(/^yoto:#/, "");
  }
  const normUrl = normalizeYotoIconUri(rawUrl);
  if (normUrl) {
    return normUrl.replace(/^yoto:#/, "");
  }
  return rawMediaId.trim().replace(/^yoto:#/, "").replace(/^yoto:/, "").replace(/^#/, "");
}

function mapRawDisplayIcon(raw: Record<string, unknown>, isPublicDefault: boolean): DisplayIcon {
  const rawMediaId = String(raw.mediaId ?? "");
  const rawUrl = typeof raw.url === "string" ? raw.url : "";
  const mediaId = cleanDisplayIconMediaId(rawMediaId, rawUrl);
  const yotoUri = normalizeYotoIconUri(mediaId) || (mediaId ? `yoto:#${mediaId}` : "");

  return {
    displayIconId: String(raw.displayIconId ?? ""),
    mediaId,
    title: typeof raw.title === "string" ? raw.title : undefined,
    url: rawUrl || undefined,
    public: typeof raw.public === "boolean" ? raw.public : isPublicDefault,
    publicTags: Array.isArray(raw.publicTags) ? raw.publicTags.map((t) => String(t)) : undefined,
    userId: typeof raw.userId === "string" ? raw.userId : undefined,
    createdAt: typeof raw.createdAt === "string" ? raw.createdAt : undefined,
    yotoUri,
  };
}

export async function getPublicIcons(apiBaseUrl: string, accessToken: string): Promise<DisplayIcon[]> {
  const base = apiBaseUrl.replace(/\/+$/, "");
  const res = await fetch(`${base}/media/displayIcons/user/yoto`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      Accept: "application/json",
    },
  });
  if (res.status === 401 || res.status === 403) {
    throw new AuthExpiredError("Yoto session expired. Please re-link your Yoto account.");
  }
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`fetching public icons failed with status ${res.status}: ${text.slice(0, 300).trim()}`);
  }
  const parsed = JSON.parse(text) as { displayIcons?: Array<Record<string, unknown>> };
  return (parsed.displayIcons ?? []).map((raw) => mapRawDisplayIcon(raw, true));
}

export async function getUserIcons(apiBaseUrl: string, accessToken: string): Promise<DisplayIcon[]> {
  const base = apiBaseUrl.replace(/\/+$/, "");
  const res = await fetch(`${base}/media/displayIcons/user/me`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      Accept: "application/json",
    },
  });
  if (res.status === 401 || res.status === 403) {
    throw new AuthExpiredError("Yoto session expired. Please re-link your Yoto account.");
  }
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`fetching user icons failed with status ${res.status}: ${text.slice(0, 300).trim()}`);
  }
  const parsed = JSON.parse(text) as { displayIcons?: Array<Record<string, unknown>> };
  return (parsed.displayIcons ?? []).map((raw) => mapRawDisplayIcon(raw, false));
}

export async function uploadCustomIcon(
  apiBaseUrl: string,
  accessToken: string,
  imageBytes: Uint8Array,
  filename = "icon.png"
): Promise<DisplayIcon> {
  const normalizedPng = normalizeIcon16x16(imageBytes);
  const base = apiBaseUrl.replace(/\/+$/, "");
  const url = new URL(`${base}/media/displayIcons/user/me/upload`);
  url.searchParams.set("autoConvert", "true");
  const stem = filename.includes(".") ? filename.slice(0, filename.lastIndexOf(".")) : filename;
  if (stem) {
    url.searchParams.set("filename", stem);
  }

  const res = await fetch(url.toString(), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "image/png",
      Accept: "application/json",
    },
    body: new Uint8Array(normalizedPng),
  });
  if (res.status === 401 || res.status === 403) {
    throw new AuthExpiredError("Yoto session expired. Please re-link your Yoto account.");
  }
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`uploading custom icon failed with status ${res.status}: ${text.slice(0, 300).trim()}`);
  }
  const parsed = JSON.parse(text) as { displayIcon?: Record<string, unknown> };
  const icon = mapRawDisplayIcon(parsed.displayIcon ?? {}, false);
  if (!icon.title) {
    icon.title = filename;
  }
  return icon;
}

export async function uploadCoverImage(
  apiBaseUrl: string,
  accessToken: string,
  imageBytes: Uint8Array,
  contentType = "image/jpeg"
): Promise<string> {
  const base = apiBaseUrl.replace(/\/+$/, "");
  const res = await fetch(`${base}/media/coverImage/user/me/upload?autoconvert=true&coverType=default`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": contentType || "image/jpeg",
      Accept: "application/json",
    },
    body: new Uint8Array(imageBytes),
  });
  if (res.status === 401 || res.status === 403) {
    throw new AuthExpiredError("Yoto session expired. Please re-link your Yoto account.");
  }
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`uploading cover image failed with status ${res.status}: ${text.slice(0, 300).trim()}`);
  }
  const parsed = JSON.parse(text) as { coverImage?: { mediaUrl?: string } };
  return String(parsed.coverImage?.mediaUrl ?? "");
}

export async function requestAudioUploadUrl(apiBaseUrl: string, accessToken: string): Promise<UploadURLResponse> {
  const base = apiBaseUrl.replace(/\/+$/, "");
  const res = await fetch(`${base}/media/transcode/audio/uploadUrl`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      Accept: "application/json",
    },
  });
  if (res.status === 401 || res.status === 403) {
    throw new AuthExpiredError("Yoto session expired. Please re-link your Yoto account.");
  }
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`requesting upload URL failed with status ${res.status}: ${text.slice(0, 300).trim()}`);
  }
  const parsed = JSON.parse(text) as { upload?: { uploadUrl?: string; uploadId?: string } };
  const uploadUrl = String(parsed.upload?.uploadUrl ?? "");
  const uploadId = String(parsed.upload?.uploadId ?? "");
  if (!uploadUrl) {
    throw new Error("empty uploadUrl returned by Yoto API");
  }
  return { uploadUrl, uploadId };
}

export async function uploadAudioStream(
  uploadUrl: string,
  stream: ReadableStream<Uint8Array>,
  contentType = "audio/mpeg",
  contentLength?: number
): Promise<void> {
  const headers: Record<string, string> = {
    "Content-Type": contentType || "audio/mpeg",
  };
  if (typeof contentLength === "number" && contentLength > 0) {
    headers["Content-Length"] = String(contentLength);
  }

  const res = await fetch(uploadUrl, {
    method: "PUT",
    headers,
    body: stream,
    duplex: "half",
  } as RequestInit);

  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`audio PUT failed with status ${res.status}: ${text.slice(0, 300).trim()}`);
  }
}

export async function waitForTranscoding(
  apiBaseUrl: string,
  accessToken: string,
  uploadId: string,
  maxAttempts = 60,
  pollIntervalMs = 500
): Promise<TranscodedAudio> {
  const base = apiBaseUrl.replace(/\/+$/, "");
  const pollUrl = `${base}/media/upload/${encodeURIComponent(uploadId)}/transcoded?loudnorm=false`;
  const attempts = maxAttempts > 0 ? maxAttempts : 60;

  for (let attempt = 0; attempt < attempts; attempt++) {
    const res = await fetch(pollUrl, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${accessToken}`,
        Accept: "application/json",
      },
    });
    if (res.status === 401 || res.status === 403) {
      throw new AuthExpiredError("Yoto session expired. Please re-link your Yoto account.");
    }
    if (res.ok) {
      const parsed = (await res.json()) as { transcode?: TranscodedAudio };
      if (parsed.transcode?.transcodedSha256) {
        return parsed.transcode;
      }
    } else {
      await res.text().catch(() => "");
    }
    if (attempt + 1 < attempts) {
      await new Promise((resolve) => setTimeout(resolve, pollIntervalMs));
    }
  }

  throw new Error(`transcoding timed out after ${attempts} attempts for upload ${uploadId}`);
}
