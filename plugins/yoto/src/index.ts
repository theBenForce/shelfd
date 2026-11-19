import {
  definePlugin,
  truncateForMCP,
  type ActionEntry,
  type ActionFormSpec,
  type ActionInvokeRequest,
  type ActionResult,
  type AuthBeginRequest,
  type AuthCompleteRequest,
  type AuthStatusResult,
  type OptionItem,
  type PluginContext,
} from "@shelfd/plugin-sdk";
import { normalizeIcon16x16, normalizeYotoIconUri } from "./image-16x16.ts";
import {
  DEFAULT_BOOK_ICON_URI,
  createOrUpdateContent,
  ensureValidAccessToken,
  exchangeCode,
  getCard,
  getMyPlaylists,
  getPublicIcons,
  getUserIcons,
  parseJwtExpiry,
  requestAudioUploadUrl,
  resolveApiBaseUrl,
  resolveAuthBaseUrl,
  uploadAudioStream,
  uploadCoverImage,
  uploadCustomIcon,
  waitForTranscoding,
  type YotoCard,
  type YotoChapter,
  type YotoPluginConfig,
} from "./yoto-client.ts";

export { normalizeIcon16x16, normalizeYotoIconUri };

interface ShelfdBookFile {
  id: string;
  book_id?: string;
  file_type?: string;
  file_path?: string;
  mime_type?: string;
  file_size_bytes?: number;
  duration_seconds?: number;
}

interface ShelfdAudioChapter {
  id?: string;
  chapter_index?: number;
  title?: string;
  start_time_sec?: number;
  end_time_sec?: number;
  duration_sec?: number;
}

interface ShelfdBook {
  id: string;
  title: string;
  description?: string;
  book_type?: string;
  file_path?: string;
  file_size_bytes?: number;
  duration_seconds?: number;
  cover_path?: string;
  files?: ShelfdBookFile[];
  audio_chapters?: ShelfdAudioChapter[];
  sequence_number?: number;
}

interface ShelfdSeriesResponse {
  series: {
    id: string;
    name: string;
    description?: string;
  };
  books: ShelfdBook[];
}

function isAudioExtension(filePath: string | undefined): boolean {
  if (!filePath) return false;
  const lower = filePath.toLowerCase();
  return [".m4b", ".m4a", ".mp3", ".ogg", ".oga", ".flac", ".opus", ".aac", ".wav"].some((ext) => lower.endsWith(ext));
}

function collectBookAudioFiles(book: ShelfdBook): ShelfdBookFile[] {
  const files = Array.isArray(book.files) ? book.files : [];
  const audioFiles = files.filter(
    (f) => f.file_type === "audiobook" || f.file_type === "audio" || isAudioExtension(f.file_path)
  );
  if (audioFiles.length > 0) {
    return audioFiles;
  }
  if (book.book_type === "audiobook" || isAudioExtension(book.file_path)) {
    return [
      {
        id: "",
        book_id: book.id,
        file_type: "audiobook",
        file_path: book.file_path,
        file_size_bytes: book.file_size_bytes,
        duration_seconds: book.duration_seconds,
      },
    ];
  }
  return [];
}

async function fetchLoopbackJson<T>(ctx: PluginContext<YotoPluginConfig>, path: string): Promise<T> {
  const res = await ctx.fetchLoopback(path, {
    headers: { Accept: "application/json" },
  });
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`Loopback GET ${path} failed (HTTP ${res.status}): ${text.slice(0, 200)}`);
  }
  return JSON.parse(text) as T;
}

async function fetchBookAndChapters(
  ctx: PluginContext<YotoPluginConfig>,
  bookId: string
): Promise<{ book: ShelfdBook; audioChapters: ShelfdAudioChapter[] }> {
  const raw = await fetchLoopbackJson<Record<string, unknown>>(ctx, `/api/v1/books/${encodeURIComponent(bookId)}`);
  const bookObj = (raw.book && typeof raw.book === "object" ? raw.book : raw) as Record<string, unknown>;
  const files = Array.isArray(raw.files)
    ? (raw.files as ShelfdBookFile[])
    : Array.isArray(bookObj.files)
      ? (bookObj.files as ShelfdBookFile[])
      : [];

  let audioChapters: ShelfdAudioChapter[] = Array.isArray(raw.audio_chapters)
    ? (raw.audio_chapters as ShelfdAudioChapter[])
    : [];

  if (audioChapters.length === 0) {
    try {
      const chList = await fetchLoopbackJson<ShelfdAudioChapter[]>(
        ctx,
        `/api/v1/audiobooks/${encodeURIComponent(bookId)}/chapters`
      );
      if (Array.isArray(chList)) {
        audioChapters = chList;
      }
    } catch {
      void 0;
    }
  }

  const book: ShelfdBook = {
    id: String(bookObj.id ?? bookId),
    title: String(bookObj.title ?? "Untitled Audiobook"),
    description: typeof bookObj.description === "string" ? bookObj.description : undefined,
    book_type: typeof bookObj.book_type === "string" ? bookObj.book_type : undefined,
    file_path: typeof bookObj.file_path === "string" ? bookObj.file_path : undefined,
    file_size_bytes: typeof bookObj.file_size_bytes === "number" ? bookObj.file_size_bytes : undefined,
    duration_seconds: typeof bookObj.duration_seconds === "number" ? bookObj.duration_seconds : undefined,
    cover_path: typeof bookObj.cover_path === "string" ? bookObj.cover_path : undefined,
    files,
    audio_chapters: audioChapters,
  };

  return { book, audioChapters };
}

async function fetchBookCoverBytes(
  ctx: PluginContext<YotoPluginConfig>,
  bookId: string
): Promise<{ bytes: Uint8Array; contentType: string } | null> {
  try {
    const res = await ctx.fetchLoopback(`/api/v1/books/${encodeURIComponent(bookId)}/cover`);
    if (!res.ok) return null;
    const bytes = new Uint8Array(await res.arrayBuffer());
    if (bytes.length === 0) return null;
    const contentType = res.headers.get("content-type") ?? "image/jpeg";
    return { bytes, contentType };
  } catch {
    return null;
  }
}

function buildBookEntries(book: ShelfdBook, audioChapters: ShelfdAudioChapter[]): ActionEntry[] {
  const audioFiles = collectBookAudioFiles(book);
  if (audioFiles.length === 1 && audioChapters.length > 0) {
    return audioChapters.map((ch, idx) => ({
      id: String(idx),
      index: idx,
      title: ch.title?.trim() || `Chapter ${idx + 1}`,
    }));
  }
  if (audioFiles.length > 1) {
    return audioFiles.map((af, idx) => {
      const chTitle = audioChapters[idx]?.title?.trim();
      const fallback = af.file_path ? af.file_path.split("/").pop()?.replace(/\.[^.]+$/, "") : undefined;
      return {
        id: String(idx),
        index: idx,
        title: chTitle || fallback || `Track ${idx + 1}`,
      };
    });
  }
  return [
    {
      id: "0",
      index: 0,
      title: book.title || "Chapter 1",
    },
  ];
}

function buildAuthStatus(ctx: PluginContext<YotoPluginConfig>): AuthStatusResult & {
  linked: boolean;
  client_id: string;
  expires_at: string;
  methods: string[];
} {
  const accessToken = typeof ctx.config.access_token === "string" ? ctx.config.access_token.trim() : "";
  const clientId = typeof ctx.config.client_id === "string" ? ctx.config.client_id.trim() : "";
  const expiresAt = typeof ctx.config.token_expires_at === "string" ? ctx.config.token_expires_at.trim() : "";
  const linked = Boolean(accessToken);

  return {
    connected: linked,
    configured: Boolean(linked || clientId),
    linked,
    client_id: clientId,
    expires_at: expiresAt,
    methods: ["oauth_pkce", "token"],
    account_label: linked ? (clientId || "Yoto Account") : undefined,
  };
}

interface ParsedInvokeOptions {
  playlistMode: "create" | "append" | "replace";
  playlistTitle: string;
  cardId: string;
  includeCover: boolean;
  useCoverAsIcon: boolean;
  defaultIconUri: string;
  chapterIcons: Record<number, string>;
  bookIds?: string[];
}

function parseInvokeOptions(req: ActionInvokeRequest): ParsedInvokeOptions {
  const vals = (req.values ?? {}) as Record<string, unknown>;
  const rawMode = String(vals.playlist_mode ?? vals.mode ?? "create").toLowerCase().trim();
  const playlistMode: "create" | "append" | "replace" =
    rawMode === "append" || rawMode === "existing"
      ? "append"
      : rawMode === "replace"
        ? "replace"
        : "create";

  const playlistTitle = String(vals.playlist_title ?? vals.new_playlist_title ?? "").trim();
  const cardId = String(vals.card_id ?? "").trim();
  const includeCover = vals.include_cover !== false;
  const useCoverAsIcon = Boolean(vals.use_cover_as_icon);

  const rawDefaultIcon = String(vals.default_icon ?? vals.default_icon_id ?? "").trim();
  const defaultIconUri = normalizeYotoIconUri(rawDefaultIcon) || DEFAULT_BOOK_ICON_URI;

  const chapterIcons: Record<number, string> = {};
  const addIconOverride = (k: string, v: unknown) => {
    const idx = Number(k);
    if (!Number.isNaN(idx) && typeof v === "string" && v.trim()) {
      const norm = normalizeYotoIconUri(v.trim());
      if (norm) {
        chapterIcons[idx] = norm;
      }
    }
  };

  if (vals.chapter_icons && typeof vals.chapter_icons === "object") {
    for (const [k, v] of Object.entries(vals.chapter_icons as Record<string, unknown>)) {
      addIconOverride(k, v);
    }
  }
  if (req.overrides) {
    for (const [k, v] of Object.entries(req.overrides)) {
      addIconOverride(k, v);
    }
  }

  const bookIds = Array.isArray(req.target.book_ids)
    ? req.target.book_ids
    : Array.isArray(vals.book_ids)
      ? (vals.book_ids as unknown[]).map((id) => String(id))
      : undefined;

  return {
    playlistMode,
    playlistTitle,
    cardId,
    includeCover,
    useCoverAsIcon,
    defaultIconUri,
    chapterIcons,
    bookIds,
  };
}

async function uploadBookChaptersToYoto(params: {
  ctx: PluginContext<YotoPluginConfig>;
  apiBaseUrl: string;
  accessToken: string;
  book: ShelfdBook;
  audioFiles: ShelfdBookFile[];
  audioChapters: ShelfdAudioChapter[];
  options: ParsedInvokeOptions;
  entryOffset: number;
  prefixBookTitle: boolean;
  onFileProgress?: (fileIdx: number, stage: string, fraction: number) => void;
}): Promise<{
  chapters: YotoChapter[];
  totalDuration: number;
  totalFileSize: number;
  nextOffset: number;
}> {
  const { ctx, apiBaseUrl, accessToken, book, audioFiles, audioChapters, options, entryOffset, prefixBookTitle, onFileProgress } = params;
  let bookDefaultIconUri = options.defaultIconUri;

  if (options.useCoverAsIcon) {
    const coverData = await fetchBookCoverBytes(ctx, book.id);
    if (coverData) {
      try {
        const uploadedIcon = await uploadCustomIcon(apiBaseUrl, accessToken, coverData.bytes, `${book.id}-cover.png`);
        const norm = normalizeYotoIconUri(uploadedIcon.yotoUri);
        if (norm) {
          bookDefaultIconUri = norm;
        }
      } catch {
        void 0;
      }
    }
  }

  const resolveChapterIcon = (globalIdx: number): string => {
    const custom = options.chapterIcons[globalIdx];
    if (custom) {
      return custom;
    }
    return bookDefaultIconUri;
  };

  const formatSeriesChapterTitle = (rawTitle: string, singleEntryForBook: boolean): string => {
    const trimmed = rawTitle.trim();
    if (!prefixBookTitle) {
      return trimmed;
    }
    if (singleEntryForBook) {
      const lower = trimmed.toLowerCase();
      if (
        !trimmed ||
        lower === "chapter 1" ||
        lower === "track 1" ||
        lower === "01" ||
        lower === "1" ||
        lower === book.title.toLowerCase()
      ) {
        return book.title;
      }
    }
    if (trimmed.toLowerCase().startsWith(book.title.toLowerCase())) {
      return trimmed;
    }
    return `${book.title} - ${trimmed}`;
  };

  const chapters: YotoChapter[] = [];
  let totalDuration = 0;
  let totalFileSize = 0;
  let currentIdx = entryOffset;
  const pollIntervalMs = typeof ctx.config.poll_interval_ms === "number" && ctx.config.poll_interval_ms > 0 ? ctx.config.poll_interval_ms : 150;

  for (let fileIdx = 0; fileIdx < audioFiles.length; fileIdx++) {
    const af = audioFiles[fileIdx];
    onFileProgress?.(fileIdx, "uploading", 0.05);

    const streamPath = af.id
      ? `/api/v1/books/${encodeURIComponent(book.id)}/files/${encodeURIComponent(af.id)}/stream`
      : `/api/v1/books/${encodeURIComponent(book.id)}/audio`;

    const { stream, contentLength, contentType } = await ctx.openLoopbackStream(streamPath);
    const upUrl = await requestAudioUploadUrl(apiBaseUrl, accessToken);
    onFileProgress?.(fileIdx, "uploading", 0.15);

    const fileSize = contentLength ?? af.file_size_bytes ?? book.file_size_bytes ?? 0;
    const mime = contentType ?? af.mime_type ?? "audio/mpeg";
    await uploadAudioStream(upUrl.uploadUrl, stream, mime, fileSize > 0 ? fileSize : undefined);

    onFileProgress?.(fileIdx, "transcoding", 0.7);
    const transcoded = await waitForTranscoding(apiBaseUrl, accessToken, upUrl.uploadId, 60, pollIntervalMs);
    onFileProgress?.(fileIdx, "transcoding", 0.95);

    const trackDuration =
      (transcoded.transcodedInfo?.duration && transcoded.transcodedInfo.duration > 0
        ? transcoded.transcodedInfo.duration
        : af.duration_seconds ?? book.duration_seconds) ?? 0;
    const trackSize =
      (transcoded.transcodedInfo?.fileSize && transcoded.transcodedInfo.fileSize > 0
        ? transcoded.transcodedInfo.fileSize
        : fileSize) ?? 0;
    const extMatch = (af.file_path ?? book.file_path ?? "").match(/\.([a-zA-Z0-9]+)$/);
    const trackFormat = transcoded.transcodedInfo?.format || (extMatch ? extMatch[1].toLowerCase() : "aac");
    const trackChannels = transcoded.transcodedInfo?.channels;

    totalDuration += trackDuration;
    totalFileSize += trackSize;

    if (audioFiles.length === 1 && audioChapters.length > 0) {
      for (let chIdx = 0; chIdx < audioChapters.length; chIdx++) {
        const ac = audioChapters[chIdx];
        const num = currentIdx + 1;
        const keyStr = String(num).padStart(2, "0");
        const labelStr = String(num);
        let chTitle = (ac.title ?? "").trim();
        if (audioChapters.length === 1) {
          const lower = chTitle.toLowerCase();
          if (!chTitle || lower === "chapter 1" || lower === "track 1" || lower === "01" || lower === "1") {
            chTitle = book.title;
          }
        } else if (!chTitle) {
          chTitle = `Chapter ${chIdx + 1}`;
        }
        chTitle = formatSeriesChapterTitle(chTitle, audioChapters.length === 1);
        const iconUri = resolveChapterIcon(currentIdx);
        const dur = ac.duration_sec && ac.duration_sec > 0 ? ac.duration_sec : trackDuration;

        chapters.push({
          key: keyStr,
          title: chTitle,
          overlayLabel: labelStr,
          duration: dur,
          fileSize: trackSize,
          display: { icon16x16: iconUri },
          tracks: [
            {
              key: keyStr,
              title: chTitle,
              trackUrl: `yoto:#${transcoded.transcodedSha256}`,
              duration: dur,
              fileSize: trackSize,
              channels: trackChannels,
              format: trackFormat,
              type: "audio",
              overlayLabel: labelStr,
              display: { icon16x16: iconUri },
            },
          ],
        });
        currentIdx += 1;
      }
    } else {
      const num = currentIdx + 1;
      const keyStr = String(num).padStart(2, "0");
      const labelStr = String(num);
      let chTitle = transcoded.transcodedInfo?.metadata?.title?.trim() ?? "";
      if (!chTitle && fileIdx < audioChapters.length && audioChapters[fileIdx].title) {
        chTitle = audioChapters[fileIdx].title!.trim();
      }
      if (!chTitle) {
        if (audioFiles.length === 1) {
          chTitle = book.title;
        } else {
          const base = (af.file_path ?? "").split("/").pop() ?? "";
          chTitle = base.replace(/\.[^.]+$/, "") || `Track ${fileIdx + 1}`;
        }
      }
      chTitle = formatSeriesChapterTitle(chTitle, audioFiles.length === 1);
      const iconUri = resolveChapterIcon(currentIdx);

      chapters.push({
        key: keyStr,
        title: chTitle,
        overlayLabel: labelStr,
        duration: trackDuration,
        fileSize: trackSize,
        display: { icon16x16: iconUri },
        tracks: [
          {
            key: keyStr,
            title: chTitle,
            trackUrl: `yoto:#${transcoded.transcodedSha256}`,
            duration: trackDuration,
            fileSize: trackSize,
            channels: trackChannels,
            format: trackFormat,
            type: "audio",
            overlayLabel: labelStr,
            display: { icon16x16: iconUri },
          },
        ],
      });
      currentIdx += 1;
    }

    onFileProgress?.(fileIdx, "transcoded", 1.0);
  }

  return {
    chapters,
    totalDuration,
    totalFileSize,
    nextOffset: currentIdx,
  };
}

async function saveChaptersToYotoCard(params: {
  ctx: PluginContext<YotoPluginConfig>;
  apiBaseUrl: string;
  accessToken: string;
  options: ParsedInvokeOptions;
  itemLabel: string;
  defaultPlaylistTitle: string;
  defaultDescription?: string;
  coverBookId?: string;
  newChapters: YotoChapter[];
  totalDuration: number;
  totalFileSize: number;
}): Promise<ActionResult> {
  const {
    ctx,
    apiBaseUrl,
    accessToken,
    options,
    itemLabel,
    defaultPlaylistTitle,
    defaultDescription,
    coverBookId,
    newChapters,
    totalDuration,
    totalFileSize,
  } = params;

  let uploadedCoverUrl = "";
  if (options.includeCover && coverBookId) {
    const coverData = await fetchBookCoverBytes(ctx, coverBookId);
    if (coverData) {
      try {
        uploadedCoverUrl = await uploadCoverImage(apiBaseUrl, accessToken, coverData.bytes, coverData.contentType);
      } catch {
        void 0;
      }
    }
  }

  let cumulativeDuration = totalDuration;
  let cumulativeFileSize = totalFileSize;
  const contentPayload: Partial<YotoCard> = {
    content: { chapters: [] },
    metadata: {},
  };

  if (options.playlistMode === "append") {
    if (!options.cardId) {
      throw new Error("card_id is required when appending to an existing playlist");
    }
    const existingCard = await getCard(apiBaseUrl, accessToken, options.cardId);
    contentPayload.cardId = existingCard.cardId;
    contentPayload.title = existingCard.title;
    contentPayload.userId = existingCard.userId;
    contentPayload.slug = existingCard.slug;
    contentPayload.createdAt = existingCard.createdAt;
    contentPayload.updatedAt = existingCard.updatedAt;
    contentPayload.metadata = { ...existingCard.metadata };
    contentPayload.content = { ...existingCard.content };

    const existingChapters = existingCard.content.chapters ?? [];
    if (existingCard.metadata.media) {
      cumulativeDuration += existingCard.metadata.media.duration ?? 0;
      cumulativeFileSize += existingCard.metadata.media.fileSize ?? 0;
    } else {
      for (const ch of existingChapters) {
        if (ch.duration && ch.duration > 0) {
          cumulativeDuration += ch.duration;
        } else {
          for (const tr of ch.tracks ?? []) {
            cumulativeDuration += tr.duration ?? 0;
          }
        }
        if (ch.fileSize && ch.fileSize > 0) {
          cumulativeFileSize += ch.fileSize;
        } else {
          for (const tr of ch.tracks ?? []) {
            cumulativeFileSize += tr.fileSize ?? 0;
          }
        }
      }
    }

    const offset = existingChapters.length;
    const reindexedNew = newChapters.map((ch, idx) => {
      const num = offset + idx + 1;
      const keyStr = String(num).padStart(2, "0");
      const labelStr = String(num);
      return {
        ...ch,
        key: keyStr,
        overlayLabel: labelStr,
        tracks: (ch.tracks ?? []).map((tr) => ({
          ...tr,
          key: keyStr,
          overlayLabel: labelStr,
        })),
      };
    });
    contentPayload.content.chapters = [...existingChapters, ...reindexedNew];
  } else if (options.playlistMode === "replace") {
    if (!options.cardId) {
      throw new Error("card_id is required when replacing an existing playlist");
    }
    try {
      const existingCard = await getCard(apiBaseUrl, accessToken, options.cardId);
      contentPayload.cardId = existingCard.cardId;
      contentPayload.title = existingCard.title;
      contentPayload.userId = existingCard.userId;
      contentPayload.slug = existingCard.slug;
      contentPayload.createdAt = existingCard.createdAt;
      contentPayload.metadata = { ...existingCard.metadata };
      contentPayload.content = { ...existingCard.content, chapters: newChapters };
    } catch {
      contentPayload.cardId = options.cardId;
      contentPayload.title = defaultPlaylistTitle;
      contentPayload.content = { chapters: newChapters };
    }
    if (options.playlistTitle) {
      contentPayload.title = options.playlistTitle;
    }
  } else {
    contentPayload.title = options.playlistTitle || defaultPlaylistTitle;
    contentPayload.content = { chapters: newChapters };
    if (defaultDescription) {
      contentPayload.metadata = { description: defaultDescription };
    }
  }

  if (!contentPayload.metadata) {
    contentPayload.metadata = {};
  }
  if (uploadedCoverUrl) {
    contentPayload.metadata.cover = { imageL: uploadedCoverUrl };
  }
  contentPayload.metadata.media = {
    duration: cumulativeDuration,
    fileSize: cumulativeFileSize,
    readableFileSize: Math.round((cumulativeFileSize / 1024 / 1024) * 10) / 10,
  };

  const savedCard = await createOrUpdateContent(apiBaseUrl, accessToken, contentPayload);
  const actionVerb =
    options.playlistMode === "append"
      ? "Appended"
      : options.playlistMode === "replace"
        ? "Replaced"
        : "Sent";

  const finalChaptersCount = contentPayload.content?.chapters?.length ?? newChapters.length;

  return {
    status: "completed",
    message: `${actionVerb} "${itemLabel}" on Yoto playlist "${savedCard.title}"`,
    data: {
      card_id: savedCard.cardId,
      title: savedCard.title,
      chapters_uploaded: newChapters.length,
      chapter_count: finalChaptersCount,
      action: options.playlistMode,
    },
  };
}

async function executeSendToYoto(req: ActionInvokeRequest, ctx: PluginContext<YotoPluginConfig>): Promise<ActionResult> {
  const accessToken = await ensureValidAccessToken(ctx);
  const apiBaseUrl = resolveApiBaseUrl(ctx.config);
  const options = parseInvokeOptions(req);
  const targetType = req.target.type === "series" ? "series" : "book";
  const targetId = req.target.id.trim();
  if (!targetId) {
    throw new Error("Target ID is required");
  }

  if (targetType === "series") {
    const seriesData = await fetchLoopbackJson<ShelfdSeriesResponse>(
      ctx,
      `/api/v1/series/${encodeURIComponent(targetId)}`
    );
    const seriesName = seriesData.series?.name || req.target.title || "Audiobook Series";
    const seriesDesc = seriesData.series?.description;

    let candidates: Array<{ book: ShelfdBook; files: ShelfdBookFile[]; chapters: ShelfdAudioChapter[] }> = [];
    for (const b of seriesData.books ?? []) {
      const detailed = await fetchBookAndChapters(ctx, b.id);
      const files = collectBookAudioFiles(detailed.book);
      if (files.length === 0) continue;
      candidates.push({
        book: detailed.book,
        files,
        chapters: detailed.audioChapters,
      });
    }

    if (options.bookIds && options.bookIds.length > 0) {
      const byId = new Map(candidates.map((c) => [c.book.id, c]));
      candidates = options.bookIds
        .map((id) => byId.get(id.trim()))
        .filter((c): c is { book: ShelfdBook; files: ShelfdBookFile[]; chapters: ShelfdAudioChapter[] } => Boolean(c));
    }

    if (candidates.length === 0) {
      throw new Error("Audio files are not available for any books in this series");
    }

    const totalBooks = candidates.length;
    const allChapters: YotoChapter[] = [];
    let totalDuration = 0;
    let totalFileSize = 0;
    let coverBookId: string | undefined;
    let entryOffset = 0;

    for (let bookIdx = 0; bookIdx < totalBooks; bookIdx++) {
      const item = candidates[bookIdx];
      if (!coverBookId) {
        coverBookId = item.book.id;
      }
      const bookFileCount = item.files.length;
      const uploaded = await uploadBookChaptersToYoto({
        ctx,
        apiBaseUrl,
        accessToken,
        book: item.book,
        audioFiles: item.files,
        audioChapters: item.chapters,
        options,
        entryOffset,
        prefixBookTitle: true,
        onFileProgress: (fileIdx, stage, fraction) => {
          const bookProgress = (fileIdx + fraction) / Math.max(1, bookFileCount);
          const percent = Math.round(((bookIdx + bookProgress) / totalBooks) * 92);
          ctx.reportProgress({
            jobId: req.job_id,
            current: bookIdx + 1,
            total: totalBooks,
            percent,
            title: item.book.title,
            status: stage,
            message: `${stage === "transcoding" ? "Transcoding" : "Uploading"} ${item.book.title} (${bookIdx + 1}/${totalBooks})`,
          });
        },
      });

      allChapters.push(...uploaded.chapters);
      totalDuration += uploaded.totalDuration;
      totalFileSize += uploaded.totalFileSize;
      entryOffset = uploaded.nextOffset;
    }

    ctx.reportProgress({
      jobId: req.job_id,
      current: totalBooks,
      total: totalBooks,
      percent: 96,
      title: seriesName,
      status: "saving",
      message: `Saving Yoto MYO card "${options.playlistTitle || seriesName}"`,
    });

    return await saveChaptersToYotoCard({
      ctx,
      apiBaseUrl,
      accessToken,
      options,
      itemLabel: seriesName,
      defaultPlaylistTitle: seriesName,
      defaultDescription: seriesDesc,
      coverBookId,
      newChapters: allChapters,
      totalDuration,
      totalFileSize,
    });
  }

  const { book, audioChapters } = await fetchBookAndChapters(ctx, targetId);
  const audioFiles = collectBookAudioFiles(book);
  if (audioFiles.length === 0) {
    throw new Error("Audio files are not available for this book");
  }

  const totalFiles = audioFiles.length;
  const uploaded = await uploadBookChaptersToYoto({
    ctx,
    apiBaseUrl,
    accessToken,
    book,
    audioFiles,
    audioChapters,
    options,
    entryOffset: 0,
    prefixBookTitle: false,
    onFileProgress: (fileIdx, stage, fraction) => {
      const percent = Math.round(((fileIdx + fraction) / Math.max(1, totalFiles)) * 92);
      ctx.reportProgress({
        jobId: req.job_id,
        current: fileIdx + 1,
        total: totalFiles,
        percent,
        title: book.title,
        status: stage,
        message: `${stage === "transcoding" ? "Transcoding" : "Uploading"} "${book.title}" (${fileIdx + 1}/${totalFiles})`,
      });
    },
  });

  ctx.reportProgress({
    jobId: req.job_id,
    current: totalFiles,
    total: totalFiles,
    percent: 96,
    title: book.title,
    status: "saving",
    message: `Saving Yoto MYO card "${options.playlistTitle || book.title}"`,
  });

  return await saveChaptersToYotoCard({
    ctx,
    apiBaseUrl,
    accessToken,
    options,
    itemLabel: book.title,
    defaultPlaylistTitle: book.title,
    defaultDescription: book.description,
    coverBookId: book.id,
    newChapters: uploaded.chapters,
    totalDuration: uploaded.totalDuration,
    totalFileSize: uploaded.totalFileSize,
  });
}

definePlugin<YotoPluginConfig>({
  name: "yoto",
  version: "0.1.0",

  auth: {
    spec: {
      methods: ["oauth_pkce", "token"],
      title: "Yoto Account",
      description: "Link your Yoto account via OAuth 2.0 PKCE or paste a Yoto access token.",
      callback_param: "code",
      fields: [
        {
          key: "client_id",
          title: "Yoto Client ID",
          placeholder: "Optional custom OAuth Client ID",
        },
        {
          key: "access_token",
          title: "Access Token",
          secret: true,
        },
        {
          key: "refresh_token",
          title: "Refresh Token (Optional)",
          secret: true,
        },
      ],
    },

    status(ctx) {
      return buildAuthStatus(ctx);
    },

    async begin(req: AuthBeginRequest, ctx) {
      const rawReq = req as AuthBeginRequest & { client_id?: string };
      const incomingClientId = typeof rawReq.client_id === "string" ? rawReq.client_id.trim() : "";
      if (incomingClientId && ctx.userId) {
        ctx.config.client_id = incomingClientId;
        await ctx.patchConfig({ client_id: incomingClientId });
      }

      const clientId =
        incomingClientId ||
        (typeof ctx.config.client_id === "string" && ctx.config.client_id.trim() ? ctx.config.client_id.trim() : "shelfd-yoto");
      const redirectUri =
        (typeof req.redirect_uri === "string" && req.redirect_uri.trim()
          ? req.redirect_uri.trim()
          : typeof ctx.config.redirect_uri === "string"
            ? ctx.config.redirect_uri.trim()
            : "") || "";

      const authBaseUrl = resolveAuthBaseUrl(ctx.config);
      const apiBaseUrl = resolveApiBaseUrl(ctx.config);
      const authorizeUrl = `${authBaseUrl}/authorize?audience=${encodeURIComponent(apiBaseUrl)}&response_type=code&client_id=${encodeURIComponent(clientId)}&redirect_uri=${encodeURIComponent(redirectUri)}&scope=offline_access%20openid%20profile&code_challenge=${encodeURIComponent(req.code_challenge || "")}&code_challenge_method=S256&state=${encodeURIComponent(req.state || "")}`;

      return {
        method: req.method || "oauth_pkce",
        authorize_url: authorizeUrl,
        callback_param: "code",
        client_id: clientId,
        state: req.state || "",
      };
    },

    async complete(req: AuthCompleteRequest, ctx) {
      const rawReq = req as AuthCompleteRequest & {
        client_id?: string;
        access_token?: string;
        refresh_token?: string;
        credentials?: Record<string, unknown>;
      };
      const values = req.values ?? {};
      const creds = rawReq.credentials ?? {};

      const directAccessToken = String(
        rawReq.access_token ?? values.access_token ?? creds.access_token ?? ""
      ).trim();
      const directRefreshToken = String(
        rawReq.refresh_token ?? values.refresh_token ?? creds.refresh_token ?? ""
      ).trim();
      const reqClientId = String(
        rawReq.client_id ?? values.client_id ?? creds.client_id ?? ctx.config.client_id ?? ""
      ).trim();

      if (req.method === "token" || directAccessToken) {
        if (!directAccessToken) {
          throw new Error("Access token is required");
        }
        const clientId = reqClientId || "manual-token";
        const expiresAt = parseJwtExpiry(directAccessToken);
        ctx.config.access_token = directAccessToken;
        ctx.config.refresh_token = directRefreshToken;
        ctx.config.token_expires_at = expiresAt;
        ctx.config.client_id = clientId;
        await ctx.patchConfig({
          access_token: directAccessToken,
          refresh_token: directRefreshToken,
          token_expires_at: expiresAt,
          client_id: clientId,
        });
        return buildAuthStatus(ctx);
      }

      const code = (req.code ?? values.code ?? "").trim();
      if (!code) {
        throw new Error("Authorization code or access_token is required");
      }
      const clientId = reqClientId || "shelfd-yoto";
      const redirectUri =
        (req.redirect_uri ?? values.redirect_uri ?? (typeof ctx.config.redirect_uri === "string" ? ctx.config.redirect_uri : ""))
          .trim();
      const clientSecret = typeof ctx.config.client_secret === "string" ? ctx.config.client_secret.trim() : undefined;

      const tok = await exchangeCode({
        authBaseUrl: resolveAuthBaseUrl(ctx.config),
        clientId,
        clientSecret,
        code,
        codeVerifier: req.code_verifier?.trim(),
        redirectUri,
      });

      ctx.config.access_token = tok.access_token;
      ctx.config.refresh_token = tok.refresh_token;
      ctx.config.token_expires_at = tok.expires_at;
      ctx.config.client_id = clientId;

      await ctx.patchConfig({
        access_token: tok.access_token,
        refresh_token: tok.refresh_token,
        token_expires_at: tok.expires_at,
        client_id: clientId,
      });

      return buildAuthStatus(ctx);
    },

    async disconnect(ctx) {
      ctx.config.access_token = "";
      ctx.config.refresh_token = "";
      ctx.config.token_expires_at = "";
      if (ctx.userId) {
        await ctx.patchConfig({
          access_token: "",
          refresh_token: "",
          token_expires_at: "",
        });
      }
      return buildAuthStatus(ctx);
    },
  },

  actions: {
    send_to_yoto: {
      title: "Send to Yoto",
      description: "Upload audiobook tracks to a new or existing Yoto Make Your Own (MYO) playlist card.",
      icon: "speaker",
      targets: ["book", "series"],
      mediaTypes: ["audiobook"],
      mode: "form",
      execution: "job",

      async prepare(req, ctx): Promise<ActionFormSpec> {
        let targetTitle = req.target.title?.trim() ?? "";
        let entries: ActionEntry[] = [];

        if (req.target.type === "series" && req.target.id) {
          try {
            const seriesData = await fetchLoopbackJson<ShelfdSeriesResponse>(
              ctx,
              `/api/v1/series/${encodeURIComponent(req.target.id)}`
            );
            if (!targetTitle && seriesData.series?.name) {
              targetTitle = seriesData.series.name;
            }
            const audioBooks = (seriesData.books ?? []).filter(
              (b) => b.book_type === "audiobook" || isAudioExtension(b.file_path) || (b.files ?? []).some((f) => f.file_type === "audiobook" || f.file_type === "audio" || isAudioExtension(f.file_path))
            );
            entries = audioBooks.map((b, idx) => ({
              id: b.id,
              index: idx,
              title: b.title,
            }));
          } catch {
            void 0;
          }
        } else if (req.target.id) {
          try {
            const { book, audioChapters } = await fetchBookAndChapters(ctx, req.target.id);
            if (!targetTitle && book.title) {
              targetTitle = book.title;
            }
            entries = buildBookEntries(book, audioChapters);
          } catch {
            void 0;
          }
        }

        if (!targetTitle) {
          targetTitle = "Audiobook";
        }
        if (entries.length === 0) {
          entries = [{ id: "0", index: 0, title: targetTitle }];
        }

        const isLinked = Boolean(typeof ctx.config.access_token === "string" && ctx.config.access_token.trim());

        return {
          title: `Send "${targetTitle}" to Yoto`,
          description: "Upload audiobook tracks to a new or existing Yoto Make Your Own (MYO) playlist card.",
          submit_label: "Send to Yoto",
          auth_required: !isLinked,
          auth_message: !isLinked
            ? "Link your Yoto account in plugin settings before sending audio to Yoto."
            : undefined,
          entries,
          fields: [
            {
              key: "playlist_mode",
              kind: "segmented",
              title: "Destination Card",
              default_value: "create",
              options: [
                { value: "create", label: "Create New Card" },
                { value: "append", label: "Add to Existing" },
                { value: "replace", label: "Replace Existing" },
              ],
            },
            {
              key: "playlist_title",
              kind: "text",
              title: "New Card Title",
              default_value: targetTitle,
              visible_when: { field: "playlist_mode", equals: "create" },
            },
            {
              key: "card_id",
              kind: "remote_select",
              title: "Target MYO Playlist",
              options_source: "playlists",
              searchable: true,
              visible_when: { field: "playlist_mode", in: ["append", "replace"] },
            },
            {
              key: "include_cover",
              kind: "boolean",
              title: "Include Book Cover Art",
              default_value: true,
            },
            {
              key: "use_cover_as_icon",
              kind: "boolean",
              title: "Auto-convert cover art to 16x16 icon",
              default_value: false,
            },
            {
              key: "default_icon",
              kind: "asset_picker",
              title: "Default 16x16 Chapter Icon",
              default_value: DEFAULT_BOOK_ICON_URI,
              options_source: "icons",
              asset_shape: "square_pixel",
              allow_upload: true,
              tabs: [
                { id: "public", label: "Public Icons", searchable: true },
                { id: "custom", label: "My Icons", searchable: true },
              ],
              per_entry_override: true,
              override_key: "chapter_icons",
              override_label: "Customize Per-Chapter Icons",
            },
          ],
        };
      },

      async options(req, ctx) {
        const accessToken = await ensureValidAccessToken(ctx);
        const apiBaseUrl = resolveApiBaseUrl(ctx.config);
        const q = (req.query ?? "").trim().toLowerCase();

        if (req.source === "playlists") {
          const playlists = await getMyPlaylists(apiBaseUrl, accessToken);
          const filtered = q
            ? playlists.filter((pl) => pl.title.toLowerCase().includes(q) || pl.cardId.toLowerCase().includes(q))
            : playlists;
          const items: OptionItem[] = filtered.map((pl) => ({
            value: pl.cardId,
            label: pl.title,
            subtitle: `${pl.chapterCount} chapters`,
            image_url: pl.coverImageUrl || undefined,
            count: pl.chapterCount,
          }));
          return { items };
        }

        if (req.source === "icons") {
          const icons =
            req.tab === "custom"
              ? await getUserIcons(apiBaseUrl, accessToken)
              : await getPublicIcons(apiBaseUrl, accessToken);

          const filtered = q
            ? icons.filter((ic) => {
                const titleMatch = (ic.title ?? "").toLowerCase().includes(q);
                const idMatch = ic.mediaId.toLowerCase().includes(q);
                const tagMatch = (ic.publicTags ?? []).some((t) => t.toLowerCase().includes(q));
                return titleMatch || idMatch || tagMatch;
              })
            : icons;

          const items: OptionItem[] = filtered.map((icon) => ({
            value: icon.yotoUri,
            label: icon.title || icon.mediaId,
            image_url: icon.url,
            icon_uri: icon.yotoUri,
          }));
          return { items };
        }

        return { items: [] };
      },

      async uploadAsset(req, ctx) {
        const accessToken = await ensureValidAccessToken(ctx);
        const apiBaseUrl = resolveApiBaseUrl(ctx.config);
        const bytes = new Uint8Array(Buffer.from(req.data_base64, "base64"));
        const icon = await uploadCustomIcon(apiBaseUrl, accessToken, bytes, req.filename || "icon.png");
        return {
          item: {
            value: icon.yotoUri,
            label: icon.title || req.filename || icon.mediaId,
            image_url: icon.url,
            icon_uri: icon.yotoUri,
          },
        };
      },

      async invoke(req, ctx) {
        return await executeSendToYoto(req, ctx);
      },
    },
  },

  tools: {
    list_playlists: {
      description: "List the user's Yoto Make Your Own (MYO) playlist cards.",
      inputSchema: {
        type: "object",
        properties: {
          query: {
            type: "string",
            description: "Optional search query to filter playlist cards by title.",
          },
        },
      },
      async execute(args, ctx) {
        const accessToken = await ensureValidAccessToken(ctx);
        const apiBaseUrl = resolveApiBaseUrl(ctx.config);
        const playlists = await getMyPlaylists(apiBaseUrl, accessToken);
        const q = typeof args.query === "string" ? args.query.trim().toLowerCase() : "";
        const filtered = q ? playlists.filter((p) => p.title.toLowerCase().includes(q)) : playlists;

        if (filtered.length === 0) {
          return "No Yoto Make Your Own (MYO) playlists found.";
        }
        const lines = filtered.map(
          (pl) => `- [card:${pl.cardId}] "${pl.title}" (${pl.chapterCount} chapter${pl.chapterCount === 1 ? "" : "s"})`
        );
        return truncateForMCP(`Found ${filtered.length} Yoto MYO playlist(s):\n${lines.join("\n")}`, 3800);
      },
    },

    send_to_yoto: {
      description: "Send a Shelfd audiobook or series to a new or existing Yoto Make Your Own (MYO) card.",
      inputSchema: {
        type: "object",
        required: ["target_id"],
        properties: {
          target_type: {
            type: "string",
            enum: ["book", "series"],
          },
          target_id: {
            type: "string",
          },
          playlist_mode: {
            type: "string",
            enum: ["create", "append", "replace"],
          },
          card_id: {
            type: "string",
          },
          playlist_title: {
            type: "string",
          },
        },
      },
      async execute(args, ctx) {
        const targetType = args.target_type === "series" ? "series" : "book";
        const targetId = String(args.target_id ?? "").trim();
        if (!targetId) {
          throw new Error("target_id is required");
        }
        const res = await executeSendToYoto(
          {
            action: "send_to_yoto",
            target: { type: targetType, id: targetId },
            values: {
              playlist_mode: args.playlist_mode ?? "create",
              card_id: args.card_id,
              playlist_title: args.playlist_title,
            },
          },
          ctx
        );
        return truncateForMCP(res.message || "Sent to Yoto.", 3800);
      },
    },
  },
});
