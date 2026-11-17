package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/config"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/yoto"
)


func TestAPI_YotoIntegration(t *testing.T) {
	// Set up mock Yoto API server
	var mockYotoURL string
	var capturedContent yoto.CardContent

	mockYotoMux := http.NewServeMux()
	mockYotoMux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "yoto-access-token-1",
			"refresh_token": "yoto-refresh-token-1",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	})

	mockYotoMux.HandleFunc("/content/mine", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cards": []map[string]any{
				{
					"cardId":    "card-existing-1",
					"title":     "Kids Bedtime Playlist",
					"updatedAt": "2026-11-16T00:00:00Z",
					"content": map[string]any{
						"chapters": []map[string]any{
							{"key": "01", "title": "Intro"},
						},
					},
				},
			},
		})
	})

	mockYotoMux.HandleFunc("/content/card-existing-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"card": map[string]any{
				"cardId": "card-existing-1",
				"title":  "Kids Bedtime Playlist",
				"content": map[string]any{
					"chapters": []map[string]any{
						{
							"key":   "01",
							"title": "Existing Chapter 1",
							"display": map[string]any{
								"icon16x16": yoto.DefaultBookIconID,
							},
						},
					},
				},
			},
		})
	})

	mockYotoMux.HandleFunc("/media/displayIcons/user/yoto", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"displayIcons": []map[string]any{
				{
					"displayIconId": "icon-pub-1",
					"mediaId":       "media-dragon-1",
					"title":         "Dragon",
					"public":        true,
					"publicTags":    []string{"fantasy", "dragon"},
					"url":           "https://yoto.example.com/dragon.png",
				},
			},
		})
	})

	mockYotoMux.HandleFunc("/media/displayIcons/user/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"displayIcons": []map[string]any{},
		})
	})

	mockYotoMux.HandleFunc("/media/transcode/audio/uploadUrl", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"upload": map[string]any{
				"uploadUrl": mockYotoURL + "/s3-upload",
				"uploadId":  "up-test-1",
			},
		})
	})

	mockYotoMux.HandleFunc("/s3-upload", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mockYotoMux.HandleFunc("/media/upload/up-test-1/transcoded", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transcode": map[string]any{
				"transcodedSha256": "sha256-audiobook-test",
				"transcodedInfo": map[string]any{
					"duration": 620.0,
					"fileSize": 2048000,
					"channels": "stereo",
					"format":   "aac",
				},
			},
		})
	})

	mockYotoMux.HandleFunc("/content", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedContent)
		w.Header().Set("Content-Type", "application/json")
		cardID := capturedContent.CardID
		if cardID == "" {
			cardID = "card-new-99"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"card": map[string]any{
				"cardId": cardID,
				"title":  capturedContent.Title,
			},
		})
	})

	mockYotoSrv := httptest.NewServer(mockYotoMux)
	defer mockYotoSrv.Close()
	mockYotoURL = mockYotoSrv.URL

	handler, repo, token, tmpDir := setupTestAPIWithYoto(t, config.YotoConfig{
		ClientID:    "default-yoto-client",
		APIBaseURL:  mockYotoSrv.URL,
		AuthBaseURL: mockYotoSrv.URL,
	})

	ctx := context.Background()

	// 1. Check initial status (unlinked)
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/integrations/yoto/status", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 for status, got %d: %s", rec1.Code, rec1.Body.String())
	}
	var status1 YotoStatusResponse
	_ = json.Unmarshal(rec1.Body.Bytes(), &status1)
	if status1.IsLinked {
		t.Fatalf("expected IsLinked=false initially")
	}
	if status1.DefaultClientID != "default-yoto-client" {
		t.Errorf("expected default_client_id default-yoto-client, got %s", status1.DefaultClientID)
	}

	// 2. Link account via OAuth code exchange
	exchBody, _ := json.Marshal(YotoOAuthExchangeRequest{
		Code:         "auth-code-123",
		CodeVerifier: "verifier-123",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/yoto/oauth/exchange", bytes.NewReader(exchBody))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for oauth exchange, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var status2 YotoStatusResponse
	_ = json.Unmarshal(rec2.Body.Bytes(), &status2)
	if !status2.IsLinked {
		t.Fatalf("expected IsLinked=true after exchange")
	}

	// 3. Fetch playlists
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/integrations/yoto/playlists", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 for playlists, got %d: %s", rec3.Code, rec3.Body.String())
	}
	var plResp struct {
		Playlists []YotoPlaylistSummary `json:"playlists"`
	}
	_ = json.Unmarshal(rec3.Body.Bytes(), &plResp)
	if len(plResp.Playlists) != 1 || plResp.Playlists[0].CardID != "card-existing-1" {
		t.Fatalf("unexpected playlists: %+v", plResp.Playlists)
	}

	// 4. Fetch icons
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/integrations/yoto/icons", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 for icons, got %d: %s", rec4.Code, rec4.Body.String())
	}

	// 5. Attempt to send EPUB-only book (without audio) -> expect 400 Bad Request
	epubBook := &repository.Book{
		ID:       "book-epub-only",
		Title:    "Pure Text Book",
		FilePath: "PureText.epub",
		BookType: "ebook",
	}
	if err := repo.CreateBook(ctx, epubBook); err != nil {
		t.Fatalf("creating epub book: %v", err)
	}
	sendEpubBody, _ := json.Marshal(SendToYotoRequest{Mode: "new", NewPlaylistTitle: "Should Fail"})
	req5 := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-epub-only/yoto", bytes.NewReader(sendEpubBody))
	req5.Header.Set("Authorization", "Bearer "+token)
	rec5 := httptest.NewRecorder()
	handler.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when sending book without audio, got %d: %s", rec5.Code, rec5.Body.String())
	}

	// 6. Send Audiobook with 2 chapters and custom per-chapter icons to existing playlist
	audioPath := filepath.Join(tmpDir, "TheHobbit.m4b")
	if err := os.WriteFile(audioPath, []byte("fake-m4b-audio-content"), 0o644); err != nil {
		t.Fatalf("writing audio file: %v", err)
	}
	dur := 620.0
	audioBook := &repository.Book{
		ID:              "book-audio-1",
		Title:           "The Hobbit",
		FilePath:        audioPath,
		BookType:        "audiobook",
		DurationSeconds: &dur,
	}
	if err := repo.CreateBook(ctx, audioBook); err != nil {
		t.Fatalf("creating audio book: %v", err)
	}
	err := repo.CreateAudioChapters(ctx, []*repository.AudioChapter{
		{BookID: "book-audio-1", ChapterIndex: 0, Title: "An Unexpected Party", StartOffsetSec: 0, DurationSec: 300},
		{BookID: "book-audio-1", ChapterIndex: 1, Title: "Roast Mutton", StartOffsetSec: 300, DurationSec: 320},
	})
	if err != nil {
		t.Fatalf("creating audio chapters: %v", err)
	}

	sendAudioBody, _ := json.Marshal(SendToYotoRequest{
		Mode:          "existing",
		CardID:        "card-existing-1",
		DefaultIconID: "yoto:#default-icon-99",
		ChapterIcons: map[int]string{
			1: "yoto:#media-dragon-1",
		},
	})
	req6 := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-audio-1/yoto", bytes.NewReader(sendAudioBody))
	req6.Header.Set("Authorization", "Bearer "+token)
	rec6 := httptest.NewRecorder()
	handler.ServeHTTP(rec6, req6)
	if rec6.Code != http.StatusOK {
		t.Fatalf("expected 200 sending audiobook to Yoto, got %d: %s", rec6.Code, rec6.Body.String())
	}

	var sendResp SendToYotoResponse
	_ = json.Unmarshal(rec6.Body.Bytes(), &sendResp)
	if sendResp.CardID != "card-existing-1" || sendResp.ChapterCount != 3 {
		t.Fatalf("expected card-existing-1 with 3 total chapters (1 existing + 2 added), got %+v", sendResp)
	}

	// Verify chapter icons on captured payload
	if len(capturedContent.Content.Chapters) != 3 {
		t.Fatalf("expected 3 chapters in captured content, got %d", len(capturedContent.Content.Chapters))
	}
	if capturedContent.Content.Chapters[1].Display.Icon16x16 != "yoto:#default-icon-99" {
		t.Errorf("expected chapter 2 to use default icon yoto:#default-icon-99, got %s", capturedContent.Content.Chapters[1].Display.Icon16x16)
	}
	if capturedContent.Content.Chapters[2].Display.Icon16x16 != "yoto:#media-dragon-1" {
		t.Errorf("expected chapter 3 to use custom chapter icon yoto:#media-dragon-1, got %s", capturedContent.Content.Chapters[2].Display.Icon16x16)
	}

	// 7. Unlink account
	req7 := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/yoto/unlink", strings.NewReader(`{}`))
	req7.Header.Set("Authorization", "Bearer "+token)
	rec7 := httptest.NewRecorder()
	handler.ServeHTTP(rec7, req7)
	if rec7.Code != http.StatusOK {
		t.Fatalf("expected 200 unlinking account, got %d", rec7.Code)
	}
}

func setupTestAPIWithYoto(t *testing.T, yotoCfg config.YotoConfig) (http.Handler, repository.StorageEngine, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("opening sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("running migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)

	user := &repository.User{
		ID:           "user-yoto-1",
		Username:     "admin",
		PasswordHash: "$2a$10$7EqJtq98hPqEX7fNZaFWoO5f7.sB0x2t3N.C.r9G.yU7p5k5p5k5p",
		CreatedAt:    time.Now().UTC(),
	}
	handler := NewRouter(RouterConfig{
		Repo:       repo,
		Yoto:       yotoCfg,
		DataDir:    tmpDir,
		LibraryDir: tmpDir,
		JWTSecret:  "test-secret-yoto",
	})

	if err := repo.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("creating user: %v", err)
	}
	tok, _, err := GenerateJWT(user.ID, user.Username, "test-secret-yoto", 1*time.Hour)
	if err != nil {
		t.Fatalf("generating jwt: %v", err)
	}
	return handler, repo, tok, tmpDir
}

func TestYotoUIContractVerification(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "pnpm-workspace.yaml")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repo root")
		}
		dir = parent
	}

	readDart := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(dir, "apps", "app", rel))
		if err != nil {
			t.Fatalf("failed reading %s: %v", rel, err)
		}
		return string(b)
	}

	bookDetail := readDart("lib/ui/features/book_detail/book_detail_view.dart")
	for _, needle := range []string{
		"Key('menu_item_send_yoto')",
		"value: 'yoto'",
		"enabled: hasAudio",
		"Send to Yoto",
		"showSendToYotoDialog(context, book)",
	} {
		if !strings.Contains(bookDetail, needle) {
			t.Errorf("book_detail_view.dart missing expected Yoto contract %q", needle)
		}
	}

	sendDialog := readDart("lib/ui/features/book_detail/send_to_yoto_dialog.dart")
	for _, needle := range []string{
		"Key('send_to_yoto_dialog')",
		"Key('yoto_change_default_icon_button')",
		"Key('yoto_use_cover_as_icon_switch')",
		"Key('yoto_customize_chapter_icons_tile')",
		"Key('confirm_send_to_yoto_button')",
	} {
		if !strings.Contains(sendDialog, needle) {
			t.Errorf("send_to_yoto_dialog.dart missing expected Yoto contract %q", needle)
		}
	}

	iconPicker := readDart("lib/ui/features/book_detail/yoto_icon_picker_dialog.dart")
	for _, needle := range []string{
		"Key('yoto_icon_picker_dialog')",
		"Key('yoto_upload_custom_icon_button')",
		"Key('yoto_icon_search_field')",
	} {
		if !strings.Contains(iconPicker, needle) {
			t.Errorf("yoto_icon_picker_dialog.dart missing expected Yoto contract %q", needle)
		}
	}

	settingsView := readDart("lib/ui/features/settings/settings_view.dart")
	for _, needle := range []string{
		"Key('yoto_integration_settings_card')",
		"Yoto Player Integration",
		"Key('yoto_client_id_field')",
		"Key('link_yoto_oauth_button')",
		"Key('unlink_yoto_account_button')",
	} {
		if !strings.Contains(settingsView, needle) {
			t.Errorf("settings_view.dart missing expected Yoto contract %q", needle)
		}
	}
}


