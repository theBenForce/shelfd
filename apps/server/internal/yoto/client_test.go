package yoto

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_OAuthExchangeAndRefresh(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		grantType := r.FormValue("grant_type")
		switch grantType {
		case "authorization_code":
			if r.FormValue("code") != "valid-code" || r.FormValue("code_verifier") != "valid-verifier" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "acc-123",
				"refresh_token": "ref-123",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
		case "refresh_token":
			if r.FormValue("refresh_token") != "ref-123" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "acc-456",
				"refresh_token": "ref-456",
				"token_type":    "Bearer",
				"expires_in":    7200,
			})
		default:
			http.Error(w, "unsupported grant_type", http.StatusBadRequest)
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(WithBaseURLs(srv.URL, srv.URL))

	tok, err := client.ExchangeCode(context.Background(), "client-1", "valid-code", "valid-verifier", "http://localhost/cb")
	if err != nil {
		t.Fatalf("unexpected error exchanging code: %v", err)
	}
	if tok.AccessToken != "acc-123" || tok.RefreshToken != "ref-123" {
		t.Errorf("unexpected tokens: %+v", tok)
	}

	refTok, err := client.RefreshToken(context.Background(), "client-1", "ref-123")
	if err != nil {
		t.Fatalf("unexpected error refreshing token: %v", err)
	}
	if refTok.AccessToken != "acc-456" || refTok.RefreshToken != "ref-456" {
		t.Errorf("unexpected refreshed tokens: %+v", refTok)
	}
}

func TestClient_UploadTranscodeAndContent(t *testing.T) {
	var pollCount int32
	mux := http.NewServeMux()

	var srvURL string
	mux.HandleFunc("/media/transcode/audio/uploadUrl", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"upload": map[string]any{
				"uploadUrl": srvURL + "/s3-put-target",
				"uploadId":  "up-99",
			},
		})
	})

	mux.HandleFunc("/s3-put-target", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "expected PUT", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/media/upload/up-99/transcoded", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&pollCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if count < 2 {
			// First poll: not ready yet
			_ = json.NewEncoder(w).Encode(map[string]any{
				"transcode": map[string]any{},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transcode": map[string]any{
				"transcodedSha256": "sha256-xyz",
				"transcodedInfo": map[string]any{
					"duration": 185.5,
					"fileSize": 1048576,
					"channels": "stereo",
					"format":   "aac",
				},
			},
		})
	})

	mux.HandleFunc("/content", func(w http.ResponseWriter, r *http.Request) {
		var payload CardContent
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(payload.Content.Chapters) != 1 || payload.Content.Chapters[0].Display.Icon16x16 != "yoto:#custom-icon-1" {
			http.Error(w, "missing chapter icon", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"card": map[string]any{
				"cardId": "card-abc",
				"title":  payload.Title,
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	client := NewClient(
		WithBaseURLs(srv.URL, srv.URL),
		WithPollInterval(5*time.Millisecond),
	)

	ctx := context.Background()
	up, err := client.RequestAudioUploadURL(ctx, "token-1")
	if err != nil {
		t.Fatalf("RequestAudioUploadURL failed: %v", err)
	}
	if up.UploadID != "up-99" {
		t.Fatalf("expected uploadId up-99, got %s", up.UploadID)
	}

	if err := client.UploadAudioStream(ctx, up.UploadURL, strings.NewReader("fake-audio-bytes"), "audio/mp4", 16); err != nil {
		t.Fatalf("UploadAudioStream failed: %v", err)
	}

	transcoded, err := client.WaitForTranscoding(ctx, "token-1", up.UploadID, 5)
	if err != nil {
		t.Fatalf("WaitForTranscoding failed: %v", err)
	}
	if transcoded.TranscodedSha256 != "sha256-xyz" {
		t.Fatalf("expected sha256-xyz, got %s", transcoded.TranscodedSha256)
	}

	card, err := client.CreateOrUpdateContent(ctx, "token-1", &CardContent{
		Title: "Bedtime Stories",
		Content: CardContentBody{
			Chapters: []Chapter{
				{
					Key:          "01",
					Title:        "Chapter 1",
					OverlayLabel: "1",
					Display:      DisplayFields{Icon16x16: "yoto:#custom-icon-1"},
					Tracks: []Track{
						{
							Key:      "01",
							Title:    "Chapter 1",
							TrackURL: "yoto:#" + transcoded.TranscodedSha256,
							Type:     "audio",
							Format:   transcoded.TranscodedInfo.Format,
							Duration: transcoded.TranscodedInfo.Duration,
							FileSize: transcoded.TranscodedInfo.FileSize,
							Display:  DisplayFields{Icon16x16: "yoto:#custom-icon-1"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrUpdateContent failed: %v", err)
	}
	if card.CardID != "card-abc" {
		t.Errorf("expected cardId card-abc, got %s", card.CardID)
	}
}

func TestClient_Icons(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/media/displayIcons/user/yoto", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"displayIcons": []map[string]any{
				{
					"displayIconId": "icon-1",
					"mediaId":       "media-book-1",
					"title":         "Book",
					"public":        true,
					"publicTags":    []string{"book", "story"},
					"url":           "https://example.com/icons/book.png",
				},
			},
		})
	})
	mux.HandleFunc("/media/displayIcons/user/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"displayIcons": []map[string]any{
				{
					"displayIconId": "uicon-1",
					"mediaId":       "media-custom-1",
					"public":        false,
					"url":           "https://example.com/icons/custom.png",
				},
			},
		})
	})
	mux.HandleFunc("/media/displayIcons/user/me/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"displayIcon": map[string]any{
				"displayIconId": "uicon-2",
				"mediaId":       "media-uploaded-2",
				"url":           "https://example.com/icons/uploaded.png",
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(WithBaseURLs(srv.URL, srv.URL))
	ctx := context.Background()

	pubIcons, err := client.GetPublicIcons(ctx, "tok")
	if err != nil || len(pubIcons) != 1 || pubIcons[0].MediaID != "media-book-1" {
		t.Fatalf("unexpected public icons: %+v, err=%v", pubIcons, err)
	}

	userIcons, err := client.GetUserIcons(ctx, "tok")
	if err != nil || len(userIcons) != 1 || userIcons[0].MediaID != "media-custom-1" {
		t.Fatalf("unexpected user icons: %+v, err=%v", userIcons, err)
	}

	uploaded, err := client.UploadCustomIcon(ctx, "tok", strings.NewReader("png-bytes"), "hero.png", true)
	if err != nil || uploaded.MediaID != "media-uploaded-2" {
		t.Fatalf("unexpected uploaded icon: %+v, err=%v", uploaded, err)
	}
}
