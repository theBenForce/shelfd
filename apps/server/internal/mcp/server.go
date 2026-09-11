package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/repository"
)

// Server coordinates the MCP HTTP/SSE transport and JSON-RPC 2.0 message routing.
type Server struct {
	basePath string
	sessions *SessionManager
	executor *ToolExecutor
	repo     repository.StorageEngine
	logger   *slog.Logger
}

// Config defines configuration parameters for initializing the MCP Server.
type Config struct {
	BasePath string // e.g. "/mcp"
	Logger   *slog.Logger
}

// NewServer creates a new MCP Server instance.
func NewServer(repo repository.StorageEngine, aiClient ai.Client, cfg Config) *Server {
	basePath := strings.TrimSuffix(cfg.BasePath, "/")
	if basePath == "" {
		basePath = "/mcp"
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Server{
		basePath: basePath,
		sessions: NewSessionManager(),
		executor: NewToolExecutor(repo, aiClient, logger),
		repo:     repo,
		logger:   logger,
	}
}

// Routes returns an http.Handler with all MCP endpoints registered and authenticated.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	auth := AuthMiddleware(s.repo)
	endpointHandler := auth(http.HandlerFunc(s.handleEndpoint))

	// Modern unified Streamable HTTP + SSE routes
	mux.Handle(s.basePath, endpointHandler)
	mux.Handle(s.basePath+"/", endpointHandler)
	mux.Handle(s.basePath+"/sse", endpointHandler)

	// Legacy message-posting route for older SSE clients
	mux.Handle(s.basePath+"/messages", auth(http.HandlerFunc(s.handleMessages)))

	return mux
}

// handleEndpoint unifies Streamable HTTP and SSE transports on a single endpoint:
// - GET: Establishes a Server-Sent Events (SSE) stream.
// - POST: Processes JSON-RPC 2.0 requests/notifications.
// - HEAD: Probes server availability.
func (s *Server) handleEndpoint(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleSSE(w, r)
	case http.MethodPost:
		s.handleMessages(w, r)
	case http.MethodHead:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSSE handles incoming GET requests establishing Server-Sent Events streams.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	session := s.sessions.Create()
	defer s.sessions.Remove(session.ID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Emit endpoint event declaring message destination
	endpointURL := fmt.Sprintf("%s/messages?sessionId=%s", s.basePath, session.ID)
	if token := r.URL.Query().Get("token"); token != "" {
		endpointURL += "&token=" + url.QueryEscape(token)
	}
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	s.logger.Info("mcp sse client connected", "session_id", session.ID)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			s.logger.Info("mcp sse client disconnected", "session_id", session.ID)
			return
		case <-session.doneCh:
			return
		case <-ticker.C:
			// Heartbeat comment line to prevent reverse proxies (Traefik) and NAT from dropping the connection
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case msg := <-session.sendCh:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		}
	}
}

// handleMessages handles incoming POST requests containing JSON-RPC 2.0 messages.
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024)) // 1MB limit
	if err != nil {
		s.respondError(w, nil, CodeParseError, "Failed to read request body")
		return
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.respondError(w, nil, CodeParseError, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	if req.JSONRPC != "2.0" {
		s.respondError(w, req.ID, CodeInvalidRequest, "Invalid jsonrpc version, expected '2.0'")
		return
	}

	resp := s.dispatch(r.Context(), req)

	// Check if associated with an active SSE session
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		sessionID = r.Header.Get("Mcp-Session-Id")
	}
	var session *Session
	if sessionID != "" {
		session, _ = s.sessions.Get(sessionID)
	}

	// Notifications (no ID) do not require responses
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		s.respondError(w, req.ID, CodeInternalError, "Failed to serialize response")
		return
	}

	if session != nil {
		session.Send(respBytes)
		w.WriteHeader(http.StatusAccepted)
	} else {
		// Deliver response directly in HTTP response body if without active SSE session
		w.Header().Set("Content-Type", "application/json")
		protoVer := r.Header.Get("MCP-Protocol-Version")
		if protoVer == "" {
			protoVer = ProtocolVersion
		}
		w.Header().Set("MCP-Protocol-Version", protoVer)
		w.WriteHeader(http.StatusOK)
		w.Write(respBytes)
	}
}

// dispatch routes a JSON-RPC 2.0 request to the appropriate handler.
func (s *Server) dispatch(ctx context.Context, req JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		protoVer := ProtocolVersion
		if len(req.Params) > 0 {
			var initParams struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			if err := json.Unmarshal(req.Params, &initParams); err == nil && initParams.ProtocolVersion != "" {
				protoVer = initParams.ProtocolVersion
			}
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: InitializeResult{
				ProtocolVersion: protoVer,
				Capabilities: ServerCapabilities{
					Tools: map[string]any{},
				},
				ServerInfo: ServerInfo{
					Name:    "shelfd",
					Version: "0.1.0",
				},
			},
		}

	case "notifications/initialized":
		return nil

	case "ping":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}

	case "tools/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: ListToolsResult{
				Tools: AvailableTools(),
			},
		}

	case "resources/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"resources": []any{}},
		}

	case "prompts/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"prompts": []any{}},
		}

	case "tools/call":
		var params CallToolParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				s.logger.Warn("mcp failed to parse tool call arguments", "error", err)
				return &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &JSONRPCError{
						Code:    CodeInvalidParams,
						Message: fmt.Sprintf("Failed to parse tool call arguments: %v", err),
					},
				}
			}
		}

		s.logger.Info("mcp tool call received", "tool", params.Name, "arguments", params.Arguments)
		start := time.Now()
		result, err := s.executor.Execute(ctx, params.Name, params.Arguments)
		duration := time.Since(start)

		if err != nil {
			s.logger.Error("mcp tool execution error", "tool", params.Name, "duration_ms", duration.Milliseconds(), "error", err)
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    CodeInternalError,
					Message: fmt.Sprintf("Tool execution error: %v", err),
				},
			}
		}

		if result != nil && result.IsError {
			s.logger.Warn("mcp tool returned error", "tool", params.Name, "duration_ms", duration.Milliseconds())
		} else {
			s.logger.Info("mcp tool completed", "tool", params.Name, "duration_ms", duration.Milliseconds())
		}

		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    CodeMethodNotFound,
				Message: fmt.Sprintf("Unknown method '%s'", req.Method),
			},
		}
	}
}

func (s *Server) respondError(w http.ResponseWriter, id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
