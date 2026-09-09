package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

type ConnectHandler struct {
	host            string
	port            int
	version         string
	defaultUsername string
}

func NewConnectHandler(host string, port int, version string, defaultUsername string) *ConnectHandler {
	return &ConnectHandler{
		host:            host,
		port:            port,
		version:         version,
		defaultUsername: defaultUsername,
	}
}

type ConnectInfoResponse struct {
	ServerName      string `json:"server_name"`
	Version         string `json:"version"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	APIBase         string `json:"api_base"`
	MCPBase         string `json:"mcp_base"`
	PairingPayload  string `json:"pairing_payload"`
	DefaultUsername string `json:"default_username,omitempty"`
}

func (h *ConnectHandler) GetConnectInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	host := ""
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
		host = xfh
	} else if h.host == "0.0.0.0" || h.host == "" {
		host = r.Host
	} else {
		host = fmt.Sprintf("%s:%d", h.host, h.port)
	}

	scheme := "http"
	if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
		scheme = xfp
	} else if r.TLS != nil {
		scheme = "https"
	}

	serverURL := fmt.Sprintf("%s://%s", scheme, host)
	payloadMap := map[string]any{
		"server":  serverURL,
		"api":     serverURL + "/api/v1",
		"mcp":     serverURL + "/mcp",
		"version": h.version,
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	pairingPayload := base64.StdEncoding.EncodeToString(payloadBytes)

	writeJSON(w, http.StatusOK, ConnectInfoResponse{
		ServerName:      "Shelfd",
		Version:         h.version,
		Host:            h.host,
		Port:            h.port,
		APIBase:         "/api/v1",
		MCPBase:         "/mcp",
		PairingPayload:  pairingPayload,
		DefaultUsername: h.defaultUsername,
	})
}
