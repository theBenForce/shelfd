package api_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/api"
)

func TestOAuthDiscovery(t *testing.T) {
	fix := setupAPITest(t)
	mux := http.NewServeMux()
	oauthHandler := api.NewOAuthHandler(fix.repo)
	oauthHandler.RegisterRoutes(mux)

	// Test GET /.well-known/oauth-authorization-server with reverse proxy headers
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	req.Header.Set("X-Forwarded-Host", "books.homelab.me")
	req.Header.Set("X-Forwarded-Proto", "https")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var meta map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if meta["issuer"] != "https://books.homelab.me" {
		t.Errorf("expected issuer 'https://books.homelab.me', got '%v'", meta["issuer"])
	}
	if meta["authorization_endpoint"] != "https://books.homelab.me/oauth/authorize" {
		t.Errorf("expected authorization_endpoint 'https://books.homelab.me/oauth/authorize', got '%v'", meta["authorization_endpoint"])
	}
	if meta["token_endpoint"] != "https://books.homelab.me/oauth/token" {
		t.Errorf("expected token_endpoint 'https://books.homelab.me/oauth/token', got '%v'", meta["token_endpoint"])
	}
	if meta["registration_endpoint"] != "https://books.homelab.me/oauth/register" {
		t.Errorf("expected registration_endpoint 'https://books.homelab.me/oauth/register', got '%v'", meta["registration_endpoint"])
	}
}

func TestOAuthDynamicClientRegistration(t *testing.T) {
	fix := setupAPITest(t)
	mux := http.NewServeMux()
	oauthHandler := api.NewOAuthHandler(fix.repo)
	oauthHandler.RegisterRoutes(mux)

	// 1. Successful dynamic registration
	regPayload := `{"client_name":"Stitch","redirect_uris":["https://oauth.stitch.test/callback"],"scope":"mcp"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(regPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var regResp api.RegisterResponse
	if err := json.NewDecoder(rec.Body).Decode(&regResp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if !strings.HasPrefix(regResp.ClientID, "client_") {
		t.Errorf("expected client_id starting with 'client_', got '%s'", regResp.ClientID)
	}
	if !strings.HasPrefix(regResp.ClientSecret, "secret_") {
		t.Errorf("expected client_secret starting with 'secret_', got '%s'", regResp.ClientSecret)
	}
	if regResp.ClientName != "Stitch" {
		t.Errorf("expected client_name 'Stitch', got '%s'", regResp.ClientName)
	}

	// 2. Failed registration (missing redirect_uris)
	badPayload := `{"client_name":"BrokenClient"}`
	reqBad := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(badPayload))
	recBad := httptest.NewRecorder()
	mux.ServeHTTP(recBad, reqBad)

	if recBad.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing redirect_uris, got %d", recBad.Code)
	}
}

func TestOAuthAuthorizeAndTokenExchangeWithPKCE(t *testing.T) {
	fix := setupAPITest(t)
	mux := http.NewServeMux()
	oauthHandler := api.NewOAuthHandler(fix.repo)
	oauthHandler.RegisterRoutes(mux)

	// Use the existing user created by setupAPITest
	adminPass := fix.password

	// 1. Dynamically register client
	regPayload := `{"client_name":"Stitch Agent","redirect_uris":["https://agent.stitch.test/callback"]}`
	reqReg := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(regPayload))
	recReg := httptest.NewRecorder()
	mux.ServeHTTP(recReg, reqReg)

	var regResp api.RegisterResponse
	_ = json.NewDecoder(recReg.Body).Decode(&regResp)
	clientID := regResp.ClientID

	// 2. GET /oauth/authorize -> renders consent HTML
	authURL := fmt.Sprintf("/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&state=xyz123",
		url.QueryEscape(clientID),
		url.QueryEscape("https://agent.stitch.test/callback"),
	)
	reqAuthGet := httptest.NewRequest(http.MethodGet, authURL, nil)
	recAuthGet := httptest.NewRecorder()
	mux.ServeHTTP(recAuthGet, reqAuthGet)

	if recAuthGet.Code != http.StatusOK {
		t.Fatalf("expected status 200 on GET /oauth/authorize, got %d", recAuthGet.Code)
	}
	bodyStr := recAuthGet.Body.String()
	if !strings.Contains(bodyStr, "Authorize Stitch Agent") {
		t.Errorf("expected HTML body to mention 'Authorize Stitch Agent', got: %s", bodyStr)
	}

	// 3. POST /oauth/authorize -> Approve with valid credentials & PKCE S256 challenge
	codeVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sha := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(sha[:])

	form := url.Values{
		"action":                {"authorize"},
		"username":              {fix.user.Username},
		"password":              {adminPass},
		"client_id":             {clientID},
		"redirect_uri":          {"https://agent.stitch.test/callback"},
		"state":                 {"xyz123"},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"mcp"},
	}

	reqAuthPost := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	reqAuthPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recAuthPost := httptest.NewRecorder()
	mux.ServeHTTP(recAuthPost, reqAuthPost)

	if recAuthPost.Code != http.StatusFound {
		t.Fatalf("expected status 302 on approve, got %d: %s", recAuthPost.Code, recAuthPost.Body.String())
	}

	redirectLocation := recAuthPost.Header().Get("Location")
	parsedRedirect, err := url.Parse(redirectLocation)
	if err != nil {
		t.Fatalf("failed to parse redirect location: %v", err)
	}

	authCode := parsedRedirect.Query().Get("code")
	if !strings.HasPrefix(authCode, "code_") {
		t.Fatalf("expected code starting with 'code_', got '%s'", authCode)
	}
	if parsedRedirect.Query().Get("state") != "xyz123" {
		t.Errorf("expected state 'xyz123', got '%s'", parsedRedirect.Query().Get("state"))
	}

	// 4. POST /oauth/token -> Exchange code with wrong verifier -> fails
	badTokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode},
		"client_id":     {clientID},
		"redirect_uri":  {"https://agent.stitch.test/callback"},
		"code_verifier": {"wrong-verifier"},
	}
	reqBadToken := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(badTokenForm.Encode()))
	reqBadToken.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBadToken := httptest.NewRecorder()
	mux.ServeHTTP(recBadToken, reqBadToken)

	if recBadToken.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for wrong verifier, got %d", recBadToken.Code)
	}

	// Code was burned on attempt; request another code for successful exchange
	reqAuthPost2 := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	reqAuthPost2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recAuthPost2 := httptest.NewRecorder()
	mux.ServeHTTP(recAuthPost2, reqAuthPost2)

	parsedRedirect2, _ := url.Parse(recAuthPost2.Header().Get("Location"))
	authCode2 := parsedRedirect2.Query().Get("code")

	// 5. POST /oauth/token -> Exchange code with correct PKCE verifier -> succeeds!
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode2},
		"client_id":     {clientID},
		"redirect_uri":  {"https://agent.stitch.test/callback"},
		"code_verifier": {codeVerifier},
	}
	reqToken := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenForm.Encode()))
	reqToken.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recToken := httptest.NewRecorder()
	mux.ServeHTTP(recToken, reqToken)

	if recToken.Code != http.StatusOK {
		t.Fatalf("expected status 200 on token exchange, got %d: %s", recToken.Code, recToken.Body.String())
	}

	var tokenResp api.TokenResponse
	if err := json.NewDecoder(recToken.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("failed to decode TokenResponse: %v", err)
	}

	if !strings.HasPrefix(tokenResp.AccessToken, "shelfd_") {
		t.Errorf("expected access_token starting with 'shelfd_', got '%s'", tokenResp.AccessToken)
	}
	if tokenResp.TokenType != "Bearer" {
		t.Errorf("expected token_type 'Bearer', got '%s'", tokenResp.TokenType)
	}

	// 6. Test replay prevention: Re-using authCode2 must fail
	reqReplay := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenForm.Encode()))
	reqReplay.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recReplay := httptest.NewRecorder()
	mux.ServeHTTP(recReplay, reqReplay)

	if recReplay.Code != http.StatusBadRequest {
		t.Errorf("expected replay to fail with 400, got %d", recReplay.Code)
	}
}
