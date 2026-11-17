package yoto

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	DefaultAPIBaseURL  = "https://api.yotoplay.com"
	DefaultAuthBaseURL = "https://login.yotoplay.com"

	// DefaultBookIconID is Yoto's built-in 16x16 Book pixel icon.
	DefaultBookIconID = "yoto:#gTMbacpoeSMYqc9fNLJnxPjylraNG6jIrYEWevyzYbA"
)

// Client interacts with the Yoto Developer API and OAuth endpoints.
type Client struct {
	apiBaseURL   string
	authBaseURL  string
	httpClient   *http.Client
	pollInterval time.Duration

	mu                sync.RWMutex
	cachedPublicIcons []DisplayIcon
	publicIconsExpiry time.Time
}

// Option customizes a Yoto Client.
type Option func(*Client)

// WithBaseURLs overrides the API and Auth base URLs (e.g. for testing).
func WithBaseURLs(apiBaseURL, authBaseURL string) Option {
	return func(c *Client) {
		if strings.TrimSpace(apiBaseURL) != "" {
			c.apiBaseURL = strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
		}
		if strings.TrimSpace(authBaseURL) != "" {
			c.authBaseURL = strings.TrimRight(strings.TrimSpace(authBaseURL), "/")
		}
	}
}

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithPollInterval overrides the transcode polling interval (defaults to 500ms).
func WithPollInterval(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.pollInterval = d
		}
	}
}

// NewClient creates a new Yoto API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		apiBaseURL:   DefaultAPIBaseURL,
		authBaseURL:  DefaultAuthBaseURL,
		httpClient:   &http.Client{Timeout: 120 * time.Second},
		pollInterval: 500 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// TokenResponse represents the OAuth 2.0 token response from login.yotoplay.com/oauth/token.
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// ParseJWTExpiry extracts the expiration timestamp from a JWT access token, or defaults to 1 hour from now.
func ParseJWTExpiry(accessToken string, expiresIn int) time.Time {
	if expiresIn > 0 {
		return time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
	}
	parts := strings.Split(accessToken, ".")
	if len(parts) == 3 {
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err == nil {
			var claims struct {
				Exp int64 `json:"exp"`
			}
			if json.Unmarshal(payload, &claims) == nil && claims.Exp > 0 {
				return time.Unix(claims.Exp, 0).UTC()
			}
		}
	}
	return time.Now().UTC().Add(1 * time.Hour)
}

// ExchangeCode exchanges an OAuth 2.0 authorization code + PKCE verifier for access and refresh tokens.
func (c *Client) ExchangeCode(ctx context.Context, clientID, code, codeVerifier, redirectURI string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("code", code)
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}
	if redirectURI != "" {
		form.Set("redirect_uri", redirectURI)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.authBaseURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tok TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}
	tok.ExpiresAt = ParseJWTExpiry(tok.AccessToken, tok.ExpiresIn)
	return &tok, nil
}

// RefreshToken exchanges a single-use refresh token for a new access_token and refresh_token.
func (c *Client) RefreshToken(ctx context.Context, clientID, refreshToken string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", clientID)
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.authBaseURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building refresh token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing token refresh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tok TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("decoding refresh token response: %w", err)
	}
	if tok.RefreshToken == "" {
		tok.RefreshToken = refreshToken
	}
	tok.ExpiresAt = ParseJWTExpiry(tok.AccessToken, tok.ExpiresIn)
	return &tok, nil
}

// DisplayFields represents the 16x16 display icon configuration for a chapter or track.
type DisplayFields struct {
	Icon16x16 string `json:"icon16x16"`
}

// Track represents a single audio track inside a Yoto chapter.
type Track struct {
	Key          string        `json:"key"`
	Title        string        `json:"title"`
	TrackURL     string        `json:"trackUrl"`
	Duration     float64       `json:"duration,omitempty"`
	FileSize     int64         `json:"fileSize,omitempty"`
	Channels     string        `json:"channels,omitempty"`
	Format       string        `json:"format,omitempty"`
	Type         string        `json:"type"`
	OverlayLabel string        `json:"overlayLabel,omitempty"`
	Display      DisplayFields `json:"display"`
}

// Chapter represents a chapter in a Yoto card/playlist.
type Chapter struct {
	Key          string        `json:"key"`
	Title        string        `json:"title"`
	OverlayLabel string        `json:"overlayLabel,omitempty"`
	Tracks       []Track       `json:"tracks"`
	Display      DisplayFields `json:"display"`
}

// CardCover represents cover art metadata on a Yoto card.
type CardCover struct {
	ImageL string `json:"imageL,omitempty"`
}

// CardMediaMetadata represents aggregate duration and size metadata for a Yoto card.
type CardMediaMetadata struct {
	Duration         float64 `json:"duration,omitempty"`
	FileSize         int64   `json:"fileSize,omitempty"`
	ReadableFileSize float64 `json:"readableFileSize,omitempty"`
}

// CardMetadata represents the metadata object on a Yoto card.
type CardMetadata struct {
	Author      string             `json:"author,omitempty"`
	Category    string             `json:"category,omitempty"`
	Description string             `json:"description,omitempty"`
	Cover       *CardCover         `json:"cover,omitempty"`
	Media       *CardMediaMetadata `json:"media,omitempty"`
}

// CardContentBody holds the chapters array and playback config.
type CardContentBody struct {
	Chapters []Chapter `json:"chapters"`
}

// CardContent is the payload for creating or updating a Yoto playlist via POST /content.
type CardContent struct {
	CardID    string          `json:"cardId,omitempty"`
	Title     string          `json:"title"`
	Content   CardContentBody `json:"content"`
	Metadata  CardMetadata    `json:"metadata"`
	CreatedAt string          `json:"createdAt,omitempty"`
	UpdatedAt string          `json:"updatedAt,omitempty"`
}

// YotoCard represents a Yoto MYO playlist/card returned by the API.
type YotoCard struct {
	CardID    string          `json:"cardId"`
	Title     string          `json:"title"`
	Deleted   bool            `json:"deleted,omitempty"`
	CreatedAt string          `json:"createdAt,omitempty"`
	UpdatedAt string          `json:"updatedAt,omitempty"`
	Content   CardContentBody `json:"content,omitempty"`
	Metadata  CardMetadata    `json:"metadata,omitempty"`
}

// GetMyPlaylists retrieves the user's Make Your Own (MYO) playlists from GET /content/mine.
func (c *Client) GetMyPlaylists(ctx context.Context, accessToken string) ([]YotoCard, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/content/mine", nil)
	if err != nil {
		return nil, fmt.Errorf("building get playlists request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching playlists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("fetching playlists failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Cards []YotoCard `json:"cards"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding playlists response: %w", err)
	}

	var active []YotoCard
	for _, card := range result.Cards {
		if !card.Deleted {
			active = append(active, card)
		}
	}
	return active, nil
}

// GetCard retrieves full card details including chapters from GET /content/{cardId}.
func (c *Client) GetCard(ctx context.Context, accessToken, cardID string) (*YotoCard, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/content/"+url.PathEscape(cardID), nil)
	if err != nil {
		return nil, fmt.Errorf("building get card request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching card %s: %w", cardID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("fetching card %s failed with status %d: %s", cardID, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Card YotoCard `json:"card"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding card response: %w", err)
	}
	return &result.Card, nil
}

// CreateOrUpdateContent creates or updates a Yoto MYO playlist via POST /content.
func (c *Client) CreateOrUpdateContent(ctx context.Context, accessToken string, content *CardContent) (*YotoCard, error) {
	payload, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("marshaling card content: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBaseURL+"/content", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building create/update content request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing create/update content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("create/update content failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Card YotoCard `json:"card"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding create/update content response: %w", err)
	}
	if result.Card.CardID == "" && content.CardID != "" {
		result.Card.CardID = content.CardID
	}
	if result.Card.Title == "" {
		result.Card.Title = content.Title
	}
	return &result.Card, nil
}

// DisplayIcon represents a 16x16 pixel icon from Yoto's public library or user custom icons.
type DisplayIcon struct {
	DisplayIconID string   `json:"displayIconId"`
	MediaID       string   `json:"mediaId"`
	Title         string   `json:"title,omitempty"`
	URL           string   `json:"url,omitempty"`
	Public        bool     `json:"public"`
	PublicTags    []string `json:"publicTags,omitempty"`
	UserID        string   `json:"userId,omitempty"`
	CreatedAt     string   `json:"createdAt,omitempty"`
}

// YotoURI returns the "yoto:#<mediaId>" reference string used in chapter/track display.icon16x16.
func (d DisplayIcon) YotoURI() string {
	if strings.HasPrefix(d.MediaID, "yoto:#") {
		return d.MediaID
	}
	return "yoto:#" + d.MediaID
}

// GetPublicIcons fetches Yoto's public 16x16 icon catalog from GET /media/displayIcons/user/yoto.
func (c *Client) GetPublicIcons(ctx context.Context, accessToken string) ([]DisplayIcon, error) {
	c.mu.RLock()
	if len(c.cachedPublicIcons) > 0 && time.Now().Before(c.publicIconsExpiry) {
		icons := make([]DisplayIcon, len(c.cachedPublicIcons))
		copy(icons, c.cachedPublicIcons)
		c.mu.RUnlock()
		return icons, nil
	}
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/media/displayIcons/user/yoto", nil)
	if err != nil {
		return nil, fmt.Errorf("building get public icons request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching public icons: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("fetching public icons failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		DisplayIcons []DisplayIcon `json:"displayIcons"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding public icons response: %w", err)
	}

	c.mu.Lock()
	c.cachedPublicIcons = result.DisplayIcons
	c.publicIconsExpiry = time.Now().Add(30 * time.Minute)
	c.mu.Unlock()

	return result.DisplayIcons, nil
}

// GetUserIcons fetches the user's custom 16x16 icons from GET /media/displayIcons/user/me.
func (c *Client) GetUserIcons(ctx context.Context, accessToken string) ([]DisplayIcon, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/media/displayIcons/user/me", nil)
	if err != nil {
		return nil, fmt.Errorf("building get user icons request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching user icons: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("fetching user icons failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		DisplayIcons []struct {
			DisplayIconID string          `json:"displayIconId"`
			MediaID       string          `json:"mediaId"`
			Title         string          `json:"title,omitempty"`
			URL           json.RawMessage `json:"url,omitempty"`
			Public        bool            `json:"public"`
			UserID        string          `json:"userId,omitempty"`
			CreatedAt     string          `json:"createdAt,omitempty"`
		} `json:"displayIcons"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding user icons response: %w", err)
	}

	icons := make([]DisplayIcon, 0, len(result.DisplayIcons))
	for _, raw := range result.DisplayIcons {
		var urlStr string
		_ = json.Unmarshal(raw.URL, &urlStr)
		icons = append(icons, DisplayIcon{
			DisplayIconID: raw.DisplayIconID,
			MediaID:       raw.MediaID,
			Title:         raw.Title,
			URL:           urlStr,
			Public:        raw.Public,
			UserID:        raw.UserID,
			CreatedAt:     raw.CreatedAt,
		})
	}
	return icons, nil
}

// UploadCustomIcon uploads an image file to POST /media/displayIcons/user/me/upload.
func (c *Client) UploadCustomIcon(ctx context.Context, accessToken string, imageReader io.Reader, filename string, autoConvert bool) (*DisplayIcon, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("creating multipart file field: %w", err)
	}
	if _, err := io.Copy(part, imageReader); err != nil {
		return nil, fmt.Errorf("writing icon file to multipart: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	u, err := url.Parse(c.apiBaseURL + "/media/displayIcons/user/me/upload")
	if err != nil {
		return nil, fmt.Errorf("parsing icon upload URL: %w", err)
	}
	q := u.Query()
	if autoConvert {
		q.Set("autoConvert", "true")
	} else {
		q.Set("autoConvert", "false")
	}
	if filename != "" {
		base := strings.TrimSuffix(filename, "."+strings.TrimPrefix(strings.ToLower(filename[strings.LastIndex(filename, ".")+1:]), "."))
		if idx := strings.LastIndex(filename, "."); idx > 0 {
			base = filename[:idx]
		}
		q.Set("filename", base)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &body)
	if err != nil {
		return nil, fmt.Errorf("building custom icon upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("uploading custom icon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("uploading custom icon failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		DisplayIcon struct {
			DisplayIconID string          `json:"displayIconId"`
			MediaID       string          `json:"mediaId"`
			UserID        string          `json:"userId"`
			URL           json.RawMessage `json:"url"`
		} `json:"displayIcon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding custom icon upload response: %w", err)
	}

	var urlStr string
	_ = json.Unmarshal(result.DisplayIcon.URL, &urlStr)

	return &DisplayIcon{
		DisplayIconID: result.DisplayIcon.DisplayIconID,
		MediaID:       result.DisplayIcon.MediaID,
		Title:         filename,
		URL:           urlStr,
		Public:        false,
		UserID:        result.DisplayIcon.UserID,
	}, nil
}

// UploadURLResponse represents the response from GET /media/transcode/audio/uploadUrl.
type UploadURLResponse struct {
	UploadURL string `json:"uploadUrl"`
	UploadID  string `json:"uploadId"`
}

// RequestAudioUploadURL requests a pre-signed S3 upload URL and uploadId from Yoto.
func (c *Client) RequestAudioUploadURL(ctx context.Context, accessToken string) (*UploadURLResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/media/transcode/audio/uploadUrl", nil)
	if err != nil {
		return nil, fmt.Errorf("building uploadUrl request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting audio upload URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("requesting upload URL failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Upload UploadURLResponse `json:"upload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding uploadUrl response: %w", err)
	}
	if result.Upload.UploadURL == "" {
		return nil, fmt.Errorf("empty uploadUrl returned by Yoto API")
	}
	return &result.Upload, nil
}

// UploadAudioStream uploads an audio stream via HTTP PUT to the pre-signed S3 upload URL.
func (c *Client) UploadAudioStream(ctx context.Context, uploadURL string, audioReader io.Reader, contentType string, contentLength int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, audioReader)
	if err != nil {
		return fmt.Errorf("building audio PUT request: %w", err)
	}
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	req.Header.Set("Content-Type", contentType)
	if contentLength > 0 {
		req.ContentLength = contentLength
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("uploading audio stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("audio PUT failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// TranscodedMediaInfo holds metadata returned after Yoto finishes transcoding an audio upload.
type TranscodedMediaInfo struct {
	Duration float64 `json:"duration"`
	FileSize int64   `json:"fileSize"`
	Channels string  `json:"channels"`
	Format   string  `json:"format"`
	Metadata struct {
		Title string `json:"title"`
	} `json:"metadata"`
}

// TranscodedAudio holds the SHA256 and media info of a transcoded Yoto audio file.
type TranscodedAudio struct {
	TranscodedSha256 string              `json:"transcodedSha256"`
	TranscodedInfo   TranscodedMediaInfo `json:"transcodedInfo"`
}

// WaitForTranscoding polls GET /media/upload/{uploadId}/transcoded?loudnorm=false until transcodedSha256 is present.
func (c *Client) WaitForTranscoding(ctx context.Context, accessToken, uploadID string, maxAttempts int) (*TranscodedAudio, error) {
	if maxAttempts <= 0 {
		maxAttempts = 60
	}
	pollURL := fmt.Sprintf("%s/media/upload/%s/transcoded?loudnorm=false", c.apiBaseURL, url.PathEscape(uploadID))

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
		if err != nil {
			return nil, fmt.Errorf("building transcode poll request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var data struct {
					Transcode TranscodedAudio `json:"transcode"`
				}
				decodeErr := json.NewDecoder(resp.Body).Decode(&data)
				resp.Body.Close()
				if decodeErr == nil && data.Transcode.TranscodedSha256 != "" {
					return &data.Transcode, nil
				}
			} else {
				resp.Body.Close()
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.pollInterval):
		}
	}

	return nil, fmt.Errorf("transcoding timed out after %d attempts for upload %s", maxAttempts, uploadID)
}

// UploadCoverImage uploads a cover image to Yoto via POST /media/coverImage/user/me/upload?autoconvert=true.
func (c *Client) UploadCoverImage(ctx context.Context, accessToken string, imageReader io.Reader, contentType string) (string, error) {
	u := c.apiBaseURL + "/media/coverImage/user/me/upload?autoconvert=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, imageReader)
	if err != nil {
		return "", fmt.Errorf("building cover upload request: %w", err)
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("uploading cover image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("uploading cover image failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		CoverImage struct {
			MediaURL string `json:"mediaUrl"`
		} `json:"coverImage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding cover image upload response: %w", err)
	}
	return result.CoverImage.MediaURL, nil
}
