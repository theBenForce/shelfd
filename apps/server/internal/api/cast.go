package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/shelfd/shelfd/internal/cast"
)

type CastHandler struct {
	manager cast.Manager
	logger  *slog.Logger
}

func NewCastHandler(manager cast.Manager, logger *slog.Logger) *CastHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CastHandler{
		manager: manager,
		logger:  logger,
	}
}

func (h *CastHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSON(w, http.StatusOK, map[string]any{"devices": []cast.Device{}})
		return
	}
	refresh := r.URL.Query().Get("refresh") == "true" || r.URL.Query().Get("refresh") == "1"
	devices, err := h.manager.ListDevices(r.Context(), refresh)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to discover cast devices: %v", err))
		return
	}
	if devices == nil {
		devices = []cast.Device{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (h *CastHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSON(w, http.StatusOK, map[string]any{"session": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": h.manager.CurrentSession()})
}

func (h *CastHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Chromecast casting is disabled")
		return
	}
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req cast.StartSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}
	req.UserID = user.ID
	req.Username = user.Username

	snap, err := h.manager.StartSession(r.Context(), req)
	if errors.Is(err, cast.ErrDeviceNotFound) {
		writeJSONError(w, http.StatusNotFound, "Cast device not found")
		return
	}
	if errors.Is(err, cast.ErrBookNotFound) {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("Failed to start cast session: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"session": snap})
}

func (h *CastHandler) ControlSession(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Chromecast casting is disabled")
		return
	}

	var cmd cast.ControlCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	snap, err := h.manager.Control(r.Context(), cmd)
	if errors.Is(err, cast.ErrNoActiveSession) {
		writeJSONError(w, http.StatusNotFound, "No active cast session")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Cast control failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"session": snap})
}

func (h *CastHandler) StopSession(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
		return
	}
	if err := h.manager.StopSession(r.Context()); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop cast session: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
}
