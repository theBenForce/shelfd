package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/shelfd/shelfd/internal/mcp"
	"github.com/shelfd/shelfd/internal/repository"
)

type AuthHandler struct {
	repo      repository.StorageEngine
	jwtSecret string
}

func NewAuthHandler(repo repository.StorageEngine, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      UserResponse `json:"user"`
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
		writeJSONError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeJSONError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	token, expiresAt, err := GenerateJWT(user.ID, user.Username, h.jwtSecret, 7*24*time.Hour)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to issue token")
		return
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
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

	writeJSON(w, http.StatusOK, map[string]any{
		"user": UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			CreatedAt: user.CreatedAt,
		},
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

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = "API Token"
	}

	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to generate token entropy")
		return
	}

	rawToken := "shelfd_" + hex.EncodeToString(buf)
	tokenHash := mcp.HashToken(rawToken)

	token := &repository.APIToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		Name:      req.Name,
	}

	if err := h.repo.CreateAPIToken(r.Context(), token); err != nil {
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

	var items []APITokenListItem
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

	if err := h.repo.DeleteAPIToken(r.Context(), tokenID); err != nil {
		writeJSONError(w, http.StatusNotFound, "Token not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
	})
}
