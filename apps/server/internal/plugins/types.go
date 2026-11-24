package plugins

import (
	"encoding/json"
	"time"
)

const ProtocolVersion = "2026-11-01"

// RPCMessage represents a bidirectional JSON-RPC 2.0 frame over NDJSON stdio.
type RPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError defines a JSON-RPC 2.0 error payload.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return e.Message
}

// InitializeParams is sent by the host to the child process immediately after spawn.
type InitializeParams struct {
	ProtocolVersion string `json:"protocol_version"`
	ShelfdVersion   string `json:"shelfd_version"`
	APIBase         string `json:"api_base"`
	DataDir         string `json:"data_dir"`
}

// InitializeResult is returned by the plugin during the startup handshake.
type InitializeResult struct {
	Name     string             `json:"name"`
	Version  string             `json:"version"`
	Auth     *AuthSpec          `json:"auth,omitempty"`
	Tools    []ToolDef          `json:"tools,omitempty"`
	Commands []CommandDef       `json:"commands,omitempty"`
	Catalog  *CatalogDescriptor `json:"catalog,omitempty"`
	Actions  []ActionDef        `json:"actions,omitempty"`
}

// AuthFieldSpec describes an input credential field for plugin auth linking.
type AuthFieldSpec struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

// AuthSpec describes interactive account-linking methods supported by a plugin.
type AuthSpec struct {
	Methods       []string        `json:"methods"` // e.g. ["oauth_pkce", "token"]
	Title         string          `json:"title,omitempty"`
	Description   string          `json:"description,omitempty"`
	CallbackParam string          `json:"callback_param,omitempty"` // default "code"
	Fields        []AuthFieldSpec `json:"fields,omitempty"`
}

// AuthStatusParams requests account-linking status for the current user.
type AuthStatusParams struct {
	Auth InvocationAuth `json:"auth"`
}

// AuthStatusResult reports whether a plugin account is linked for the current user.
type AuthStatusResult struct {
	Linked       bool           `json:"linked"`
	AccountLabel string         `json:"account_label,omitempty"`
	ClientID     string         `json:"client_id,omitempty"`
	ExpiresAt    string         `json:"expires_at,omitempty"`
	AuthError    string         `json:"auth_error,omitempty"`
	Methods      []string       `json:"methods,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

// AuthBeginParams initiates an OAuth 2.0 PKCE or interactive authorization flow.
type AuthBeginParams struct {
	Method        string         `json:"method,omitempty"` // "oauth_pkce"
	RedirectURI   string         `json:"redirect_uri,omitempty"`
	CodeChallenge string         `json:"code_challenge,omitempty"`
	State         string         `json:"state,omitempty"`
	ClientID      string         `json:"client_id,omitempty"`
	Auth          InvocationAuth `json:"auth"`
}

// AuthBeginResult returns the external authorization URL to open in the user's browser.
type AuthBeginResult struct {
	AuthorizeURL string `json:"authorize_url"`
	ClientID     string `json:"client_id,omitempty"`
	State        string `json:"state,omitempty"`
}

// AuthCompleteParams completes an OAuth PKCE code exchange or direct token link.
type AuthCompleteParams struct {
	Method       string         `json:"method,omitempty"` // "oauth_pkce" | "token"
	Code         string         `json:"code,omitempty"`
	CodeVerifier string         `json:"code_verifier,omitempty"`
	RedirectURI  string         `json:"redirect_uri,omitempty"`
	ClientID     string         `json:"client_id,omitempty"`
	AccessToken  string         `json:"access_token,omitempty"`
	RefreshToken string         `json:"refresh_token,omitempty"`
	Credentials  map[string]any `json:"credentials,omitempty"`
	Auth         InvocationAuth `json:"auth"`
}

// AuthDisconnectParams unlinks the user's plugin account credentials.
type AuthDisconnectParams struct {
	Auth InvocationAuth `json:"auth"`
}

// ActionDef describes a book or series action exposed by a plugin.
type ActionDef struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	Targets     []string `json:"targets"`               // ["book", "series"]
	MediaTypes  []string `json:"media_types,omitempty"` // ["ebook", "audiobook"]
	Mode        string   `json:"mode,omitempty"`        // "immediate" | "form" (default "immediate")
	Execution   string   `json:"execution,omitempty"`   // "sync" | "job" (default "sync")
}

// ActionTarget identifies the book or series entity an action operates on.
type ActionTarget struct {
	Type    string   `json:"type"`               // "book" | "series"
	ID      string   `json:"id"`
	Title   string   `json:"title,omitempty"`
	BookIDs []string `json:"book_ids,omitempty"` // optional explicit book IDs for series
}

// ActionEntry represents a chapter, file, or book row inside an action form supporting per-entry overrides.
type ActionEntry struct {
	ID       string `json:"id"`
	Index    int    `json:"index"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	IconURI  string `json:"icon_uri,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// FieldVisibilityRule controls conditional display of a declarative form field.
type FieldVisibilityRule struct {
	Field  string   `json:"field"`
	Equals string   `json:"equals,omitempty"`
	In     []string `json:"in,omitempty"`
}

// OptionItem represents a selectable choice in a select, segmented, remote_select, or asset_picker field.
type OptionItem struct {
	Value       string         `json:"value"`
	Label       string         `json:"label"`
	Subtitle    string         `json:"subtitle,omitempty"`
	Description string         `json:"description,omitempty"`
	ImageURL    string         `json:"image_url,omitempty"`
	IconURI     string         `json:"icon_uri,omitempty"`
	Badge       string         `json:"badge,omitempty"`
	Count       *int           `json:"count,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

// AssetPickerTab defines a tab inside a visual asset_picker field.
type AssetPickerTab struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Searchable bool   `json:"searchable,omitempty"`
}

// InputFieldSpec describes a single declarative form control returned by actions/prepare.
type InputFieldSpec struct {
	Key              string               `json:"key"`
	Kind             string               `json:"kind"` // "text" | "boolean" | "select" | "segmented" | "remote_select" | "asset_picker"
	Title            string               `json:"title"`
	Description      string               `json:"description,omitempty"`
	Placeholder      string               `json:"placeholder,omitempty"`
	Required         bool                 `json:"required,omitempty"`
	DefaultValue     any                  `json:"default_value,omitempty"`
	VisibleWhen      *FieldVisibilityRule `json:"visible_when,omitempty"`
	Options          []OptionItem         `json:"options,omitempty"`
	OptionsSource    string               `json:"options_source,omitempty"`
	Searchable       bool                 `json:"searchable,omitempty"`
	EmptyMessage     string               `json:"empty_message,omitempty"`
	AssetShape       string               `json:"asset_shape,omitempty"` // "square_pixel" | "cover"
	Tabs             []AssetPickerTab     `json:"tabs,omitempty"`
	AllowUpload      bool                 `json:"allow_upload,omitempty"`
	PerEntryOverride bool                 `json:"per_entry_override,omitempty"`
	OverrideKey      string               `json:"override_key,omitempty"`
	OverrideLabel    string               `json:"override_label,omitempty"`
}

// ActionFormSpec is returned by actions/prepare to render a dynamic input dialog.
type ActionFormSpec struct {
	Title        string           `json:"title"`
	Description  string           `json:"description,omitempty"`
	SubmitLabel  string           `json:"submit_label,omitempty"`
	AuthRequired bool             `json:"auth_required,omitempty"`
	AuthMessage  string           `json:"auth_message,omitempty"`
	Entries      []ActionEntry    `json:"entries,omitempty"`
	Fields       []InputFieldSpec `json:"fields"`
}

// ActionPrepareParams requests the declarative form schema for a target book or series.
type ActionPrepareParams struct {
	Action  string         `json:"action"`
	Target  ActionTarget   `json:"target"`
	Context map[string]any `json:"context,omitempty"`
	Auth    InvocationAuth `json:"auth"`
}

// ActionOptionsParams queries dynamic options or visual assets for a form field.
type ActionOptionsParams struct {
	Action string         `json:"action"`
	Source string         `json:"source"`
	Tab    string         `json:"tab,omitempty"`
	Query  string         `json:"query,omitempty"`
	Cursor string         `json:"cursor,omitempty"`
	Target *ActionTarget  `json:"target,omitempty"`
	Auth   InvocationAuth `json:"auth"`
}

// ActionOptionsResult returns selectable OptionItems for remote_select or asset_picker fields.
type ActionOptionsResult struct {
	Items      []OptionItem `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

// AssetUploadParams forwards a user-uploaded custom asset (up to 5 MB) to a plugin action.
type AssetUploadParams struct {
	Action     string         `json:"action"`
	Field      string         `json:"field,omitempty"`
	Filename   string         `json:"filename"`
	MimeType   string         `json:"mime_type"`
	DataBase64 string         `json:"data_base64"`
	Auth       InvocationAuth `json:"auth"`
}

// AssetUploadResult returns the OptionItem created from an uploaded custom asset.
type AssetUploadResult struct {
	Item OptionItem `json:"item"`
}

// ActionInvokeParams requests execution of a plugin action.
type ActionInvokeParams struct {
	Action    string            `json:"action"`
	JobID     string            `json:"job_id,omitempty"`
	Target    ActionTarget      `json:"target"`
	Values    map[string]any    `json:"values,omitempty"`
	Overrides map[string]string `json:"overrides,omitempty"`
	Auth      InvocationAuth    `json:"auth"`
}

// ActionResult summarizes the outcome of an action execution.
type ActionResult struct {
	Status          string         `json:"status"` // "completed" | "client_handoff" | "job_started"
	Message         string         `json:"message,omitempty"`
	ExternalURL     string         `json:"external_url,omitempty"`
	OpenURL         string         `json:"open_url,omitempty"`
	DownloadAPIPath string         `json:"download_api_path,omitempty"`
	FileName        string         `json:"file_name,omitempty"`
	MimeType        string         `json:"mime_type,omitempty"`
	JobID           string         `json:"job_id,omitempty"`
	Job             *PluginJob     `json:"job,omitempty"`
	Data            map[string]any `json:"data,omitempty"`
}

// InvocationAuth carries user attribution, a short-lived scoped loopback JWT, and decrypted per-user config.
type InvocationAuth struct {
	UserID          string         `json:"user_id"`
	InvocationToken string         `json:"invocation_token,omitempty"`
	APIBase         string         `json:"api_base"`
	Config          map[string]any `json:"config,omitempty"`
}

// ToolCallParams is sent for MCP tools/call invocations.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Auth      InvocationAuth `json:"auth"`
}

// ToolCallResult is returned by a plugin for tools/call.
type ToolCallResult struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// CommandInvokeParams is sent for REST commands/invoke invocations.
type CommandInvokeParams struct {
	Command string         `json:"command"`
	Args    map[string]any `json:"args,omitempty"`
	Auth    InvocationAuth `json:"auth"`
}

// CatalogBrowseParams requests owned library items or topic browse results from a plugin.
type CatalogBrowseParams struct {
	Group  string         `json:"group,omitempty"`
	Cursor string         `json:"cursor,omitempty"`
	Filter string         `json:"filter,omitempty"`
	Auth   InvocationAuth `json:"auth"`
}

// CatalogSearchParams requests live catalog search results from a plugin.
type CatalogSearchParams struct {
	Query   string         `json:"query"`
	Cursor  string         `json:"cursor,omitempty"`
	Filters map[string]any `json:"filters,omitempty"`
	Auth    InvocationAuth `json:"auth"`
}

// CatalogFormat describes an available downloadable file format for a remote catalog item.
type CatalogFormat struct {
	Format    string `json:"format"`
	Label     string `json:"label,omitempty"`
	SizeBytes *int64 `json:"size_bytes,omitempty"`
}

// CatalogGroup represents a bundle, shelf, or topic filter chip in a catalog response.
type CatalogGroup struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Count *int   `json:"count,omitempty"`
}

// CatalogItem represents a single remote book or audiobook in a plugin catalog.
type CatalogItem struct {
	ExternalID  string          `json:"external_id"`
	Title       string          `json:"title"`
	Authors     []string        `json:"authors,omitempty"`
	Summary     string          `json:"summary,omitempty"`
	CoverURL    string          `json:"cover_url,omitempty"`
	Group       string          `json:"group,omitempty"`
	Publisher   string          `json:"publisher,omitempty"`
	Language    string          `json:"language,omitempty"`
	Genres      []string        `json:"genres,omitempty"`
	Formats     []CatalogFormat `json:"formats,omitempty"`
	Status      string          `json:"status,omitempty"` // "available" | "staged" | "in_library"
	UploadJobID *string         `json:"upload_job_id,omitempty"`
	BookID      *string         `json:"book_id,omitempty"`
}

// CatalogPage is returned by catalog/browse and catalog/search.
type CatalogPage struct {
	Items      []CatalogItem  `json:"items"`
	Groups     []CatalogGroup `json:"groups,omitempty"`
	NextCursor string         `json:"next_cursor,omitempty"`
	TotalCount *int           `json:"total_count,omitempty"`
}

// CatalogStageItem identifies a remote item and optional format preference to stage for review.
type CatalogStageItem struct {
	ExternalID      string `json:"external_id"`
	Title           string `json:"title,omitempty"`
	PreferredFormat string `json:"preferred_format,omitempty"`
}

// CatalogStageParams requests staging one or more remote catalog items into Shelfd's upload review queue.
type CatalogStageParams struct {
	JobID string             `json:"job_id,omitempty"`
	Items []CatalogStageItem `json:"items"`
	Auth  InvocationAuth     `json:"auth"`
}

// CatalogStagedEntry reports the result of staging a single item.
type CatalogStagedEntry struct {
	ExternalID  string  `json:"external_id"`
	Title       string  `json:"title,omitempty"`
	UploadJobID string  `json:"upload_job_id,omitempty"`
	Status      string  `json:"status"` // "staged" | "queued" | "failed"
	Error       *string `json:"error,omitempty"`
}

// CatalogStageResult summarizes a catalog/stage call.
type CatalogStageResult struct {
	BatchID string               `json:"batch_id,omitempty"`
	Async   bool                 `json:"async,omitempty"`
	JobID   string               `json:"job_id,omitempty"`
	Job     *PluginJob           `json:"job,omitempty"`
	Results []CatalogStagedEntry `json:"results,omitempty"`
	Message string               `json:"message,omitempty"`
}

// PluginStatus summarizes runtime lifecycle state of a managed plugin.
type PluginStatus string

const (
	StatusRunning   PluginStatus = "running"
	StatusIdle      PluginStatus = "idle"
	StatusDisabled  PluginStatus = "disabled"
	StatusStarting  PluginStatus = "starting"
	StatusUnhealthy PluginStatus = "unhealthy"
)

// PluginSummary is returned by GET /api/v1/plugins and GET /api/v1/plugins/{name}.
type PluginSummary struct {
	Name             string             `json:"name"`
	DisplayName      string             `json:"display_name"`
	Version          string             `json:"version"`
	Description      string             `json:"description,omitempty"`
	Homepage         string             `json:"homepage,omitempty"`
	Enabled          bool               `json:"enabled"`
	Daemon           bool               `json:"daemon"`
	Status           PluginStatus       `json:"status"`
	LastError        string             `json:"last_error,omitempty"`
	Capabilities     []string           `json:"capabilities"`
	CatalogModes     []string           `json:"catalog_modes,omitempty"`
	SupportedFormats []string           `json:"supported_formats,omitempty"`
	APIScopes        []string           `json:"api_scopes,omitempty"`
	Permissions      PermissionsSpec    `json:"permissions"`
	ConfigSchema     map[string]any     `json:"config_schema,omitempty"`
	Auth             *AuthSpec          `json:"auth,omitempty"`
	Tools            []ToolDef          `json:"tools,omitempty"`
	Commands         []CommandDef       `json:"commands,omitempty"`
	Catalog          *CatalogDescriptor `json:"catalog,omitempty"`
	Actions          []ActionDef        `json:"actions,omitempty"`
	Configured       bool               `json:"configured"`
	UpdatedAt        *time.Time         `json:"updated_at,omitempty"`
}

// PluginDetail extends PluginSummary with recent log lines for diagnostics.
type PluginDetail struct {
	PluginSummary
	Logs []string `json:"logs"`
}

// PluginInstallResult extends PluginSummary with install/update metadata.
type PluginInstallResult struct {
	PluginSummary
	Updated         bool   `json:"updated"`
	PreviousVersion string `json:"previous_version,omitempty"`
}
