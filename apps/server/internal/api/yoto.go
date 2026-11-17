package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shelfd/shelfd/internal/config"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/yoto"
)

// YotoHandler handles Yoto account linking, playlists, display icons, and sending books to Yoto.
type YotoHandler struct {
	repo       repository.StorageEngine
	yotoClient *yoto.Client
	cfg        config.YotoConfig
	dataDir    string
	libraryDir string
	logger     *slog.Logger
}

func NewYotoHandler(
	repo repository.StorageEngine,
	cfg config.YotoConfig,
	dataDir string,
	libraryDir string,
	logger *slog.Logger,
) *YotoHandler {
	if logger == nil {
		logger = slog.Default()
	}
	client := yoto.NewClient(
		yoto.WithBaseURLs(cfg.ResolvedAPIBaseURL(), cfg.ResolvedAuthBaseURL()),
	)
	return &YotoHandler{
		repo:       repo,
		yotoClient: client,
		cfg:        cfg,
		dataDir:    dataDir,
		libraryDir: libraryDir,
		logger:     logger,
	}
}

// SetYotoClient allows overriding the Yoto client (e.g. for testing).
func (h *YotoHandler) SetYotoClient(client *yoto.Client) {
	if client != nil {
		h.yotoClient = client
	}
}

type YotoStatusResponse struct {
	IsLinked        bool       `json:"is_linked"`
	ClientID        string     `json:"client_id,omitempty"`
	DefaultClientID string     `json:"default_client_id,omitempty"`
	AuthBaseURL     string     `json:"auth_base_url,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
}

// GetStatus handles GET /api/v1/integrations/yoto/status.
func (h *YotoHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	resp := YotoStatusResponse{
		IsLinked:        false,
		DefaultClientID: h.cfg.ClientID,
		AuthBaseURL:     h.cfg.ResolvedAuthBaseURL(),
	}

	acc, err := h.repo.GetYotoAccountByUserID(r.Context(), user.ID)
	if err == nil && acc != nil && acc.AccessToken != "" {
		resp.IsLinked = true
		resp.ClientID = acc.ClientID
		exp := acc.TokenExpiresAt
		resp.ExpiresAt = &exp
	}

	writeJSON(w, http.StatusOK, resp)
}

type YotoOAuthExchangeRequest struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	ClientID     string `json:"client_id"`
	RedirectURI  string `json:"redirect_uri"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// ExchangeOAuth handles POST /api/v1/integrations/yoto/oauth/exchange.
// Supports both OAuth 2.0 PKCE code exchange and direct token linking.
func (h *YotoHandler) ExchangeOAuth(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req YotoOAuthExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	clientID := strings.TrimSpace(req.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(h.cfg.ClientID)
	}

	var accessToken, refreshToken string
	var expiresAt time.Time

	if strings.TrimSpace(req.AccessToken) != "" {
		accessToken = strings.TrimSpace(req.AccessToken)
		refreshToken = strings.TrimSpace(req.RefreshToken)
		expiresAt = yoto.ParseJWTExpiry(accessToken, 0)
		if clientID == "" {
			clientID = "manual-token"
		}
	} else {
		code := strings.TrimSpace(req.Code)
		if code == "" {
			writeJSONError(w, http.StatusBadRequest, "Authorization code or access_token is required")
			return
		}
		if clientID == "" {
			writeJSONError(w, http.StatusBadRequest, "Yoto Client ID is required")
			return
		}
		redirectURI := strings.TrimSpace(req.RedirectURI)
		if redirectURI == "" {
			redirectURI = strings.TrimSpace(h.cfg.RedirectURI)
		}

		tok, err := h.yotoClient.ExchangeCode(r.Context(), clientID, code, strings.TrimSpace(req.CodeVerifier), redirectURI)
		if err != nil {
			h.logger.Warn("Yoto OAuth token exchange failed", "user_id", user.ID, "error", err)
			writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("Failed to exchange Yoto authorization code: %v", err))
			return
		}
		accessToken = tok.AccessToken
		refreshToken = tok.RefreshToken
		expiresAt = tok.ExpiresAt
	}

	acc := &repository.YotoAccount{
		UserID:         user.ID,
		ClientID:       clientID,
		AccessToken:    accessToken,
		RefreshToken:   refreshToken,
		TokenExpiresAt: expiresAt,
	}
	if err := h.repo.UpsertYotoAccount(r.Context(), acc); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save Yoto account: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, YotoStatusResponse{
		IsLinked:        true,
		ClientID:        clientID,
		DefaultClientID: h.cfg.ClientID,
		AuthBaseURL:     h.cfg.ResolvedAuthBaseURL(),
		ExpiresAt:       &expiresAt,
	})
}

// Unlink handles POST /api/v1/integrations/yoto/unlink.
func (h *YotoHandler) Unlink(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := h.repo.DeleteYotoAccount(r.Context(), user.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to unlink Yoto account: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"is_linked": false,
		"message":   "Yoto account unlinked",
	})
}

// getValidAccessToken retrieves the user's Yoto access token and refreshes it automatically if expired.
func (h *YotoHandler) getValidAccessToken(ctx context.Context, userID string) (string, error) {
	acc, err := h.repo.GetYotoAccountByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", errors.New("Yoto account not linked")
		}
		return "", err
	}

	// Refresh if expiring within 2 minutes and refresh token is available
	if time.Now().UTC().Add(2*time.Minute).After(acc.TokenExpiresAt) && acc.RefreshToken != "" && acc.ClientID != "" && acc.ClientID != "manual-token" {
		tok, refreshErr := h.yotoClient.RefreshToken(ctx, acc.ClientID, acc.RefreshToken)
		if refreshErr == nil && tok.AccessToken != "" {
			acc.AccessToken = tok.AccessToken
			if tok.RefreshToken != "" {
				acc.RefreshToken = tok.RefreshToken
			}
			acc.TokenExpiresAt = tok.ExpiresAt
			_ = h.repo.UpsertYotoAccount(ctx, acc)
		} else if refreshErr != nil {
			h.logger.Warn("Failed to refresh Yoto token, attempting with current token", "user_id", userID, "error", refreshErr)
		}
	}

	return acc.AccessToken, nil
}

type YotoPlaylistSummary struct {
	CardID       string `json:"card_id"`
	Title        string `json:"title"`
	CoverURL     string `json:"cover_url,omitempty"`
	ChapterCount int    `json:"chapter_count"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

// ListPlaylists handles GET /api/v1/integrations/yoto/playlists.
func (h *YotoHandler) ListPlaylists(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	accessToken, err := h.getValidAccessToken(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	cards, err := h.yotoClient.GetMyPlaylists(r.Context(), accessToken)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("Failed to fetch Yoto playlists: %v", err))
		return
	}

	playlists := make([]YotoPlaylistSummary, 0, len(cards))
	for _, c := range cards {
		coverURL := ""
		if c.Metadata.Cover != nil {
			coverURL = c.Metadata.Cover.ImageL
		}
		playlists = append(playlists, YotoPlaylistSummary{
			CardID:       c.CardID,
			Title:        c.Title,
			CoverURL:     coverURL,
			ChapterCount: len(c.Content.Chapters),
			UpdatedAt:    c.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"playlists": playlists,
	})
}

// ListIcons handles GET /api/v1/integrations/yoto/icons.
func (h *YotoHandler) ListIcons(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	accessToken, err := h.getValidAccessToken(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	publicIcons, pubErr := h.yotoClient.GetPublicIcons(r.Context(), accessToken)
	if pubErr != nil {
		h.logger.Warn("Failed to fetch Yoto public icons", "error", pubErr)
		publicIcons = []yoto.DisplayIcon{}
	}

	userIcons, userErr := h.yotoClient.GetUserIcons(r.Context(), accessToken)
	if userErr != nil {
		h.logger.Warn("Failed to fetch Yoto user icons", "error", userErr)
		userIcons = []yoto.DisplayIcon{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"public_icons":     publicIcons,
		"user_icons":       userIcons,
		"default_icon_uri": yoto.DefaultBookIconID,
	})
}

// UploadIcon handles POST /api/v1/integrations/yoto/icons.
func (h *YotoHandler) UploadIcon(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	accessToken, err := h.getValidAccessToken(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid multipart form data")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Missing 'file' field in multipart form")
		return
	}
	defer file.Close()

	filename := header.Filename
	if filename == "" {
		filename = "custom-icon.png"
	}

	icon, err := h.yotoClient.UploadCustomIcon(r.Context(), accessToken, file, filename, true)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("Failed to upload custom icon to Yoto: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, icon)
}

func (h *YotoHandler) resolveFilePath(relOrAbsPath string) (string, error) {
	if relOrAbsPath == "" {
		return "", errors.New("empty file path")
	}
	if filepath.IsAbs(relOrAbsPath) {
		if _, err := os.Stat(relOrAbsPath); err == nil {
			return relOrAbsPath, nil
		}
	}
	if h.libraryDir != "" {
		p := filepath.Join(h.libraryDir, relOrAbsPath)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if h.dataDir != "" {
		p := filepath.Join(h.dataDir, relOrAbsPath)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if _, err := os.Stat(relOrAbsPath); err == nil {
		return relOrAbsPath, nil
	}
	return "", fmt.Errorf("file not found: %s", relOrAbsPath)
}

func isAudioExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".m4b", ".m4a", ".mp3", ".ogg", ".oga", ".flac", ".opus", ".aac", ".wav":
		return true
	default:
		return false
	}
}

func normalizeYotoIconURI(iconID string) string {
	trimmed := strings.TrimSpace(iconID)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "yoto:#") {
		return trimmed
	}
	return "yoto:#" + trimmed
}

type SendToYotoRequest struct {
	Mode             string         `json:"mode"` // "existing" or "new"
	CardID           string         `json:"card_id,omitempty"`
	NewPlaylistTitle string         `json:"new_playlist_title,omitempty"`
	IncludeCover     bool           `json:"include_cover"`
	UseCoverAsIcon   bool           `json:"use_cover_as_icon"`
	DefaultIconID    string         `json:"default_icon_id,omitempty"`
	ChapterIcons     map[int]string `json:"chapter_icons,omitempty"`
}

type SendToYotoResponse struct {
	Status       string `json:"status"`
	CardID       string `json:"card_id"`
	Title        string `json:"title"`
	ChapterCount int    `json:"chapter_count"`
	Message      string `json:"message"`
}

// SendBookToYoto handles POST /api/v1/books/{id}/yoto.
func (h *YotoHandler) SendBookToYoto(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		bookID = extractIDFromPath(r.URL.Path, "books")
	}
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID is required")
		return
	}

	var req SendToYotoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	book, err := h.repo.GetBookByID(r.Context(), bookID)
	if errors.Is(err, repository.ErrNotFound) || book == nil {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load book: %v", err))
		return
	}

	// Collect audio files for this book
	files, _ := h.repo.GetBookFilesByBookID(r.Context(), book.ID)
	var audioFiles []*repository.BookFile
	for _, f := range files {
		if f.FileType == "audiobook" || f.FileType == "audio" || isAudioExtension(f.FilePath) {
			audioFiles = append(audioFiles, f)
		}
	}
	if len(audioFiles) == 0 && (book.BookType == "audiobook" || isAudioExtension(book.FilePath)) {
		audioFiles = append(audioFiles, &repository.BookFile{
			BookID:          book.ID,
			FileType:        "audiobook",
			FilePath:        book.FilePath,
			FileSizeBytes:   book.FileSizeBytes,
			DurationSeconds: book.DurationSeconds,
		})
	}

	if len(audioFiles) == 0 {
		writeJSONError(w, http.StatusBadRequest, "Audio files are not available for this book")
		return
	}

	accessToken, err := h.getValidAccessToken(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Determine default 16x16 icon URI
	defaultIconURI := normalizeYotoIconURI(req.DefaultIconID)
	if defaultIconURI == "" {
		defaultIconURI = yoto.DefaultBookIconID
	}

	if req.UseCoverAsIcon && book.CoverPath != nil && *book.CoverPath != "" {
		if coverDiskPath, err := h.resolveFilePath(*book.CoverPath); err == nil {
			if cf, err := os.Open(coverDiskPath); err == nil {
				uploadedIcon, upErr := h.yotoClient.UploadCustomIcon(r.Context(), accessToken, cf, filepath.Base(coverDiskPath), true)
				cf.Close()
				if upErr == nil && uploadedIcon != nil && uploadedIcon.MediaID != "" {
					defaultIconURI = uploadedIcon.YotoURI()
				}
			}
		}
	}

	resolveChapterIcon := func(idx int) string {
		if req.ChapterIcons != nil {
			if custom, ok := req.ChapterIcons[idx]; ok && strings.TrimSpace(custom) != "" {
				return normalizeYotoIconURI(custom)
			}
		}
		return defaultIconURI
	}

	audioChapters, _ := h.repo.GetAudioChaptersByBookID(r.Context(), book.ID)

	var newChapters []yoto.Chapter
	var totalDuration float64
	var totalFileSize int64

	// Upload each audio file and map chapters
	for fileIdx, af := range audioFiles {
		resolvedPath, err := h.resolveFilePath(af.FilePath)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, fmt.Sprintf("Audio file not found on disk: %s", af.FilePath))
			return
		}

		f, err := os.Open(resolvedPath)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open audio file: %v", err))
			return
		}
		stat, statErr := f.Stat()
		var fileSize int64
		if statErr == nil {
			fileSize = stat.Size()
		}

		upURL, err := h.yotoClient.RequestAudioUploadURL(r.Context(), accessToken)
		if err != nil {
			f.Close()
			writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("Failed to request Yoto upload URL: %v", err))
			return
		}

		mimeType := getAudioMimeType(resolvedPath)
		err = h.yotoClient.UploadAudioStream(r.Context(), upURL.UploadURL, f, mimeType, fileSize)
		f.Close()
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("Failed to upload audio to Yoto: %v", err))
			return
		}

		transcoded, err := h.yotoClient.WaitForTranscoding(r.Context(), accessToken, upURL.UploadID, 60)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("Yoto audio transcoding failed: %v", err))
			return
		}

		trackDuration := transcoded.TranscodedInfo.Duration
		if trackDuration <= 0 && af.DurationSeconds != nil {
			trackDuration = *af.DurationSeconds
		}
		trackSize := transcoded.TranscodedInfo.FileSize
		if trackSize <= 0 {
			trackSize = fileSize
		}
		trackFormat := transcoded.TranscodedInfo.Format
		if trackFormat == "" {
			trackFormat = strings.TrimPrefix(strings.ToLower(filepath.Ext(resolvedPath)), ".")
		}

		totalDuration += trackDuration
		totalFileSize += trackSize

		// If single audio file with multiple parsed audio chapters, create chapter entries for each chapter
		if len(audioFiles) == 1 && len(audioChapters) > 0 {
			for chIdx, ac := range audioChapters {
				keyStr := fmt.Sprintf("%02d", chIdx+1)
				labelStr := fmt.Sprintf("%d", chIdx+1)
				chTitle := strings.TrimSpace(ac.Title)
				if chTitle == "" {
					chTitle = fmt.Sprintf("Chapter %d", chIdx+1)
				}
				iconURI := resolveChapterIcon(chIdx)
				dur := ac.DurationSec
				if dur <= 0 {
					dur = trackDuration
				}
				newChapters = append(newChapters, yoto.Chapter{
					Key:          keyStr,
					Title:        chTitle,
					OverlayLabel: labelStr,
					Display:      yoto.DisplayFields{Icon16x16: iconURI},
					Tracks: []yoto.Track{
						{
							Key:          keyStr,
							Title:        chTitle,
							TrackURL:     "yoto:#" + transcoded.TranscodedSha256,
							Duration:     dur,
							FileSize:     trackSize,
							Channels:     transcoded.TranscodedInfo.Channels,
							Format:       trackFormat,
							Type:         "audio",
							OverlayLabel: labelStr,
							Display:      yoto.DisplayFields{Icon16x16: iconURI},
						},
					},
				})
			}
		} else {
			keyStr := fmt.Sprintf("%02d", fileIdx+1)
			labelStr := fmt.Sprintf("%d", fileIdx+1)
			chTitle := transcoded.TranscodedInfo.Metadata.Title
			if chTitle == "" && fileIdx < len(audioChapters) && audioChapters[fileIdx].Title != "" {
				chTitle = audioChapters[fileIdx].Title
			}
			if chTitle == "" {
				if len(audioFiles) == 1 {
					chTitle = book.Title
				} else {
					base := filepath.Base(af.FilePath)
					chTitle = strings.TrimSuffix(base, filepath.Ext(base))
				}
			}
			iconURI := resolveChapterIcon(fileIdx)
			newChapters = append(newChapters, yoto.Chapter{
				Key:          keyStr,
				Title:        chTitle,
				OverlayLabel: labelStr,
				Display:      yoto.DisplayFields{Icon16x16: iconURI},
				Tracks: []yoto.Track{
					{
						Key:          keyStr,
						Title:        chTitle,
						TrackURL:     "yoto:#" + transcoded.TranscodedSha256,
						Duration:     trackDuration,
						FileSize:     trackSize,
						Channels:     transcoded.TranscodedInfo.Channels,
						Format:       trackFormat,
						Type:         "audio",
						OverlayLabel: labelStr,
						Display:      yoto.DisplayFields{Icon16x16: iconURI},
					},
				},
			})
		}
	}

	// Optional cover image upload
	var uploadedCoverURL string
	if req.IncludeCover && book.CoverPath != nil && *book.CoverPath != "" {
		if coverDiskPath, err := h.resolveFilePath(*book.CoverPath); err == nil {
			if cf, err := os.Open(coverDiskPath); err == nil {
				mime := getAudioMimeType(coverDiskPath)
				if mediaURL, upErr := h.yotoClient.UploadCoverImage(r.Context(), accessToken, cf, mime); upErr == nil {
					uploadedCoverURL = mediaURL
				}
				cf.Close()
			}
		}
	}

	var contentPayload yoto.CardContent
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "existing" && strings.TrimSpace(req.CardID) != "" {
		targetCardID := strings.TrimSpace(req.CardID)
		existingCard, getErr := h.yotoClient.GetCard(r.Context(), accessToken, targetCardID)
		if getErr == nil && existingCard != nil {
			contentPayload.CardID = existingCard.CardID
			contentPayload.Title = existingCard.Title
			contentPayload.Metadata = existingCard.Metadata
			existingChapters := existingCard.Content.Chapters
			offset := len(existingChapters)
			for i := range newChapters {
				num := offset + i + 1
				keyStr := fmt.Sprintf("%02d", num)
				labelStr := fmt.Sprintf("%d", num)
				newChapters[i].Key = keyStr
				newChapters[i].OverlayLabel = labelStr
				for j := range newChapters[i].Tracks {
					newChapters[i].Tracks[j].Key = keyStr
					newChapters[i].Tracks[j].OverlayLabel = labelStr
				}
			}
			contentPayload.Content.Chapters = append(existingChapters, newChapters...)
		} else {
			contentPayload.CardID = targetCardID
			contentPayload.Title = book.Title
			contentPayload.Content.Chapters = newChapters
		}
	} else {
		playlistTitle := strings.TrimSpace(req.NewPlaylistTitle)
		if playlistTitle == "" {
			playlistTitle = book.Title
		}
		contentPayload.Title = playlistTitle
		contentPayload.Content.Chapters = newChapters
		if book.Description != nil {
			contentPayload.Metadata.Description = *book.Description
		}
	}

	if uploadedCoverURL != "" {
		contentPayload.Metadata.Cover = &yoto.CardCover{ImageL: uploadedCoverURL}
	}
	contentPayload.Metadata.Media = &yoto.CardMediaMetadata{
		Duration:         totalDuration,
		FileSize:         totalFileSize,
		ReadableFileSize: math.Round((float64(totalFileSize)/1024.0/1024.0)*10) / 10,
	}

	savedCard, err := h.yotoClient.CreateOrUpdateContent(r.Context(), accessToken, &contentPayload)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("Failed to save Yoto playlist: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, SendToYotoResponse{
		Status:       "sent",
		CardID:       savedCard.CardID,
		Title:        savedCard.Title,
		ChapterCount: len(contentPayload.Content.Chapters),
		Message:      fmt.Sprintf("Sent %q to Yoto playlist %q", book.Title, savedCard.Title),
	})
}
