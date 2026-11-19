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
	Tools    []ToolDef          `json:"tools,omitempty"`
	Commands []CommandDef       `json:"commands,omitempty"`
	Catalog  *CatalogDescriptor `json:"catalog,omitempty"`
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
	PreferredFormat string `json:"preferred_format,omitempty"`
}

// CatalogStageParams requests staging one or more remote catalog items into Shelfd's upload review queue.
type CatalogStageParams struct {
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
	Tools            []ToolDef          `json:"tools,omitempty"`
	Commands         []CommandDef       `json:"commands,omitempty"`
	Catalog          *CatalogDescriptor `json:"catalog,omitempty"`
	Configured       bool               `json:"configured"`
	UpdatedAt        *time.Time         `json:"updated_at,omitempty"`
}

// PluginDetail extends PluginSummary with recent log lines for diagnostics.
type PluginDetail struct {
	PluginSummary
	Logs []string `json:"logs"`
}
