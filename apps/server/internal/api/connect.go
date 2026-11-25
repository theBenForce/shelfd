package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func firstForwardedValue(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	return strings.TrimSpace(parts[0])
}

func isHTTPSRequest(r *http.Request) bool {
	if xfp := firstForwardedValue(r.Header.Get("X-Forwarded-Proto")); xfp != "" {
		return strings.EqualFold(xfp, "https")
	}
	return r.TLS != nil
}

func resolveExternalOrigin(r *http.Request, cfgHost string, cfgPort int) string {
	scheme := "http"
	if isHTTPSRequest(r) {
		scheme = "https"
	}

	host := ""
	if xfh := firstForwardedValue(r.Header.Get("X-Forwarded-Host")); xfh != "" {
		host = xfh
	} else if (cfgHost == "0.0.0.0" || cfgHost == "") && r.Host != "" {
		host = r.Host
	} else if cfgHost != "" && cfgPort > 0 {
		host = fmt.Sprintf("%s:%d", cfgHost, cfgPort)
	} else {
		host = r.Host
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}

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

	serverURL := resolveExternalOrigin(r, h.host, h.port)
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
