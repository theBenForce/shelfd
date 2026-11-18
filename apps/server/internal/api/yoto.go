package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shelfd/shelfd/internal/config"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/yoto"
)

// YotoUploadProgressResponse reports active Yoto upload and transcode progress for a user.
type YotoUploadProgressResponse struct {
	IsActive        bool    `json:"is_active"`
	Title           string  `json:"title,omitempty"`
	CurrentItem     string  `json:"current_item,omitempty"`
	Stage           string  `json:"stage,omitempty"`
	CurrentProgress float64 `json:"current_progress"`
	OverallProgress float64 `json:"overall_progress"`
	CompletedItems  int     `json:"completed_items"`
	TotalItems      int     `json:"total_items"`
}

// YotoHandler handles Yoto account linking, playlists, display icons, and sending books to Yoto.
type YotoHandler struct {
	repo       repository.StorageEngine
	yotoClient *yoto.Client
	cfg        config.YotoConfig
	dataDir    string
	libraryDir string
	logger     *slog.Logger

	progressMu     sync.RWMutex
	progressByUser map[string]YotoUploadProgressResponse
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
		repo:           repo,
		yotoClient:     client,
		cfg:            cfg,
		dataDir:        dataDir,
		libraryDir:     libraryDir,
		logger:         logger,
		progressByUser: make(map[string]YotoUploadProgressResponse),
	}
}

func (h *YotoHandler) setUploadProgress(userID string, p YotoUploadProgressResponse) {
	if userID == "" {
		return
	}
	h.progressMu.Lock()
	h.progressByUser[userID] = p
	h.progressMu.Unlock()
}

func (h *YotoHandler) clearUploadProgress(userID string) {
	if userID == "" {
		return
	}
	h.progressMu.Lock()
	delete(h.progressByUser, userID)
	h.progressMu.Unlock()
}

// GetUploadProgress handles GET /api/v1/integrations/yoto/progress.
func (h *YotoHandler) GetUploadProgress(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	h.progressMu.RLock()
	prog, ok := h.progressByUser[user.ID]
	h.progressMu.RUnlock()
	if !ok {
		prog = YotoUploadProgressResponse{IsActive: false}
	}
	writeJSON(w, http.StatusOK, prog)
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
	return yoto.NormalizeIcon16x16(iconID)
}

type yotoProgressReader struct {
	r       io.Reader
	total   int64
	read    int64
	onBytes func(readBytes, totalBytes int64)
}

func (pr *yotoProgressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		pr.read += int64(n)
		if pr.onBytes != nil {
			pr.onBytes(pr.read, pr.total)
		}
	}
	return n, err
}

type SendToYotoRequest struct {
	Mode             string         `json:"mode"` // "existing" or "new"
	CardID           string         `json:"card_id,omitempty"`
	NewPlaylistTitle string         `json:"new_playlist_title,omitempty"`
	IncludeCover     bool           `json:"include_cover"`
	UseCoverAsIcon   bool           `json:"use_cover_as_icon"`
	DefaultIconID    string         `json:"default_icon_id,omitempty"`
	ChapterIcons     map[int]string `json:"chapter_icons,omitempty"`
	BookIDs          []string       `json:"book_ids,omitempty"`
}

type SendToYotoResponse struct {
	Status       string `json:"status"`
	CardID       string `json:"card_id"`
	Title        string `json:"title"`
	ChapterCount int    `json:"chapter_count"`
	Message      string `json:"message"`
}

func (h *YotoHandler) collectBookAudioFiles(ctx context.Context, book *repository.Book) []*repository.BookFile {
	files, _ := h.repo.GetBookFilesByBookID(ctx, book.ID)
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
	return audioFiles
}

type yotoHTTPError struct {
	statusCode int
	message    string
}

func (e *yotoHTTPError) Error() string {
	return e.message
}

func (h *YotoHandler) uploadBookChaptersToYoto(
	ctx context.Context,
	accessToken string,
	book *repository.Book,
	audioFiles []*repository.BookFile,
	req SendToYotoRequest,
	baseDefaultIconURI string,
	entryOffset int,
	prefixBookTitle bool,
	onFileProgress func(fileIdx int, stage string, fileFraction float64),
) ([]yoto.Chapter, float64, int64, int, *yotoHTTPError) {
	bookDefaultIconURI := baseDefaultIconURI
	if req.UseCoverAsIcon && book.CoverPath != nil && *book.CoverPath != "" {
		if coverDiskPath, err := h.resolveFilePath(*book.CoverPath); err == nil {
			if cf, err := os.Open(coverDiskPath); err == nil {
				uploadedIcon, upErr := h.yotoClient.UploadCustomIcon(ctx, accessToken, cf, filepath.Base(coverDiskPath), true)
				cf.Close()
				if upErr == nil && uploadedIcon != nil && uploadedIcon.MediaID != "" {
					if norm := normalizeYotoIconURI(uploadedIcon.YotoURI()); norm != "" {
						bookDefaultIconURI = norm
					}
				}
			}
		}
	}

	resolveChapterIcon := func(globalIdx int) string {
		if req.ChapterIcons != nil {
			if custom, ok := req.ChapterIcons[globalIdx]; ok && strings.TrimSpace(custom) != "" {
				if norm := normalizeYotoIconURI(custom); norm != "" {
					return norm
				}
			}
		}
		return bookDefaultIconURI
	}

	formatSeriesChapterTitle := func(rawTitle string, singleEntryForBook bool) string {
		trimmed := strings.TrimSpace(rawTitle)
		if !prefixBookTitle {
			return trimmed
		}
		if singleEntryForBook {
			lower := strings.ToLower(trimmed)
			if trimmed == "" || lower == "chapter 1" || lower == "track 1" || lower == "01" || lower == "1" || strings.EqualFold(trimmed, book.Title) {
				return book.Title
			}
		}
		if strings.HasPrefix(strings.ToLower(trimmed), strings.ToLower(book.Title)) {
			return trimmed
		}
		return fmt.Sprintf("%s - %s", book.Title, trimmed)
	}

	audioChapters, _ := h.repo.GetAudioChaptersByBookID(ctx, book.ID)

	var chapters []yoto.Chapter
	var totalDuration float64
	var totalFileSize int64
	currentIdx := entryOffset

	for fileIdx, af := range audioFiles {
		if onFileProgress != nil {
			onFileProgress(fileIdx, "uploading", 0.05)
		}
		resolvedPath, err := h.resolveFilePath(af.FilePath)
		if err != nil {
			return nil, 0, 0, currentIdx, &yotoHTTPError{
				statusCode: http.StatusNotFound,
				message:    fmt.Sprintf("Audio file not found on disk: %s", af.FilePath),
			}
		}

		f, err := os.Open(resolvedPath)
		if err != nil {
			return nil, 0, 0, currentIdx, &yotoHTTPError{
				statusCode: http.StatusInternalServerError,
				message:    fmt.Sprintf("Failed to open audio file: %v", err),
			}
		}
		stat, statErr := f.Stat()
		var fileSize int64
		if statErr == nil {
			fileSize = stat.Size()
		}

		upURL, err := h.yotoClient.RequestAudioUploadURL(ctx, accessToken)
		if err != nil {
			f.Close()
			return nil, 0, 0, currentIdx, &yotoHTTPError{
				statusCode: http.StatusBadGateway,
				message:    fmt.Sprintf("Failed to request Yoto upload URL: %v", err),
			}
		}

		if onFileProgress != nil {
			onFileProgress(fileIdx, "uploading", 0.10)
		}

		mimeType := getAudioMimeType(resolvedPath)
		reader := &yotoProgressReader{
			r:     f,
			total: fileSize,
			onBytes: func(readBytes, totalBytes int64) {
				if onFileProgress != nil && totalBytes > 0 {
					frac := float64(readBytes) / float64(totalBytes)
					if frac > 1 {
						frac = 1
					}
					onFileProgress(fileIdx, "uploading", 0.10+frac*0.55)
				}
			},
		}
		err = h.yotoClient.UploadAudioStream(ctx, upURL.UploadURL, reader, mimeType, fileSize)
		f.Close()
		if err != nil {
			return nil, 0, 0, currentIdx, &yotoHTTPError{
				statusCode: http.StatusBadGateway,
				message:    fmt.Sprintf("Failed to upload audio to Yoto: %v", err),
			}
		}

		if onFileProgress != nil {
			onFileProgress(fileIdx, "transcoding", 0.70)
		}

		transcoded, err := h.yotoClient.WaitForTranscoding(ctx, accessToken, upURL.UploadID, 60)
		if err != nil {
			return nil, 0, 0, currentIdx, &yotoHTTPError{
				statusCode: http.StatusBadGateway,
				message:    fmt.Sprintf("Yoto audio transcoding failed: %v", err),
			}
		}

		if onFileProgress != nil {
			onFileProgress(fileIdx, "transcoding", 0.95)
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

		if len(audioFiles) == 1 && len(audioChapters) > 0 {
			for chIdx, ac := range audioChapters {
				num := currentIdx + 1
				keyStr := fmt.Sprintf("%02d", num)
				labelStr := fmt.Sprintf("%d", num)
				chTitle := strings.TrimSpace(ac.Title)
				if len(audioChapters) == 1 {
					lowerTitle := strings.ToLower(chTitle)
					if chTitle == "" || lowerTitle == "chapter 1" || lowerTitle == "track 1" || lowerTitle == "01" || lowerTitle == "1" {
						chTitle = book.Title
					}
				} else if chTitle == "" {
					chTitle = fmt.Sprintf("Chapter %d", chIdx+1)
				}
				chTitle = formatSeriesChapterTitle(chTitle, len(audioChapters) == 1)
				iconURI := resolveChapterIcon(currentIdx)
				dur := ac.DurationSec
				if dur <= 0 {
					dur = trackDuration
				}
				chapters = append(chapters, yoto.Chapter{
					Key:          keyStr,
					Title:        chTitle,
					OverlayLabel: labelStr,
					Duration:     dur,
					FileSize:     trackSize,
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
				currentIdx++
			}
		} else {
			num := currentIdx + 1
			keyStr := fmt.Sprintf("%02d", num)
			labelStr := fmt.Sprintf("%d", num)
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
			chTitle = formatSeriesChapterTitle(chTitle, len(audioFiles) == 1)
			iconURI := resolveChapterIcon(currentIdx)
			chapters = append(chapters, yoto.Chapter{
				Key:          keyStr,
				Title:        chTitle,
				OverlayLabel: labelStr,
				Duration:     trackDuration,
				FileSize:     trackSize,
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
			currentIdx++
		}
		if onFileProgress != nil {
			onFileProgress(fileIdx, "transcoded", 1.0)
		}
	}

	return chapters, totalDuration, totalFileSize, currentIdx, nil
}

func (h *YotoHandler) saveChaptersToYotoCard(
	ctx context.Context,
	accessToken string,
	req SendToYotoRequest,
	itemLabel string,
	defaultPlaylistTitle string,
	defaultDescription *string,
	coverPath *string,
	newChapters []yoto.Chapter,
	totalDuration float64,
	totalFileSize int64,
) (*SendToYotoResponse, *yotoHTTPError) {
	var uploadedCoverURL string
	if req.IncludeCover && coverPath != nil && *coverPath != "" {
		if coverDiskPath, err := h.resolveFilePath(*coverPath); err == nil {
			if cf, err := os.Open(coverDiskPath); err == nil {
				mime := getAudioMimeType(coverDiskPath)
				if mediaURL, upErr := h.yotoClient.UploadCoverImage(ctx, accessToken, cf, mime); upErr == nil {
					uploadedCoverURL = mediaURL
				}
				cf.Close()
			}
		}
	}

	var contentPayload yoto.CardContent
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	targetCardID := strings.TrimSpace(req.CardID)
	cumulativeDuration := totalDuration
	cumulativeFileSize := totalFileSize

	switch mode {
	case "existing", "append":
		if targetCardID == "" {
			return nil, &yotoHTTPError{
				statusCode: http.StatusBadRequest,
				message:    "card_id is required when appending to an existing playlist",
			}
		}
		existingCard, getErr := h.yotoClient.GetCard(ctx, accessToken, targetCardID)
		if getErr != nil || existingCard == nil {
			return nil, &yotoHTTPError{
				statusCode: http.StatusBadGateway,
				message:    fmt.Sprintf("Failed to load existing Yoto playlist: %v", getErr),
			}
		}
		contentPayload.CardID = existingCard.CardID
		contentPayload.Title = existingCard.Title
		contentPayload.UserID = existingCard.UserID
		contentPayload.Slug = existingCard.Slug
		contentPayload.CreatedAt = existingCard.CreatedAt
		contentPayload.UpdatedAt = existingCard.UpdatedAt
		contentPayload.Metadata = existingCard.Metadata
		contentPayload.Content = existingCard.Content

		existingChapters := existingCard.Content.Chapters
		if existingCard.Metadata.Media != nil {
			cumulativeDuration += existingCard.Metadata.Media.Duration
			cumulativeFileSize += existingCard.Metadata.Media.FileSize
		} else {
			for _, ch := range existingChapters {
				if ch.Duration > 0 {
					cumulativeDuration += ch.Duration
				} else {
					for _, tr := range ch.Tracks {
						cumulativeDuration += tr.Duration
					}
				}
				if ch.FileSize > 0 {
					cumulativeFileSize += ch.FileSize
				} else {
					for _, tr := range ch.Tracks {
						cumulativeFileSize += tr.FileSize
					}
				}
			}
		}

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

	case "replace":
		if targetCardID == "" {
			return nil, &yotoHTTPError{
				statusCode: http.StatusBadRequest,
				message:    "card_id is required when replacing an existing playlist",
			}
		}
		existingCard, getErr := h.yotoClient.GetCard(ctx, accessToken, targetCardID)
		if getErr == nil && existingCard != nil {
			contentPayload.CardID = existingCard.CardID
			contentPayload.Title = existingCard.Title
			contentPayload.UserID = existingCard.UserID
			contentPayload.Slug = existingCard.Slug
			contentPayload.CreatedAt = existingCard.CreatedAt
			contentPayload.Metadata = existingCard.Metadata
			contentPayload.Content = existingCard.Content
		} else {
			contentPayload.CardID = targetCardID
			contentPayload.Title = defaultPlaylistTitle
		}
		if strings.TrimSpace(req.NewPlaylistTitle) != "" {
			contentPayload.Title = strings.TrimSpace(req.NewPlaylistTitle)
		}
		contentPayload.Content.Chapters = newChapters

	default:
		playlistTitle := strings.TrimSpace(req.NewPlaylistTitle)
		if playlistTitle == "" {
			playlistTitle = defaultPlaylistTitle
		}
		contentPayload.Title = playlistTitle
		contentPayload.Content.Chapters = newChapters
		if defaultDescription != nil {
			contentPayload.Metadata.Description = *defaultDescription
		}
	}

	if uploadedCoverURL != "" {
		contentPayload.Metadata.Cover = &yoto.CardCover{ImageL: uploadedCoverURL}
	}
	contentPayload.Metadata.Media = &yoto.CardMediaMetadata{
		Duration:         cumulativeDuration,
		FileSize:         cumulativeFileSize,
		ReadableFileSize: math.Round((float64(cumulativeFileSize)/1024.0/1024.0)*10) / 10,
	}

	savedCard, err := h.yotoClient.CreateOrUpdateContent(ctx, accessToken, &contentPayload)
	if err != nil {
		return nil, &yotoHTTPError{
			statusCode: http.StatusBadGateway,
			message:    fmt.Sprintf("Failed to save Yoto playlist: %v", err),
		}
	}

	actionVerb := "Sent"
	if mode == "existing" || mode == "append" {
		actionVerb = "Appended"
	} else if mode == "replace" {
		actionVerb = "Replaced"
	}

	return &SendToYotoResponse{
		Status:       "sent",
		CardID:       savedCard.CardID,
		Title:        savedCard.Title,
		ChapterCount: len(contentPayload.Content.Chapters),
		Message:      fmt.Sprintf("%s %q on Yoto playlist %q", actionVerb, itemLabel, savedCard.Title),
	}, nil
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

	audioFiles := h.collectBookAudioFiles(r.Context(), book)
	if len(audioFiles) == 0 {
		writeJSONError(w, http.StatusBadRequest, "Audio files are not available for this book")
		return
	}

	accessToken, err := h.getValidAccessToken(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	defaultIconURI := normalizeYotoIconURI(req.DefaultIconID)
	if defaultIconURI == "" {
		defaultIconURI = yoto.DefaultBookIconID
	}

	totalFiles := len(audioFiles)
	defer h.clearUploadProgress(user.ID)

	newChapters, totalDuration, totalFileSize, _, httpErr := h.uploadBookChaptersToYoto(
		r.Context(),
		accessToken,
		book,
		audioFiles,
		req,
		defaultIconURI,
		0,
		false,
		func(fileIdx int, stage string, fileFraction float64) {
			overall := ((float64(fileIdx) + fileFraction) / float64(totalFiles)) * 0.92
			h.setUploadProgress(user.ID, YotoUploadProgressResponse{
				IsActive:        true,
				Title:           book.Title,
				CurrentItem:     book.Title,
				Stage:           stage,
				CurrentProgress: fileFraction,
				OverallProgress: overall,
				CompletedItems:  fileIdx,
				TotalItems:      totalFiles,
			})
		},
	)
	if httpErr != nil {
		writeJSONError(w, httpErr.statusCode, httpErr.message)
		return
	}

	h.setUploadProgress(user.ID, YotoUploadProgressResponse{
		IsActive:        true,
		Title:           book.Title,
		CurrentItem:     book.Title,
		Stage:           "saving",
		CurrentProgress: 0.96,
		OverallProgress: 0.96,
		CompletedItems:  totalFiles,
		TotalItems:      totalFiles,
	})

	resp, httpErr := h.saveChaptersToYotoCard(
		r.Context(),
		accessToken,
		req,
		book.Title,
		book.Title,
		book.Description,
		book.CoverPath,
		newChapters,
		totalDuration,
		totalFileSize,
	)
	if httpErr != nil {
		writeJSONError(w, httpErr.statusCode, httpErr.message)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// SendSeriesToYoto handles POST /api/v1/series/{id}/yoto.
func (h *YotoHandler) SendSeriesToYoto(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	seriesID := r.PathValue("id")
	if seriesID == "" {
		seriesID = extractIDFromPath(r.URL.Path, "series")
	}
	if seriesID == "" {
		writeJSONError(w, http.StatusBadRequest, "Series ID is required")
		return
	}

	var req SendToYotoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	series, err := h.repo.GetSeriesByID(r.Context(), seriesID)
	if errors.Is(err, repository.ErrNotFound) || series == nil {
		writeJSONError(w, http.StatusNotFound, "Series not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load series: %v", err))
		return
	}

	books, err := h.repo.ListBooks(r.Context(), repository.BookFilter{
		SeriesID: &seriesID,
		Limit:    500,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list books in series: %v", err))
		return
	}

	type seriesBookItem struct {
		book     *repository.Book
		files    []*repository.BookFile
		sequence *float64
	}

	var candidates []seriesBookItem
	for _, b := range books {
		files := h.collectBookAudioFiles(r.Context(), b)
		if len(files) == 0 {
			continue
		}
		var seq *float64
		if details, sErr := h.repo.GetBookSeries(r.Context(), b.ID); sErr == nil {
			for _, d := range details {
				if d.ID == seriesID {
					seq = d.SequenceNumber
					break
				}
			}
		}
		candidates = append(candidates, seriesBookItem{
			book:     b,
			files:    files,
			sequence: seq,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		seqA := candidates[i].sequence
		seqB := candidates[j].sequence
		if seqA != nil && seqB != nil && *seqA != *seqB {
			return *seqA < *seqB
		}
		if seqA != nil && seqB == nil {
			return true
		}
		if seqA == nil && seqB != nil {
			return false
		}
		return strings.ToLower(candidates[i].book.Title) < strings.ToLower(candidates[j].book.Title)
	})

	if len(req.BookIDs) > 0 {
		byID := make(map[string]seriesBookItem, len(candidates))
		for _, c := range candidates {
			byID[c.book.ID] = c
		}
		var filtered []seriesBookItem
		for _, id := range req.BookIDs {
			if item, ok := byID[strings.TrimSpace(id)]; ok {
				filtered = append(filtered, item)
			}
		}
		candidates = filtered
	}

	if len(candidates) == 0 {
		writeJSONError(w, http.StatusBadRequest, "Audio files are not available for any books in this series")
		return
	}

	accessToken, err := h.getValidAccessToken(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	defaultIconURI := normalizeYotoIconURI(req.DefaultIconID)
	if defaultIconURI == "" {
		defaultIconURI = yoto.DefaultBookIconID
	}

	var allChapters []yoto.Chapter
	var totalDuration float64
	var totalFileSize int64
	var coverPath *string
	entryOffset := 0

	totalBooks := len(candidates)
	defer h.clearUploadProgress(user.ID)

	for bookIdx, item := range candidates {
		if coverPath == nil && item.book.CoverPath != nil && *item.book.CoverPath != "" {
			coverPath = item.book.CoverPath
		}
		bookFileCount := len(item.files)
		bookChapters, dur, size, nextOffset, httpErr := h.uploadBookChaptersToYoto(
			r.Context(),
			accessToken,
			item.book,
			item.files,
			req,
			defaultIconURI,
			entryOffset,
			true,
			func(fileIdx int, stage string, fileFraction float64) {
				bookProgress := (float64(fileIdx) + fileFraction) / float64(bookFileCount)
				overall := ((float64(bookIdx) + bookProgress) / float64(totalBooks)) * 0.92
				h.setUploadProgress(user.ID, YotoUploadProgressResponse{
					IsActive:        true,
					Title:           series.Name,
					CurrentItem:     item.book.Title,
					Stage:           stage,
					CurrentProgress: bookProgress,
					OverallProgress: overall,
					CompletedItems:  bookIdx,
					TotalItems:      totalBooks,
				})
			},
		)
		if httpErr != nil {
			writeJSONError(w, httpErr.statusCode, httpErr.message)
			return
		}
		allChapters = append(allChapters, bookChapters...)
		totalDuration += dur
		totalFileSize += size
		entryOffset = nextOffset
	}

	h.setUploadProgress(user.ID, YotoUploadProgressResponse{
		IsActive:        true,
		Title:           series.Name,
		CurrentItem:     series.Name,
		Stage:           "saving",
		CurrentProgress: 1.0,
		OverallProgress: 0.96,
		CompletedItems:  totalBooks,
		TotalItems:      totalBooks,
	})

	resp, httpErr := h.saveChaptersToYotoCard(
		r.Context(),
		accessToken,
		req,
		series.Name,
		series.Name,
		series.Description,
		coverPath,
		allChapters,
		totalDuration,
		totalFileSize,
	)
	if httpErr != nil {
		writeJSONError(w, httpErr.statusCode, httpErr.message)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
