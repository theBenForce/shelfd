package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/shelfd/shelfd/internal/mcp"
	"github.com/shelfd/shelfd/internal/repository"
)

const (
	pairNonceTTL         = 90 * time.Second
	pairMaxEntries       = 64
	pairRateWindow       = 60 * time.Second
	pairMaxPerIPAttempts = 10
	pairMaxGlobalFails   = 30
	bookMediaTokenTTL    = 2 * time.Hour
)

type pairNonceEntry struct {
	userID    string
	expiresAt time.Time
}

type AuthHandler struct {
	repo           repository.StorageEngine
	jwtSecret      string
	logger         *slog.Logger
	nowFunc        func() time.Time
	pairMu         sync.Mutex
	pairCodes      map[string]pairNonceEntry
	ipAttempts     map[string][]time.Time
	globalFailures []time.Time
}

func NewAuthHandler(repo repository.StorageEngine, jwtSecret string, logger *slog.Logger) *AuthHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthHandler{
		repo:       repo,
		jwtSecret:  jwtSecret,
		logger:     logger,
		nowFunc:    time.Now,
		pairCodes:  make(map[string]pairNonceEntry),
		ipAttempts: make(map[string][]time.Time),
	}
}

func (h *AuthHandler) SetClockForTest(nowFunc func() time.Time) {
	h.pairMu.Lock()
	defer h.pairMu.Unlock()
	if nowFunc == nil {
		h.nowFunc = time.Now
	} else {
		h.nowFunc = nowFunc
	}
}

func issueAPIToken(ctx context.Context, repo repository.StorageEngine, userID, name string) (*repository.APIToken, string, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		cleanName = "API Token"
	}
	if len(cleanName) > 120 {
		cleanName = cleanName[:120]
	}

	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return nil, "", err
	}

	rawToken := "shelfd_" + hex.EncodeToString(buf)
	tokenHash := mcp.HashToken(rawToken)

	token := &repository.APIToken{
		UserID:    userID,
		TokenHash: tokenHash,
		Name:      cleanName,
		CreatedAt: time.Now().UTC(),
	}

	if err := repo.CreateAPIToken(ctx, token); err != nil {
		return nil, "", err
	}

	return token, rawToken, nil
}

type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	DeviceName string `json:"device_name,omitempty"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginResponse struct {
	Token      string       `json:"token"`
	ExpiresAt  time.Time    `json:"expires_at"`
	APIToken   string       `json:"api_token,omitempty"`
	APITokenID string       `json:"api_token_id,omitempty"`
	User       UserResponse `json:"user"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeJSONError(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	user, err := h.repo.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		h.logger.Warn("Failed login attempt (unknown user)", "username", req.Username, "remote_ip", clientIP(r))
		writeJSONError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		h.logger.Warn("Failed login attempt (invalid password)", "username", req.Username, "remote_ip", clientIP(r))
		writeJSONError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	token, expiresAt, err := GenerateJWT(user.ID, user.Username, h.jwtSecret, 7*24*time.Hour)
	if err != nil {
		h.logger.Error("Failed to issue JWT token", "user_id", user.ID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to issue token")
		return
	}

	var rawAPIToken string
	var apiTokenID string
	if deviceName := strings.TrimSpace(req.DeviceName); deviceName != "" {
		apiTok, rawTok, issueErr := issueAPIToken(r.Context(), h.repo, user.ID, deviceName)
		if issueErr != nil {
			h.logger.Error("Failed to issue device API token", "user_id", user.ID, "error", issueErr)
			writeJSONError(w, http.StatusInternalServerError, "Failed to issue device token")
			return
		}
		rawAPIToken = rawTok
		apiTokenID = apiTok.ID
	}

	isSecure := isHTTPSRequest(r)
	http.SetCookie(w, &http.Cookie{
		Name:     "shelfd_token",
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure,
	})

	h.logger.Info("User logged in successfully", "user_id", user.ID, "username", user.Username, "remote_ip", clientIP(r))

	writeJSON(w, http.StatusOK, LoginResponse{
		Token:      token,
		ExpiresAt:  expiresAt,
		APIToken:   rawAPIToken,
		APITokenID: apiTokenID,
		User: UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			CreatedAt: user.CreatedAt,
		},
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if token != "" && !strings.HasPrefix(token, "shelfd_") {
			isSecure := isHTTPSRequest(r)
			http.SetCookie(w, &http.Cookie{
				Name:     "shelfd_token",
				Value:    token,
				Path:     "/",
				Expires:  time.Now().Add(7 * 24 * time.Hour),
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   isSecure,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			CreatedAt: user.CreatedAt,
		},
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	isSecure := isHTTPSRequest(r)
	http.SetCookie(w, &http.Cookie{
		Name:     "shelfd_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure,
	})
	writeJSON(w, http.StatusOK, map[string]any{"message": "Logged out successfully"})
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.CurrentPassword) == "" || strings.TrimSpace(req.NewPassword) == "" {
		writeJSONError(w, http.StatusBadRequest, "Current password and new password are required")
		return
	}

	if len(req.NewPassword) < 6 {
		writeJSONError(w, http.StatusBadRequest, "New password must be at least 6 characters")
		return
	}

	dbUser, err := h.repo.GetUserByID(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		writeJSONError(w, http.StatusUnauthorized, "Current password is incorrect")
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	if err := h.repo.UpdateUserPassword(r.Context(), user.ID, string(newHash)); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Password updated successfully",
	})
}

type CreateTokenRequest struct {
	Name string `json:"name"`
}

type CreateTokenResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *AuthHandler) CreateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token, rawToken, err := issueAPIToken(r.Context(), h.repo, user.ID, req.Name)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to save token")
		return
	}

	writeJSON(w, http.StatusCreated, CreateTokenResponse{
		ID:        token.ID,
		Name:      token.Name,
		Token:     rawToken,
		CreatedAt: token.CreatedAt,
	})
}

type APITokenListItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *AuthHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	tokens, err := h.repo.ListAPITokensByUserID(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list tokens")
		return
	}

	items := make([]APITokenListItem, 0, len(tokens))
	for _, t := range tokens {
		items = append(items, APITokenListItem{
			ID:        t.ID,
			Name:      t.Name,
			CreatedAt: t.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tokens": items,
	})
}

func (h *AuthHandler) DeleteToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	tokenID := r.PathValue("id")
	if tokenID == "" {
		parts := strings.Split(r.URL.Path, "/")
		tokenID = parts[len(parts)-1]
	}
	if tokenID == "" || tokenID == "tokens" {
		writeJSONError(w, http.StatusBadRequest, "Token ID required")
		return
	}

	if err := h.repo.DeleteAPIToken(r.Context(), user.ID, tokenID); err != nil {
		writeJSONError(w, http.StatusNotFound, "Token not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
	})
}

type PairBeginResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type PairExchangeRequest struct {
	Code       string `json:"code"`
	DeviceName string `json:"device_name"`
}

type PairExchangeResponse struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Token     string       `json:"token"`
	CreatedAt time.Time    `json:"created_at"`
	User      UserResponse `json:"user"`
}

func remoteHostOnly(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func pruneTimestamps(src []time.Time, cutoff time.Time) []time.Time {
	idx := 0
	for idx < len(src) && !src[idx].After(cutoff) {
		idx++
	}
	if idx == 0 {
		return src
	}
	out := make([]time.Time, len(src)-idx)
	copy(out, src[idx:])
	return out
}

func (h *AuthHandler) PairBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to generate pairing code")
		return
	}
	rawCode := hex.EncodeToString(buf)
	codeHash := mcp.HashToken(rawCode)

	h.pairMu.Lock()
	now := h.nowFunc().UTC()
	for k, v := range h.pairCodes {
		if !now.Before(v.expiresAt) {
			delete(h.pairCodes, k)
		}
	}
	if len(h.pairCodes) >= pairMaxEntries {
		var oldestKey string
		var oldestExp time.Time
		for k, v := range h.pairCodes {
			if oldestKey == "" || v.expiresAt.Before(oldestExp) {
				oldestKey = k
				oldestExp = v.expiresAt
			}
		}
		if oldestKey != "" {
			delete(h.pairCodes, oldestKey)
		}
	}
	expiresAt := now.Add(pairNonceTTL)
	h.pairCodes[codeHash] = pairNonceEntry{
		userID:    user.ID,
		expiresAt: expiresAt,
	}
	h.pairMu.Unlock()

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, PairBeginResponse{
		Code:      rawCode,
		ExpiresAt: expiresAt,
	})
}

func (h *AuthHandler) PairExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	remoteIP := remoteHostOnly(r)

	h.pairMu.Lock()
	now := h.nowFunc().UTC()
	cutoff := now.Add(-pairRateWindow)
	h.globalFailures = pruneTimestamps(h.globalFailures, cutoff)
	for ip, list := range h.ipAttempts {
		pruned := pruneTimestamps(list, cutoff)
		if len(pruned) == 0 {
			delete(h.ipAttempts, ip)
		} else {
			h.ipAttempts[ip] = pruned
		}
	}
	ipList := h.ipAttempts[remoteIP]
	if len(ipList) >= pairMaxPerIPAttempts || len(h.globalFailures) >= pairMaxGlobalFails {
		h.pairMu.Unlock()
		writeJSONError(w, http.StatusTooManyRequests, "Too many pairing attempts. Try again in a minute.")
		return
	}
	h.pairMu.Unlock()

	var req PairExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.recordPairFailure(remoteIP)
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		h.recordPairFailure(remoteIP)
		writeJSONError(w, http.StatusBadRequest, "Pairing code is required")
		return
	}

	codeHash := mcp.HashToken(code)

	h.pairMu.Lock()
	now = h.nowFunc().UTC()
	entry, ok := h.pairCodes[codeHash]
	if ok {
		delete(h.pairCodes, codeHash)
	}
	valid := ok && now.Before(entry.expiresAt)
	if !valid {
		h.ipAttempts[remoteIP] = append(h.ipAttempts[remoteIP], now)
		h.globalFailures = append(h.globalFailures, now)
	}
	h.pairMu.Unlock()

	if !valid {
		writeJSONError(w, http.StatusUnauthorized, "Invalid or expired pairing code")
		return
	}

	user, err := h.repo.GetUserByID(r.Context(), entry.userID)
	if err != nil || user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Paired user no longer exists")
		return
	}

	deviceName := strings.TrimSpace(req.DeviceName)
	if deviceName == "" {
		deviceName = "Shelfd Mobile"
	}

	apiTok, rawToken, err := issueAPIToken(r.Context(), h.repo, user.ID, deviceName)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to issue device token")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, PairExchangeResponse{
		ID:        apiTok.ID,
		Name:      apiTok.Name,
		Token:     rawToken,
		CreatedAt: apiTok.CreatedAt,
		User: UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			CreatedAt: user.CreatedAt,
		},
	})
}

func (h *AuthHandler) recordPairFailure(remoteIP string) {
	h.pairMu.Lock()
	defer h.pairMu.Unlock()
	now := h.nowFunc().UTC()
	if remoteIP != "" {
		h.ipAttempts[remoteIP] = append(h.ipAttempts[remoteIP], now)
	}
	h.globalFailures = append(h.globalFailures, now)
}

type BookMediaTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (h *AuthHandler) MintBookMediaToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, p := range parts {
			if p == "books" && i+1 < len(parts) {
				bookID = parts[i+1]
				break
			}
		}
	}
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID required")
		return
	}

	book, err := h.repo.GetBookByID(r.Context(), bookID)
	if errors.Is(err, repository.ErrNotFound) || book == nil {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load book")
		return
	}

	jwtStr, expiresAt, err := GenerateCastMediaJWT(user.ID, user.Username, bookID, h.jwtSecret, bookMediaTokenTTL)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to mint book media token")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, BookMediaTokenResponse{
		Token:     jwtStr,
		ExpiresAt: expiresAt,
	})
}
