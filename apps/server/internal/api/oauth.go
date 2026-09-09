package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/shelfd/shelfd/internal/mcp"
	"github.com/shelfd/shelfd/internal/repository"
)

// OAuthHandler manages OAuth 2.0 Discovery, RFC 7591 Dynamic Registration,
// interactive user authorization, and token exchange.
type OAuthHandler struct {
	repo repository.StorageEngine
}

// NewOAuthHandler creates a new OAuthHandler instance.
func NewOAuthHandler(repo repository.StorageEngine) *OAuthHandler {
	return &OAuthHandler{repo: repo}
}

// RegisterRoutes attaches all OAuth 2.0 and discovery routes to the provided mux.
func (h *OAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", h.HandleDiscovery)
	mux.HandleFunc("GET /.well-known/openid-configuration", h.HandleDiscovery)
	mux.HandleFunc("POST /oauth/register", h.HandleRegister)
	mux.HandleFunc("GET /oauth/authorize", h.HandleAuthorize)
	mux.HandleFunc("POST /oauth/authorize", h.HandleAuthorize)
	mux.HandleFunc("POST /oauth/token", h.HandleToken)

	// Also support under /api/v1/oauth/
	mux.HandleFunc("GET /api/v1/.well-known/oauth-authorization-server", h.HandleDiscovery)
	mux.HandleFunc("GET /api/v1/.well-known/openid-configuration", h.HandleDiscovery)
	mux.HandleFunc("POST /api/v1/oauth/register", h.HandleRegister)
	mux.HandleFunc("GET /api/v1/oauth/authorize", h.HandleAuthorize)
	mux.HandleFunc("POST /api/v1/oauth/authorize", h.HandleAuthorize)
	mux.HandleFunc("POST /api/v1/oauth/token", h.HandleToken)
}

// resolveBaseURL dynamically derives the server's public base URL honoring reverse proxy headers.
func resolveBaseURL(r *http.Request) string {
	scheme := "http"
	if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
		scheme = xfp
	} else if r.TLS != nil {
		scheme = "https"
	}

	host := r.Host
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
		host = xfh
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}

// HandleDiscovery implements RFC 8414 OAuth 2.0 Authorization Server Metadata
// and OpenID Connect Discovery.
func (h *OAuthHandler) HandleDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	base := resolveBaseURL(r)

	meta := map[string]any{
		"issuer":                                base,
		"authorization_endpoint":                base + "/oauth/authorize",
		"token_endpoint":                        base + "/oauth/token",
		"registration_endpoint":                 base + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256", "plain"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic", "none"},
		"scopes_supported":                      []string{"mcp", "library", "read", "write"},
	}

	writeJSON(w, http.StatusOK, meta)
}

// RegisterRequest models RFC 7591 Dynamic Client Registration request.
type RegisterRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// RegisterResponse models RFC 7591 Dynamic Client Registration response.
type RegisterResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
}

// HandleRegister implements RFC 7591 Dynamic Client Registration.
func (h *OAuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON registration payload")
		return
	}

	req.ClientName = strings.TrimSpace(req.ClientName)
	if req.ClientName == "" {
		req.ClientName = "OAuth Client"
	}

	if len(req.RedirectURIs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "redirect_uris is required and cannot be empty")
		return
	}

	// Validate redirect URIs
	for _, u := range req.RedirectURIs {
		parsed, err := url.Parse(u)
		if err != nil || parsed.Scheme == "" {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid redirect_uri: %s", u))
			return
		}
	}

	if len(req.GrantTypes) == 0 {
		req.GrantTypes = []string{"authorization_code"}
	}
	if len(req.ResponseTypes) == 0 {
		req.ResponseTypes = []string{"code"}
	}
	if req.Scope == "" {
		req.Scope = "mcp"
	}

	clientIDBytes := make([]byte, 16)
	_, _ = rand.Read(clientIDBytes)
	clientID := "client_" + hex.EncodeToString(clientIDBytes)

	clientSecretBytes := make([]byte, 24)
	_, _ = rand.Read(clientSecretBytes)
	clientSecret := "secret_" + hex.EncodeToString(clientSecretBytes)

	urisJoined := strings.Join(req.RedirectURIs, " ")
	grantsJoined := strings.Join(req.GrantTypes, " ")
	responsesJoined := strings.Join(req.ResponseTypes, " ")

	client := &repository.OAuthClient{
		ID:            clientID,
		ClientSecret:  &clientSecret,
		ClientName:    req.ClientName,
		RedirectURIs:  urisJoined,
		GrantTypes:    grantsJoined,
		ResponseTypes: responsesJoined,
		Scope:         &req.Scope,
		CreatedAt:     time.Now().UTC(),
	}

	if err := h.repo.CreateOAuthClient(r.Context(), client); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to register client: %v", err))
		return
	}

	resp := RegisterResponse{
		ClientID:                clientID,
		ClientSecret:            clientSecret,
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              req.GrantTypes,
		ResponseTypes:           req.ResponseTypes,
		Scope:                   req.Scope,
		TokenEndpointAuthMethod: "client_secret_post",
		ClientIDIssuedAt:        client.CreatedAt.Unix(),
	}

	writeJSON(w, http.StatusCreated, resp)
}

var consentPageTemplate = template.Must(template.New("consent").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Authorize {{.ClientName}} — Shelfd</title>
  <style>
    :root {
      --bg: #F8F6F0;
      --card-bg: #FFFFFF;
      --ink: #1F1F1F;
      --muted: #666660;
      --border: #E5E0D5;
      --accent: #2B2622;
      --accent-hover: #403A35;
      --error-bg: #FFF0F0;
      --error-border: #FFC9C9;
      --error-text: #C92A2A;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background-color: var(--bg);
      color: var(--ink);
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      padding: 24px;
    }
    .card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 12px;
      max-width: 440px;
      width: 100%;
      padding: 36px 32px;
      box-shadow: 0 4px 20px rgba(0, 0, 0, 0.04);
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 10px;
      margin-bottom: 24px;
    }
    .brand-icon {
      width: 36px;
      height: 36px;
      background: var(--accent);
      color: var(--bg);
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: bold;
      font-size: 18px;
    }
    .brand-name {
      font-family: Georgia, serif;
      font-size: 22px;
      font-weight: 600;
      letter-spacing: -0.5px;
    }
    h1 {
      font-family: Georgia, serif;
      font-size: 24px;
      font-weight: 600;
      margin-bottom: 8px;
      color: var(--ink);
    }
    p.desc {
      color: var(--muted);
      font-size: 14px;
      line-height: 1.5;
      margin-bottom: 20px;
    }
    .scope-box {
      background: #FDFBF7;
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 12px 14px;
      margin-bottom: 24px;
      font-size: 13px;
      color: var(--ink);
    }
    .scope-box strong {
      display: block;
      margin-bottom: 4px;
    }
    .error {
      background: var(--error-bg);
      border: 1px solid var(--error-border);
      color: var(--error-text);
      padding: 10px 14px;
      border-radius: 6px;
      font-size: 13px;
      margin-bottom: 16px;
    }
    .form-group {
      margin-bottom: 16px;
    }
    label {
      display: block;
      font-size: 12px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: var(--muted);
      margin-bottom: 6px;
    }
    input[type="text"], input[type="password"] {
      width: 100%;
      padding: 10px 12px;
      border: 1px solid var(--border);
      border-radius: 6px;
      font-size: 14px;
      outline: none;
      transition: border-color 0.15s ease;
    }
    input[type="text"]:focus, input[type="password"]:focus {
      border-color: var(--accent);
    }
    .btn-group {
      display: flex;
      gap: 12px;
      margin-top: 24px;
    }
    button {
      flex: 1;
      padding: 12px;
      border-radius: 6px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      transition: background 0.15s ease;
      border: none;
    }
    .btn-primary {
      background: var(--accent);
      color: #FFFFFF;
    }
    .btn-primary:hover {
      background: var(--accent-hover);
    }
    .btn-secondary {
      background: transparent;
      border: 1px solid var(--border);
      color: var(--muted);
    }
    .btn-secondary:hover {
      background: #F8F6F0;
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="brand">
      <div class="brand-icon">S</div>
      <div class="brand-name">Shelfd</div>
    </div>
    <h1>Authorize Application</h1>
    <p class="desc"><strong>{{.ClientName}}</strong> is requesting access to your Shelfd personal library and Model Context Protocol (MCP) server.</p>

    <div class="scope-box">
      <strong>Requested Permissions:</strong>
      Search catalog, retrieve book details, and read chapter passages via MCP tools.
    </div>

    {{if .Error}}
    <div class="error">{{.Error}}</div>
    {{end}}

    <form method="POST" action="/oauth/authorize">
      <input type="hidden" name="client_id" value="{{.ClientID}}">
      <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
      <input type="hidden" name="scope" value="{{.Scope}}">
      <input type="hidden" name="state" value="{{.State}}">
      <input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
      <input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">

      <div class="form-group">
        <label for="username">Administrator Username</label>
        <input type="text" id="username" name="username" value="{{.Username}}" required autofocus>
      </div>

      <div class="form-group">
        <label for="password">Password</label>
        <input type="password" id="password" name="password" required>
      </div>

      <div class="btn-group">
        <button type="submit" name="action" value="deny" class="btn-secondary">Cancel</button>
        <button type="submit" name="action" value="authorize" class="btn-primary">Authorize</button>
      </div>
    </form>
  </div>
</body>
</html>`))

type consentViewData struct {
	ClientName          string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Username            string
	Error               string
}

// HandleAuthorize processes GET (render consent page) and POST (approve/deny consent).
func (h *OAuthHandler) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientID := r.FormValue("client_id")
	if clientID == "" {
		clientID = r.URL.Query().Get("client_id")
	}
	redirectURI := r.FormValue("redirect_uri")
	if redirectURI == "" {
		redirectURI = r.URL.Query().Get("redirect_uri")
	}
	state := r.FormValue("state")
	if state == "" {
		state = r.URL.Query().Get("state")
	}
	codeChallenge := r.FormValue("code_challenge")
	if codeChallenge == "" {
		codeChallenge = r.URL.Query().Get("code_challenge")
	}
	codeChallengeMethod := r.FormValue("code_challenge_method")
	if codeChallengeMethod == "" {
		codeChallengeMethod = r.URL.Query().Get("code_challenge_method")
	}
	scope := r.FormValue("scope")
	if scope == "" {
		scope = r.URL.Query().Get("scope")
	}

	if clientID == "" || redirectURI == "" {
		http.Error(w, "Missing client_id or redirect_uri", http.StatusBadRequest)
		return
	}

	client, err := h.repo.GetOAuthClientByID(r.Context(), clientID)
	if err != nil || client == nil {
		http.Error(w, "Invalid client_id", http.StatusBadRequest)
		return
	}

	// Validate that redirectURI is allowed for this client
	allowedURIs := strings.Fields(client.RedirectURIs)
	isAllowed := false
	for _, u := range allowedURIs {
		if strings.EqualFold(u, redirectURI) {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		http.Error(w, "Unauthorized redirect_uri for client", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		data := consentViewData{
			ClientName:          client.ClientName,
			ClientID:            clientID,
			RedirectURI:         redirectURI,
			Scope:               scope,
			State:               state,
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: codeChallengeMethod,
			Username:            "admin",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = consentPageTemplate.Execute(w, data)
		return
	}

	// Handle POST
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}

	action := r.Form.Get("action")
	if action == "deny" {
		targetURL, _ := url.Parse(redirectURI)
		q := targetURL.Query()
		q.Set("error", "access_denied")
		q.Set("error_description", "The user denied the authorization request")
		if state != "" {
			q.Set("state", state)
		}
		targetURL.RawQuery = q.Encode()
		http.Redirect(w, r, targetURL.String(), http.StatusFound)
		return
	}

	username := strings.TrimSpace(r.Form.Get("username"))
	password := r.Form.Get("password")

	user, err := h.repo.GetUserByUsername(r.Context(), username)
	if err != nil || user == nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		data := consentViewData{
			ClientName:          client.ClientName,
			ClientID:            clientID,
			RedirectURI:         redirectURI,
			Scope:               scope,
			State:               state,
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: codeChallengeMethod,
			Username:            username,
			Error:               "Invalid username or password.",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = consentPageTemplate.Execute(w, data)
		return
	}

	// Credentials valid! Issue authorization code
	codeBytes := make([]byte, 24)
	_, _ = rand.Read(codeBytes)
	authCode := "code_" + hex.EncodeToString(codeBytes)

	codeRecord := &repository.OAuthCode{
		Code:                authCode,
		ClientID:            clientID,
		UserID:              user.ID,
		RedirectURI:         redirectURI,
		CodeChallenge:       &codeChallenge,
		CodeChallengeMethod: &codeChallengeMethod,
		Scope:               &scope,
		ExpiresAt:           time.Now().UTC().Add(5 * time.Minute),
		CreatedAt:           time.Now().UTC(),
	}

	if err := h.repo.CreateOAuthCode(r.Context(), codeRecord); err != nil {
		http.Error(w, "Failed to issue authorization code", http.StatusInternalServerError)
		return
	}

	targetURL, _ := url.Parse(redirectURI)
	q := targetURL.Query()
	q.Set("code", authCode)
	if state != "" {
		q.Set("state", state)
	}
	targetURL.RawQuery = q.Encode()
	http.Redirect(w, r, targetURL.String(), http.StatusFound)
}

// TokenResponse models the standard OAuth 2.0 token response.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope,omitempty"`
}

// HandleToken processes POST /oauth/token for authorization_code exchange with PKCE.
func (h *OAuthHandler) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	_ = r.ParseForm()

	grantType := r.FormValue("grant_type")
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	codeVerifier := r.FormValue("code_verifier")

	// Support HTTP Basic Authentication for client credentials
	if u, p, ok := r.BasicAuth(); ok {
		clientID = u
		clientSecret = p
	}

	// Also support JSON body
	if grantType == "" && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var jsonReq struct {
			GrantType    string `json:"grant_type"`
			Code         string `json:"code"`
			RedirectURI  string `json:"redirect_uri"`
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			CodeVerifier string `json:"code_verifier"`
		}
		if err := json.NewDecoder(r.Body).Decode(&jsonReq); err == nil {
			grantType = jsonReq.GrantType
			code = jsonReq.Code
			redirectURI = jsonReq.RedirectURI
			if clientID == "" {
				clientID = jsonReq.ClientID
			}
			if clientSecret == "" {
				clientSecret = jsonReq.ClientSecret
			}
			if codeVerifier == "" {
				codeVerifier = jsonReq.CodeVerifier
			}
		}
	}

	if grantType != "authorization_code" {
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code")
		return
	}

	if code == "" || clientID == "" || redirectURI == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "code, client_id, and redirect_uri are required")
		return
	}

	client, err := h.repo.GetOAuthClientByID(r.Context(), clientID)
	if err != nil || client == nil {
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "Client not found")
		return
	}

	// Validate client secret if one was issued and provided
	if client.ClientSecret != nil && *client.ClientSecret != "" && clientSecret != "" {
		if subtle.ConstantTimeCompare([]byte(*client.ClientSecret), []byte(clientSecret)) != 1 {
			writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "Invalid client secret")
			return
		}
	}

	codeRecord, err := h.repo.GetOAuthCode(r.Context(), code)
	if err != nil || codeRecord == nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "Invalid or expired authorization code")
		return
	}

	// Replay protection: Burn the code immediately
	_ = h.repo.DeleteOAuthCode(r.Context(), code)

	if codeRecord.ExpiresAt.Before(time.Now().UTC()) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "Authorization code has expired")
		return
	}

	if codeRecord.ClientID != clientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "Authorization code does not match client")
		return
	}

	if codeRecord.RedirectURI != redirectURI {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "Redirect URI mismatch")
		return
	}

	// Validate PKCE (RFC 7636)
	if codeRecord.CodeChallenge != nil && *codeRecord.CodeChallenge != "" {
		if codeVerifier == "" {
			writeOAuthError(w, http.StatusBadRequest, "invalid_request", "code_verifier required for PKCE challenge")
			return
		}

		method := "plain"
		if codeRecord.CodeChallengeMethod != nil && *codeRecord.CodeChallengeMethod != "" {
			method = strings.ToUpper(*codeRecord.CodeChallengeMethod)
		}

		if method == "S256" {
			h := sha256.Sum256([]byte(codeVerifier))
			computed := base64.RawURLEncoding.EncodeToString(h[:])
			if subtle.ConstantTimeCompare([]byte(computed), []byte(*codeRecord.CodeChallenge)) != 1 {
				writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "PKCE code_verifier verification failed")
				return
			}
		} else {
			if subtle.ConstantTimeCompare([]byte(codeVerifier), []byte(*codeRecord.CodeChallenge)) != 1 {
				writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "PKCE code_verifier verification failed")
				return
			}
		}
	}

	// Issue a persistent Shelfd API Token for the user and client
	tokenBytes := make([]byte, 24)
	_, _ = rand.Read(tokenBytes)
	rawToken := "shelfd_" + hex.EncodeToString(tokenBytes)
	tokenHash := mcp.HashToken(rawToken)

	apiToken := &repository.APIToken{
		UserID:    codeRecord.UserID,
		TokenHash: tokenHash,
		Name:      fmt.Sprintf("OAuth: %s", client.ClientName),
		CreatedAt: time.Now().UTC(),
	}

	if err := h.repo.CreateAPIToken(r.Context(), apiToken); err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "Failed to generate access token")
		return
	}

	scope := "mcp"
	if codeRecord.Scope != nil && *codeRecord.Scope != "" {
		scope = *codeRecord.Scope
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, TokenResponse{
		AccessToken: rawToken,
		TokenType:   "Bearer",
		ExpiresIn:   365 * 24 * 3600, // 1 year long-lived bearer token
		Scope:       scope,
	})
}

func writeOAuthError(w http.ResponseWriter, status int, errCode, errDesc string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             errCode,
		"error_description": errDesc,
	})
}
