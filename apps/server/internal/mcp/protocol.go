package mcp

import "encoding/json"

// JSON-RPC 2.0 Error Codes
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// MCP Protocol Version
const ProtocolVersion = "2024-11-05"

// JSONRPCRequest represents an incoming JSON-RPC 2.0 message.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError defines the standard JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// InitializeResult represents the response payload for the initialize method.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities declares capabilities provided by Shelfd MCP server.
type ServerCapabilities struct {
	Tools map[string]any `json:"tools"`
}

// ServerInfo identifies the server name and version.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool describes an individual tool available for agents to invoke.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ListToolsResult represents the result payload of tools/list.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams represents the input arguments for tools/call.
type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ContentItem represents an individual output item returned by a tool.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallToolResult represents the outcome of executing a tool.
type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// AvailableTools returns the schema definitions for all Shelfd MCP tools.
func AvailableTools() []Tool {
	return []Tool{
		{
			Name:        "search_library",
			Description: "Perform semantic vector search across chapter summaries in the library, returning matching chapters with book titles, authors, and relevance scores.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Natural language search prompt (e.g. 'How do the characters escape the orbital station?')",
					},
					"author": map[string]any{
						"type":        "string",
						"description": "Optional author name filter (e.g. 'William Gibson')",
					},
					"genre": map[string]any{
						"type":        "string",
						"description": "Optional genre filter (e.g. 'Science Fiction')",
					},
					"series": map[string]any{
						"type":        "string",
						"description": "Optional series name filter (e.g. 'Sprawl')",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of search results to return (default: 5, max: 20)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "get_book_metadata",
			Description: "Retrieve detailed metadata for a book, including title, author, series, genres, publisher, description, and the complete Table of Contents with chapter summaries.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"book_id": map[string]any{
						"type":        "string",
						"description": "The unique UUID of the book to retrieve",
					},
				},
				"required": []string{"book_id"},
			},
		},
		{
			Name:        "read_chapter_content",
			Description: "Read plain text content for a specific chapter with paragraph windowing to inspect text without overflowing context limits.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"book_id": map[string]any{
						"type":        "string",
						"description": "The unique UUID of the book",
					},
					"chapter_index": map[string]any{
						"type":        "integer",
						"description": "The chapter sequence index (e.g. 1 for Chapter 1)",
					},
					"start_paragraph": map[string]any{
						"type":        "integer",
						"description": "Optional 1-based start paragraph number (default: 1)",
					},
					"end_paragraph": map[string]any{
						"type":        "integer",
						"description": "Optional 1-based end paragraph number (inclusive, maximum 50 paragraphs per call)",
					},
				},
				"required": []string{"book_id", "chapter_index"},
			},
		},
	}
}
