package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/shelfd/shelfd/internal/plugins"
)

// PluginHandler handles REST endpoints for plugin management, configuration, commands, and catalog imports.
type PluginHandler struct {
	host   *plugins.Host
	logger *slog.Logger
}

func NewPluginHandler(host *plugins.Host, logger *slog.Logger) *PluginHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PluginHandler{
		host:   host,
		logger: logger,
	}
}

func (h *PluginHandler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	if h.host == nil {
		writeJSON(w, http.StatusOK, map[string]any{"plugins": []plugins.PluginSummary{}})
		return
	}
	user := CurrentUser(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}
	list, err := h.host.ListPlugins(r.Context(), userID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list plugins: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plugins": list})
}

func (h *PluginHandler) DiscoverPlugins(w http.ResponseWriter, r *http.Request) {
	if h.host == nil {
		writeJSON(w, http.StatusOK, map[string]any{"plugins": []plugins.PluginSummary{}})
		return
	}
	if err := h.host.Discover(r.Context()); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to scan plugins directory: %v", err))
		return
	}
	h.ListPlugins(w, r)
}

func (h *PluginHandler) InstallPlugin(w http.ResponseWriter, r *http.Request) {
	if h.host == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Plugin host is not configured")
		return
	}
	if err := r.ParseMultipartForm(55 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse multipart form: %v", err))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Missing 'file' field containing plugin .zip bundle")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 55<<20))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to read archive: %v", err))
		return
	}

	sum, err := h.host.InstallZip(r.Context(), data)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to install plugin: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, sum)
}

func (h *PluginHandler) GetPlugin(w http.ResponseWriter, r *http.Request) {
	if h.host == nil {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	user := CurrentUser(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}
	detail, err := h.host.GetPluginDetail(r.Context(), name, userID)
	if errors.Is(err, plugins.ErrPluginNotFound) {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to inspect plugin: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *PluginHandler) EnablePlugin(w http.ResponseWriter, r *http.Request) {
	h.setPluginEnabled(w, r, true)
}

func (h *PluginHandler) DisablePlugin(w http.ResponseWriter, r *http.Request) {
	h.setPluginEnabled(w, r, false)
}

func (h *PluginHandler) setPluginEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	if h.host == nil {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	user := CurrentUser(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}
	sum, err := h.host.SetEnabled(r.Context(), name, enabled, userID)
	if errors.Is(err, plugins.ErrPluginNotFound) {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update plugin state: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (h *PluginHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if h.host == nil {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	redacted, secretFields, err := h.host.GetRedactedUserConfig(r.Context(), user.ID, name)
	if errors.Is(err, plugins.ErrPluginNotFound) {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get plugin config: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"config":            redacted,
		"secret_fields_set": secretFields,
	})
}

func (h *PluginHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if h.host == nil {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))

	var body map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON payload: %v", err))
		return
	}
	incoming := body
	if nested, ok := body["config"].(map[string]any); ok && len(body) == 1 {
		incoming = nested
	}

	redacted, secretFields, err := h.host.UpdateUserConfig(r.Context(), user.ID, name, incoming)
	if errors.Is(err, plugins.ErrPluginNotFound) {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to save plugin config: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"config":            redacted,
		"secret_fields_set": secretFields,
	})
}

func (h *PluginHandler) InvokeCommand(w http.ResponseWriter, r *http.Request) {
	if h.host == nil {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	command := strings.TrimSpace(r.PathValue("command"))
	if command == "" {
		writeJSONError(w, http.StatusBadRequest, "Command name required")
		return
	}

	var args map[string]any
	if r.ContentLength != 0 {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&args)
	}

	raw, err := h.host.InvokeCommand(r.Context(), name, command, args, user.ID, user.Username)
	h.writePluginResultOrError(w, raw, err)
}

func (h *PluginHandler) GetCatalog(w http.ResponseWriter, r *http.Request) {
	if h.host == nil {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	mode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mode")))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	group := strings.TrimSpace(r.URL.Query().Get("group"))
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	filter := strings.TrimSpace(r.URL.Query().Get("filter"))

	if mode == "" {
		if query != "" {
			mode = "search"
		} else {
			mode = "browse"
		}
	}

	var (
		page *plugins.CatalogPage
		err  error
	)
	if mode == "search" {
		filters := map[string]any{}
		if group != "" {
			filters["group"] = group
		}
		page, err = h.host.SearchCatalog(r.Context(), name, query, cursor, filters, user.ID, user.Username)
	} else {
		page, err = h.host.BrowseCatalog(r.Context(), name, group, cursor, filter, user.ID, user.Username)
	}

	if err != nil {
		h.writePluginError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *PluginHandler) StageCatalogItems(w http.ResponseWriter, r *http.Request) {
	if h.host == nil {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))

	var req struct {
		Items []plugins.CatalogStageItem `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON request: %v", err))
		return
	}
	if len(req.Items) == 0 {
		writeJSONError(w, http.StatusBadRequest, "At least one catalog item is required")
		return
	}

	res, err := h.host.StageCatalogItems(r.Context(), name, req.Items, user.ID, user.Username)
	if err != nil {
		h.writePluginError(w, err)
		return
	}
	statusCode := http.StatusOK
	if res.Async {
		statusCode = http.StatusAccepted
	}
	writeJSON(w, statusCode, res)
}

func (h *PluginHandler) writePluginResultOrError(w http.ResponseWriter, raw json.RawMessage, err error) {
	if err != nil {
		h.writePluginError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if len(raw) == 0 {
		_, _ = w.Write([]byte(`{}`))
		return
	}
	_, _ = w.Write(raw)
}

func (h *PluginHandler) writePluginError(w http.ResponseWriter, err error) {
	if errors.Is(err, plugins.ErrPluginNotFound) {
		writeJSONError(w, http.StatusNotFound, "Plugin not found")
		return
	}
	if errors.Is(err, plugins.ErrPluginDisabled) {
		writeJSONError(w, http.StatusConflict, "Plugin is currently disabled")
		return
	}
	var rpcErr *plugins.RPCError
	if errors.As(err, &rpcErr) {
		if rpcErr.Code == -32001 {
			writeJSONError(w, http.StatusUnauthorized, rpcErr.Message)
			return
		}
		writeJSONError(w, http.StatusBadGateway, rpcErr.Message)
		return
	}
	writeJSONError(w, http.StatusBadGateway, err.Error())
}
