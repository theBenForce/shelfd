package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/cast"
	"github.com/shelfd/shelfd/internal/cast/casttest"
	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/repository"
)

func setupCastAPITest(t *testing.T, verifyHTTP bool) (*testFixture, *httptest.Server, *casttest.Receiver, *casttest.Receiver, cast.Manager, *events.Hub) {
	t.Helper()
	f := setupAPITest(t)

	tvRec, err := casttest.StartReceiver(verifyHTTP)
	if err != nil {
		t.Fatalf("start tv receiver: %v", err)
	}
	spkRec, err := casttest.StartReceiver(verifyHTTP)
	if err != nil {
		tvRec.Close()
		t.Fatalf("start speaker receiver: %v", err)
	}

	hub := events.NewHub()
	var srv *httptest.Server
	var mgr cast.Manager

	// Create HTTP test server first so LANBaseURL points directly to it.
	muxHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.handler.ServeHTTP(w, r)
	})
	srv = httptest.NewServer(muxHandler)

	mgr = cast.NewManager(cast.ManagerConfig{
		Repo:       f.repo,
		Hub:        hub,
		LibraryDir: f.libDir,
		DataDir:    f.dataDir,
		LANBaseURL: srv.URL,
		StaticDevices: []cast.Device{
			{ID: "tv-1", Name: "Living Room TV", Model: "Chromecast", Host: tvRec.Host, Port: tvRec.Port, HasVideo: true, HasAudio: true, IsStatic: true},
			{ID: "spk-1", Name: "Bedroom Speaker", Model: "Nest Audio", Host: spkRec.Host, Port: spkRec.Port, HasVideo: false, HasAudio: true, IsStatic: true},
		},
		PollInterval:  150 * time.Millisecond,
		ProgressEvery: 200 * time.Millisecond,
		TokenMinter: func(userID, username, bookID string, ttl time.Duration) (string, error) {
			tok, _, err := api.GenerateCastMediaJWT(userID, username, bookID, f.jwtSecret, ttl)
			return tok, err
		},
	})

	f.handler = api.NewRouter(api.RouterConfig{
		Repo:       f.repo,
		CastManager: mgr,
		Hub:        hub,
		DataDir:    f.dataDir,
		LibraryDir: f.libDir,
		JWTSecret:  f.jwtSecret,
	})

	return f, srv, tvRec, spkRec, mgr, hub
}

func TestCastAPI_EndToEndAudioReadalongMultiTrackAndProgress(t *testing.T) {
	f, srv, tvRec, spkRec, mgr, hub := setupCastAPITest(t, true)
	defer mgr.Close()
	defer spkRec.Close()
	defer tvRec.Close()
	defer srv.Close()
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	user, _ := f.repo.GetUserByUsername(context.Background(), "admin")

	// Seed a multi-track SMIL EPUB with cover art.
	epubBytes := buildLargeAudioEPUB(t, bytes.Repeat([]byte{0x49, 0x44, 0x33}, 512))
	bookRel := filepath.Join("Test Author", "Cast Readalong", "Cast Readalong.epub")
	bookFull := filepath.Join(f.libDir, bookRel)
	if err := os.MkdirAll(filepath.Dir(bookFull), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(bookFull, epubBytes, 0644); err != nil {
		t.Fatalf("write epub: %v", err)
	}
	coverRel := filepath.Join("Test Author", "Cast Readalong", "cover.jpg")
	if err := os.WriteFile(filepath.Join(f.libDir, coverRel), bytes.Repeat([]byte{0xFF, 0xD8}, 64), 0644); err != nil {
		t.Fatalf("write cover: %v", err)
	}

	dur := 9.0
	book := &repository.Book{
		Title:           "Cast Readalong",
		FilePath:        bookRel,
		CoverPath:       &coverRel,
		BookType:        "ebook",
		DurationSeconds: &dur,
	}
	if err := f.repo.CreateBook(context.Background(), book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	// Subscribe to SSE hub to verify cast_status broadcasts.
	evCh, unsub := hub.Subscribe()
	defer unsub()

	// 1. List devices.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cast/devices?refresh=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/cast/devices expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var devResp struct {
		Devices []cast.Device `json:"devices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &devResp); err != nil || len(devResp.Devices) < 2 {
		t.Fatalf("expected >= 2 static devices, got %+v (%v)", devResp, err)
	}

	// 2. Start multi-track SMIL Cast session on Living Room TV with captions enabled.
	startBody, _ := json.Marshal(map[string]any{
		"device_id":        "tv-1",
		"book_id":          book.ID,
		"position_seconds": 1.0,
		"speed":            1.25,
		"captions_enabled": true,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cast/session", bytes.NewReader(startBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/cast/session expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify real CastV2 QUEUE_LOAD and HTTP 206 media pull on loopback server.
	cmds, contentIDs, fetched, activeTracks := tvRec.Snapshot()
	if len(contentIDs) != 2 {
		t.Fatalf("expected 2 multi-track SMIL items queued, got %d (%v)", len(contentIDs), contentIDs)
	}
	if len(activeTracks) != 1 || activeTracks[0] != 1 {
		t.Fatalf("expected captions activeTrackIds [1] on video TV, got %v", activeTracks)
	}
	if len(fetched) == 0 {
		t.Fatalf("expected loopback receiver to perform HTTP range fetch on loaded contentId")
	}
	for u, code := range fetched {
		if code != http.StatusOK && code != http.StatusPartialContent {
			t.Fatalf("receiver HTTP fetch %s returned unexpected status %d", u, code)
		}
	}
	_ = cmds

	// 3. Cross-track global seek to 6.0s (item 2 local 2.0s) and pause.
	seekBody, _ := json.Marshal(map[string]any{"action": "seek", "position_seconds": 6.0})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cast/session/control", bytes.NewReader(seekBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seek control failed: %d %s", rec.Code, rec.Body.String())
	}
	var ctrlResp struct {
		Session cast.SessionSnapshot `json:"session"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ctrlResp)
	if ctrlResp.Session.PositionSeconds < 5.9 {
		t.Fatalf("expected global position >= 5.9 after cross-track seek, got %f", ctrlResp.Session.PositionSeconds)
	}

	// 4. Verify SSE cast_status event was broadcast.
	sawCastStatus := false
	deadline := time.After(1 * time.Second)
	for !sawCastStatus {
		select {
		case evt := <-evCh:
			if evt.Type == events.EventCastStatus {
				sawCastStatus = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for SSE cast_status event")
		}
	}

	// 5. Stop session and verify server-side SQLite audiobook progress persisted.
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/cast/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/cast/session expected 200, got %d", rec.Code)
	}

	prog, err := f.repo.GetAudiobookProgress(context.Background(), book.ID, user.ID)
	if err != nil || prog == nil || prog.PositionSeconds < 5.9 {
		t.Fatalf("expected persisted SQLite progress >= 5.9, got %+v (%v)", prog, err)
	}

	// 6. Verify scoped cast:media token cannot control /api/v1/cast/session (403 Forbidden).
	castTok, _, _ := api.GenerateCastMediaJWT(user.ID, user.Username, book.ID, f.jwtSecret, time.Hour)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cast/session?token="+castTok, bytes.NewReader(startBody))
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when using cast:media token on /api/v1/cast/session, got %d", rec.Code)
	}
}

func BenchmarkCastControlRoundTrip(b *testing.B) {
	rec, err := casttest.StartReceiver(false)
	if err != nil {
		b.Fatalf("StartReceiver: %v", err)
	}
	defer rec.Close()

	ctx := context.Background()
	client, err := cast.DialClient(ctx, rec.Host, rec.Port, cast.ClientCallbacks{})
	if err != nil {
		b.Fatalf("DialClient: %v", err)
	}
	defer client.Close()

	if _, err := client.Launch(ctx, cast.DefaultMediaReceiverAppID); err != nil {
		b.Fatalf("Launch: %v", err)
	}
	if _, err := client.Load(ctx, cast.MediaInformation{
		ContentID:   "http://127.0.0.1:8080/stream",
		ContentType: "audio/mpeg",
		StreamType:  "BUFFERED",
	}, 0, nil); err != nil {
		b.Fatalf("Load: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := client.Seek(ctx, float64(i%60)); err != nil {
			b.Fatalf("Seek: %v", err)
		}
	}
}
