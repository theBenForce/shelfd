export const PROTOCOL_VERSION = "2026-11-01";

export type PluginCapability = "tools" | "commands" | "events" | "catalog" | "actions";
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

export interface AuthFieldSpec {
  key: string;
  title: string;
  description?: string;
  secret?: boolean;
  required?: boolean;
  placeholder?: string;
}

export interface AuthSpec {
  methods: string[];
  title?: string;
  description?: string;
  callback_param?: string;
  fields?: AuthFieldSpec[];
}

export interface AuthStatusResult {
  connected: boolean;
  configured: boolean;
  method?: string;
  account_label?: string;
  auth_url?: string;
  callback_param?: string;
  message?: string;
  details?: Record<string, unknown>;
}

export interface AuthBeginRequest {
  method?: string;
  redirect_uri?: string;
  state?: string;
  code_challenge?: string;
  code_challenge_method?: string;
}

export interface AuthBeginResult {
  method: string;
  authorize_url?: string;
  callback_param?: string;
  instructions?: string;
  fields?: AuthFieldSpec[];
}

export interface AuthCompleteRequest {
  method?: string;
  code?: string;
  redirect_uri?: string;
  code_verifier?: string;
  state?: string;
  access_token?: string;
  refresh_token?: string;
  client_id?: string;
  credentials?: Record<string, string>;
  values?: Record<string, string>;
}

export interface ActionDef {
  name: string;
  title: string;
  description?: string;
  icon?: string;
  targets: Array<"book" | "series">;
  media_types?: Array<"ebook" | "audiobook">;
  mode?: "immediate" | "form";
  execution?: "sync" | "job";
}

export interface ActionTarget {
  type: "book" | "series";
  id: string;
  title?: string;
  book_ids?: string[];
}

export interface ActionEntry {
  id: string;
  index: number;
  title: string;
  subtitle?: string;
  icon_uri?: string;
  image_url?: string;
}

export interface FieldVisibilityRule {
  field: string;
  equals?: string;
  in?: string[];
}

export interface OptionItem {
  value: string;
  label: string;
  subtitle?: string;
  description?: string;
  image_url?: string;
  icon_uri?: string;
  badge?: string;
  count?: number;
  meta?: Record<string, unknown>;
}

export interface AssetPickerTab {
  id: string;
  label: string;
  searchable?: boolean;
}

export type InputFieldKind = "text" | "boolean" | "select" | "segmented" | "remote_select" | "asset_picker";

export interface InputFieldSpec {
  key: string;
  kind: InputFieldKind;
  title: string;
  description?: string;
  placeholder?: string;
  required?: boolean;
  default_value?: unknown;
  visible_when?: FieldVisibilityRule;
  options?: OptionItem[];
  options_source?: string;
  searchable?: boolean;
  empty_message?: string;
  asset_shape?: "square_pixel" | "cover";
  tabs?: AssetPickerTab[];
  allow_upload?: boolean;
  per_entry_override?: boolean;
  override_key?: string;
  override_label?: string;
}

export interface ActionFormSpec {
  title: string;
  description?: string;
  submit_label?: string;
  auth_required?: boolean;
  auth_message?: string;
  entries?: ActionEntry[];
  fields: InputFieldSpec[];
}

export interface ActionPrepareRequest {
  action: string;
  target: ActionTarget;
  context?: Record<string, unknown>;
}

export interface ActionOptionsRequest {
  action: string;
  source: string;
  tab?: string;
  query?: string;
  cursor?: string;
  target?: ActionTarget;
}

export interface ActionOptionsResult {
  items: OptionItem[];
  next_cursor?: string;
}

export interface AssetUploadRequest {
  action: string;
  field?: string;
  filename: string;
  mime_type: string;
  data_base64: string;
}

export interface AssetUploadResult {
  item: OptionItem;
}

export interface ActionInvokeRequest {
  action: string;
  job_id?: string;
  target: ActionTarget;
  values?: Record<string, unknown>;
  overrides?: Record<string, string>;
}

export interface ActionResult {
  status: "completed" | "client_handoff" | "job_started";
  message?: string;
  external_url?: string;
  open_url?: string;
  download_api_path?: string;
  file_name?: string;
  mime_type?: string;
  job_id?: string;
  data?: Record<string, unknown>;
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
  auth?: AuthSpec;
  actions?: ActionDef[];
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
  title?: string;
  preferred_format?: string;
}

export interface CatalogStageRequest {
  job_id?: string;
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
  job_id?: string;
  results?: CatalogStagedEntry[];
  message?: string;
}

export interface DownloadProgressInfo {
  loadedBytes: number;
  totalBytes?: number;
  fraction?: number;
}

export interface DownloadProgressOptions {
  minPercentDelta?: number;
  minIntervalMs?: number;
  keepaliveIntervalMs?: number;
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
  jobId?: string;
  batchId?: string;
  current: number;
  total: number;
  percent?: number;
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
  refreshInvocationToken(): Promise<string>;
  fetchLoopback(path: string, init?: RequestInit): Promise<Response>;
  openLoopbackStream(path: string): Promise<{ stream: ReadableStream<Uint8Array>; contentLength?: number; contentType?: string }>;
  getBookMarkdown(bookId: string): Promise<{ markdown: string; fileName: string }>;
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

export interface PluginAuthHandlers<TConfig extends Record<string, unknown> = Record<string, unknown>> {
  spec?: AuthSpec;
  status: (ctx: PluginContext<TConfig>) => Promise<AuthStatusResult> | AuthStatusResult;
  begin?: (req: AuthBeginRequest, ctx: PluginContext<TConfig>) => Promise<AuthBeginResult> | AuthBeginResult;
  complete?: (req: AuthCompleteRequest, ctx: PluginContext<TConfig>) => Promise<AuthStatusResult> | AuthStatusResult;
  disconnect?: (ctx: PluginContext<TConfig>) => Promise<AuthStatusResult> | AuthStatusResult;
}

export interface ActionHandlerSpec<TConfig extends Record<string, unknown> = Record<string, unknown>> {
  title?: string;
  description?: string;
  icon?: string;
  targets?: Array<"book" | "series">;
  mediaTypes?: Array<"ebook" | "audiobook">;
  mode?: "immediate" | "form";
  execution?: "sync" | "job";
  prepare?: (req: ActionPrepareRequest, ctx: PluginContext<TConfig>) => Promise<ActionFormSpec> | ActionFormSpec;
  options?: (req: ActionOptionsRequest, ctx: PluginContext<TConfig>) => Promise<ActionOptionsResult> | ActionOptionsResult;
  uploadAsset?: (req: AssetUploadRequest, ctx: PluginContext<TConfig>) => Promise<AssetUploadResult> | AssetUploadResult;
  invoke: (req: ActionInvokeRequest, ctx: PluginContext<TConfig>) => Promise<ActionResult> | ActionResult;
}

export interface PluginDefinition<TConfig extends Record<string, unknown> = Record<string, unknown>> {
  name: string;
  version: string;
  tools?: Record<string, ToolHandlerSpec<TConfig>>;
  commands?: Record<string, CommandHandlerSpec<TConfig>>;
  catalog?: CatalogHandlers<TConfig>;
  auth?: PluginAuthHandlers<TConfig>;
  actions?: Record<string, ActionHandlerSpec<TConfig>>;
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

