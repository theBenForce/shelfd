export const PROTOCOL_VERSION = "2026-11-01";

export type PluginCapability = "tools" | "commands" | "events" | "catalog";
export type CatalogMode = "browse" | "search";
export type PluginAPIScope = "uploads:stage" | "uploads:write" | "books:read";

export interface EntrypointSpec {
  command: string;
  args?: string[];
}

export interface PermissionsSpec {
  network_domains?: string[];
  library_read?: boolean;
  library_write?: boolean;
}

export interface ToolDef {
  name: string;
  description: string;
  input_schema: Record<string, unknown>;
}

export interface CommandDef {
  name: string;
  title?: string;
  description?: string;
  input_schema?: Record<string, unknown>;
}

export interface CatalogDescriptor {
  modes?: CatalogMode[];
  supported_formats?: string[];
  search_hint?: string;
  requires_auth?: boolean;
}

export interface PluginManifest {
  name: string;
  version: string;
  display_name?: string;
  description?: string;
  homepage?: string;
  entrypoint?: EntrypointSpec;
  daemon?: boolean;
  capabilities?: PluginCapability[];
  catalog_modes?: CatalogMode[];
  supported_formats?: string[];
  api_scopes?: PluginAPIScope[];
  permissions?: PermissionsSpec;
  config_schema?: Record<string, unknown>;
  tools?: ToolDef[];
  commands?: CommandDef[];
  catalog?: CatalogDescriptor;
}

export interface CatalogFormat {
  format: string;
  label?: string;
  size_bytes?: number;
}

export interface CatalogGroup {
  id: string;
  label: string;
  count?: number;
}

export interface CatalogItem {
  external_id: string;
  title: string;
  authors?: string[];
  summary?: string;
  cover_url?: string;
  group?: string;
  publisher?: string;
  language?: string;
  genres?: string[];
  formats?: CatalogFormat[];
  status?: "available" | "staged" | "in_library";
  upload_job_id?: string;
  book_id?: string;
}

export interface CatalogPage {
  items: CatalogItem[];
  groups?: CatalogGroup[];
  next_cursor?: string;
  total_count?: number;
}

export interface CatalogBrowseRequest {
  group?: string;
  cursor?: string;
  filter?: string;
}

export interface CatalogSearchRequest {
  query: string;
  cursor?: string;
  filters?: Record<string, unknown>;
}

export interface CatalogStageItem {
  external_id: string;
  preferred_format?: string;
}

export interface CatalogStageRequest {
  items: CatalogStageItem[];
}

export interface CatalogStagedEntry {
  external_id: string;
  title?: string;
  upload_job_id?: string;
  status: "staged" | "queued" | "failed";
  error?: string;
}

export interface CatalogStageResult {
  batch_id?: string;
  async?: boolean;
  results?: CatalogStagedEntry[];
  message?: string;
}

export interface MetadataDefaults {
  title?: string;
  authors?: string[];
  series?: string;
  sequence_number?: number;
  description?: string;
  publisher?: string;
  language?: string;
  genres?: string[];
  narrator?: string;
}

export interface StageBookInput {
  data: Uint8Array | ArrayBuffer | Blob;
  filename: string;
  mimeType?: string;
  externalId?: string;
  cover?: {
    data: Uint8Array | ArrayBuffer | Blob;
    filename?: string;
    mimeType?: string;
  };
  metadataDefaults?: MetadataDefaults;
}

export interface StageBookResponse {
  job_id: string;
  status: string;
  filename: string;
  has_cover: boolean;
  metadata: {
    title: string;
    authors: string[];
    series?: string;
    sequence_number?: number;
    description?: string;
    publisher?: string;
    language?: string;
    genres?: string[];
    is_duplicate?: boolean;
    warnings?: string[];
  };
  warnings?: string[];
}

export interface ProgressNotification {
  batchId?: string;
  current: number;
  total: number;
  externalId?: string;
  title?: string;
  status?: string;
  message?: string;
}

export interface PluginContext<TConfig extends Record<string, unknown> = Record<string, unknown>> {
  readonly userId: string;
  readonly apiBase: string;
  readonly invocationToken: string;
  readonly dataDir: string;
  readonly config: TConfig;
  stageBook(input: StageBookInput): Promise<StageBookResponse>;
  patchConfig(values: Partial<TConfig>): Promise<void>;
  emitEvent(event: string, data?: Record<string, unknown>): void;
  reportProgress(progress: ProgressNotification): void;
  log(level: "debug" | "info" | "warn" | "error", message: string): void;
}

export interface ToolHandlerSpec<TConfig extends Record<string, unknown> = Record<string, unknown>> {
  description: string;
  inputSchema: Record<string, unknown>;
  execute: (args: Record<string, unknown>, ctx: PluginContext<TConfig>) => Promise<string | { content: string; isError?: boolean }> | string | { content: string; isError?: boolean };
}

export interface CommandHandlerSpec<TConfig extends Record<string, unknown> = Record<string, unknown>> {
  title?: string;
  description?: string;
  inputSchema?: Record<string, unknown>;
  execute: (args: Record<string, unknown>, ctx: PluginContext<TConfig>) => Promise<unknown> | unknown;
}

export interface CatalogHandlers<TConfig extends Record<string, unknown> = Record<string, unknown>> {
  descriptor?: CatalogDescriptor;
  browse?: (req: CatalogBrowseRequest, ctx: PluginContext<TConfig>) => Promise<CatalogPage>;
  search?: (req: CatalogSearchRequest, ctx: PluginContext<TConfig>) => Promise<CatalogPage>;
  stage: (req: CatalogStageRequest, ctx: PluginContext<TConfig>) => Promise<CatalogStageResult>;
}

export interface PluginDefinition<TConfig extends Record<string, unknown> = Record<string, unknown>> {
  name: string;
  version: string;
  tools?: Record<string, ToolHandlerSpec<TConfig>>;
  commands?: Record<string, CommandHandlerSpec<TConfig>>;
  catalog?: CatalogHandlers<TConfig>;
  onInitialize?: (params: { protocolVersion: string; shelfdVersion: string; apiBase: string; dataDir: string }) => Promise<void> | void;
  onConfigUpdate?: (userId: string, config: TConfig) => Promise<void> | void;
  onShutdown?: () => Promise<void> | void;
}

export interface PoliteFetcherOptions {
  minIntervalMs?: number;
  maxRetries?: number;
  userAgent?: string;
  fetchImpl?: typeof fetch;
}
