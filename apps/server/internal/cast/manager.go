package cast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/repository"
)

var (
	ErrDeviceNotFound  = errors.New("cast device not found")
	ErrBookNotFound    = errors.New("book not found")
	ErrNoActiveSession = errors.New("no active cast session")
)

type TokenMinter func(userID, username, bookID string, ttl time.Duration) (string, error)

type ManagerConfig struct {
	Repo          repository.StorageEngine
	Hub           *events.Hub
	TokenMinter   TokenMinter
	LibraryDir    string
	DataDir       string
	ServerPort    int
	LANBaseURL    string
	StaticDevices []Device
	PollInterval  time.Duration
	ProgressEvery time.Duration
	Logger        *slog.Logger
}

type Service struct {
	cfg        ManagerConfig
	discoverer *Discoverer
	logger     *slog.Logger

	mu               sync.RWMutex
	devices          []Device
	client           *Client
	session          *SessionSnapshot
	queueTracks      []QueueTrackOffset
	pollCancel       context.CancelFunc
	lastProgressSave time.Time
}

func NewManager(cfg ManagerConfig) *Service {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.ProgressEvery <= 0 {
		cfg.ProgressEvery = 10 * time.Second
	}
	return &Service{
		cfg: cfg,
		discoverer: &Discoverer{
			StaticDevices: cfg.StaticDevices,
		},
		logger: cfg.Logger,
	}
}

func (s *Service) ListDevices(ctx context.Context, refresh bool) ([]Device, error) {
	s.mu.RLock()
	cached := append([]Device(nil), s.devices...)
	s.mu.RUnlock()

	if !refresh && len(cached) > 0 {
		return cached, nil
	}

	devices, err := s.discoverer.Discover(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.devices = devices
	s.mu.Unlock()

	if s.cfg.Hub != nil {
		s.cfg.Hub.Broadcast(events.Event{
			Type: events.EventCastDevices,
			Data: map[string]any{"devices": devices},
		})
	}
	return devices, nil
}

func (s *Service) CurrentSession() *SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.session == nil {
		return nil
	}
	cp := *s.session
	return &cp
}

func (s *Service) StartSession(ctx context.Context, req StartSessionRequest) (*SessionSnapshot, error) {
	if req.DeviceID == "" || req.BookID == "" {
		return nil, errors.New("device_id and book_id are required")
	}

	device, err := s.findDevice(ctx, req.DeviceID)
	if err != nil {
		return nil, err
	}

	book, err := s.cfg.Repo.GetBookByID(ctx, req.BookID)
	if errors.Is(err, repository.ErrNotFound) || book == nil {
		return nil, ErrBookNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load book: %w", err)
	}

	// Stop any existing Cast session before starting a new one.
	_ = s.StopSession(ctx)

	baseURL := s.resolveLANBaseURL(device.Host)
	token := ""
	if s.cfg.TokenMinter != nil {
		token, err = s.cfg.TokenMinter(req.UserID, req.Username, book.ID, 12*time.Hour)
		if err != nil {
			return nil, fmt.Errorf("mint cast token: %w", err)
		}
	}

	speed := req.Speed
	if speed <= 0 {
		speed = 1.0
	}
	captionsEnabled := req.CaptionsEnabled && device.HasVideo

	sessionID := uuid.NewString()
	snap := &SessionSnapshot{
		SessionID:       sessionID,
		UserID:          req.UserID,
		Device:          device,
		BookID:          book.ID,
		FileID:          req.FileID,
		Phase:           PhaseConnecting,
		PositionSeconds: req.PositionSeconds,
		Speed:           speed,
		Volume:          1.0,
		CaptionsEnabled: captionsEnabled,
		UpdatedAt:       time.Now().UTC(),
	}
	if book.DurationSeconds != nil && *book.DurationSeconds > 0 {
		snap.DurationSeconds = *book.DurationSeconds
	}

	client, err := DialClient(ctx, device.Host, device.Port, ClientCallbacks{
		OnReceiverStatus: func(rs ReceiverStatus) {
			s.handleReceiverStatus(sessionID, rs)
		},
		OnMediaStatus: func(ms MediaStatus) {
			s.handleMediaStatus(sessionID, ms)
		},
		OnDisconnect: func(dErr error) {
			s.handleDisconnect(sessionID, dErr)
		},
	})
	if err != nil {
		return nil, err
	}

	snap.Phase = PhaseLaunching
	rs, err := client.Launch(ctx, DefaultMediaReceiverAppID)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("launch default receiver: %w", err)
	}
	if rs.Volume > 0 {
		snap.Volume = rs.Volume
	}
	snap.Muted = rs.Muted

	authors, _ := s.cfg.Repo.GetBookAuthors(ctx, book.ID)
	artist := ""
	if len(authors) > 0 {
		artist = authors[0].Name
	}

	meta := MediaMetadata{
		MetadataType: 3, // MusicTrack / Audiobook generic metadata accepted by CC1AD845
		Title:        book.Title,
		Artist:       artist,
	}
	if book.CoverPath != nil && *book.CoverPath != "" {
		meta.Images = []MediaImage{{URL: appendToken(baseURL+"/api/v1/books/"+book.ID+"/cover", token)}}
	}

	tracks, totalDur := s.resolveMultiTrackSMIL(book)
	var ms MediaStatus
	snap.Phase = PhaseLoading

	if len(tracks) > 1 && req.FileID == "" {
		if totalDur > 0 {
			snap.DurationSeconds = totalDur
		}
		items := make([]QueueItem, 0, len(tracks))
		startIdx := 0
		for i, tr := range tracks {
			if req.PositionSeconds >= tr.GlobalBegin && req.PositionSeconds < tr.GlobalEnd {
				startIdx = i
			}
			itemStart := 0.0
			if i == startIdx && req.PositionSeconds > tr.GlobalBegin {
				itemStart = req.PositionSeconds - tr.GlobalBegin
			}
			media := MediaInformation{
				ContentID:   appendToken(baseURL+"/api/v1/books/"+book.ID+"/assets/"+strings.TrimPrefix(tr.AudioSrc, "/"), token),
				ContentType: detectAudioMIME(tr.AudioSrc),
				StreamType:  "BUFFERED",
				Duration:    tr.GlobalEnd - tr.GlobalBegin,
				Metadata:    meta,
			}
			var activeIDs []int
			if device.HasVideo {
				vttURL := appendToken(baseURL+"/api/v1/books/"+book.ID+"/readalong.vtt?track="+url.QueryEscape(tr.AudioSrc), token)
				media.Tracks = []MediaTrack{{
					TrackID:          1,
					Type:             "TEXT",
					TrackContentID:   vttURL,
					TrackContentType: "text/vtt",
					Name:             "Read-Along",
					Subtype:          "SUBTITLES",
					Language:         "en-US",
				}}
				if captionsEnabled {
					activeIDs = []int{1}
				}
			}
			items = append(items, QueueItem{
				ItemID:         tr.ItemID,
				Media:          media,
				Autoplay:       true,
				StartTime:      itemStart,
				ActiveTrackIDs: activeIDs,
			})
		}
		ms, err = client.QueueLoad(ctx, items, startIdx)
	} else {
		streamPath := "/api/v1/audiobooks/" + book.ID + "/stream"
		if req.FileID != "" {
			streamPath = "/api/v1/books/" + book.ID + "/files/" + req.FileID + "/stream"
		}
		media := MediaInformation{
			ContentID:   appendToken(baseURL+streamPath, token),
			ContentType: s.resolveAudioMIME(ctx, book, req.FileID),
			StreamType:  "BUFFERED",
			Duration:    snap.DurationSeconds,
			Metadata:    meta,
		}
		var activeIDs []int
		if device.HasVideo {
			vttURL := appendToken(baseURL+"/api/v1/books/"+book.ID+"/readalong.vtt", token)
			media.Tracks = []MediaTrack{{
				TrackID:          1,
				Type:             "TEXT",
				TrackContentID:   vttURL,
				TrackContentType: "text/vtt",
				Name:             "Read-Along",
				Subtype:          "SUBTITLES",
				Language:         "en-US",
			}}
			if captionsEnabled {
				activeIDs = []int{1}
			}
		}
		ms, err = client.Load(ctx, media, req.PositionSeconds, activeIDs)
	}

	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("load media on cast receiver: %w", err)
	}

	if speed != 1.0 {
		if rateStatus, rErr := client.SetPlaybackRate(ctx, speed); rErr == nil {
			ms = rateStatus
		}
	}

	pollCtx, pollCancel := context.WithCancel(context.Background())

	s.mu.Lock()
	s.client = client
	s.queueTracks = tracks
	s.pollCancel = pollCancel
	s.lastProgressSave = time.Now()
	s.applyMediaStatusLocked(snap, ms)
	s.session = snap
	out := *snap
	s.mu.Unlock()

	s.logger.Info("cast: session started",
		"device", device.Name,
		"app_id", DefaultMediaReceiverAppID,
		"book_id", book.ID,
		"session_id", sessionID,
	)

	s.broadcastSession(out)
	go s.pollLoop(pollCtx, sessionID, client)

	return &out, nil
}

func (s *Service) Control(ctx context.Context, cmd ControlCommand) (*SessionSnapshot, error) {
	s.mu.RLock()
	client := s.client
	sess := s.session
	tracks := append([]QueueTrackOffset(nil), s.queueTracks...)
	s.mu.RUnlock()

	if client == nil || sess == nil {
		return nil, ErrNoActiveSession
	}

	var (
		ms  MediaStatus
		rs  ReceiverStatus
		err error
	)

	switch strings.ToLower(strings.TrimSpace(cmd.Action)) {
	case "play":
		ms, err = client.Play(ctx)
	case "pause":
		ms, err = client.Pause(ctx)
	case "seek":
		if cmd.PositionSeconds == nil {
			return nil, errors.New("position_seconds is required for seek")
		}
		pos := *cmd.PositionSeconds
		if pos < 0 {
			pos = 0
		}
		if len(tracks) > 1 {
			target := tracks[len(tracks)-1]
			for _, tr := range tracks {
				if pos >= tr.GlobalBegin && pos < tr.GlobalEnd {
					target = tr
					break
				}
			}
			ms, err = client.QueueJump(ctx, target.ItemID, pos-target.GlobalBegin)
		} else {
			ms, err = client.Seek(ctx, pos)
		}
	case "speed":
		if cmd.Speed == nil || *cmd.Speed <= 0 {
			return nil, errors.New("positive speed is required")
		}
		ms, err = client.SetPlaybackRate(ctx, *cmd.Speed)
	case "volume":
		if cmd.Volume == nil && cmd.Muted == nil {
			return nil, errors.New("volume or muted is required")
		}
		rs, err = client.SetVolume(ctx, cmd.Volume, cmd.Muted)
	case "captions":
		if cmd.CaptionsEnabled == nil {
			return nil, errors.New("captions_enabled is required")
		}
		active := []int{}
		if *cmd.CaptionsEnabled && sess.Device.HasVideo {
			active = []int{1}
		}
		ms, err = client.EditTracksInfo(ctx, active)
	default:
		return nil, fmt.Errorf("unsupported cast action: %s", cmd.Action)
	}
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if s.session == nil {
		s.mu.Unlock()
		return nil, ErrNoActiveSession
	}
	if strings.EqualFold(cmd.Action, "volume") {
		if cmd.Volume != nil {
			s.session.Volume = *cmd.Volume
		} else if rs.Volume > 0 {
			s.session.Volume = rs.Volume
		}
		if cmd.Muted != nil {
			s.session.Muted = *cmd.Muted
		} else {
			s.session.Muted = rs.Muted
		}
		s.session.UpdatedAt = time.Now().UTC()
	} else {
		if strings.EqualFold(cmd.Action, "captions") && cmd.CaptionsEnabled != nil {
			s.session.CaptionsEnabled = *cmd.CaptionsEnabled && s.session.Device.HasVideo
		}
		s.applyMediaStatusLocked(s.session, ms)
	}
	out := *s.session
	s.mu.Unlock()

	if strings.EqualFold(cmd.Action, "pause") || strings.EqualFold(cmd.Action, "seek") || strings.EqualFold(cmd.Action, "speed") {
		s.persistProgress(ctx, out, false)
	}
	s.broadcastSession(out)
	return &out, nil
}

func (s *Service) StopSession(ctx context.Context) error {
	s.mu.Lock()
	client := s.client
	sess := s.session
	cancel := s.pollCancel
	s.client = nil
	s.session = nil
	s.queueTracks = nil
	s.pollCancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if sess != nil {
		s.persistProgress(ctx, *sess, false)
	}
	if client != nil {
		_ = client.Stop(ctx)
		_ = client.Close()
	}
	if sess != nil {
		ended := *sess
		ended.Phase = PhaseIdle
		ended.UpdatedAt = time.Now().UTC()
		s.broadcastSession(ended)
	}
	return nil
}

func (s *Service) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.StopSession(ctx)
}

func (s *Service) pollLoop(ctx context.Context, sessionID string, client *Client) {
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pollCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			ms, err := client.GetMediaStatus(pollCtx)
			cancel()
			if err != nil {
				continue
			}
			s.handleMediaStatus(sessionID, ms)
		}
	}
}

func (s *Service) handleReceiverStatus(sessionID string, rs ReceiverStatus) {
	s.mu.Lock()
	if s.session == nil || s.session.SessionID != sessionID {
		s.mu.Unlock()
		return
	}
	if rs.Volume > 0 {
		s.session.Volume = rs.Volume
	}
	s.session.Muted = rs.Muted
	s.session.UpdatedAt = time.Now().UTC()
	out := *s.session
	s.mu.Unlock()
	s.broadcastSession(out)
}

func (s *Service) handleMediaStatus(sessionID string, ms MediaStatus) {
	var (
		out        SessionSnapshot
		shouldSave bool
		completed  bool
	)

	s.mu.Lock()
	if s.session == nil || s.session.SessionID != sessionID {
		s.mu.Unlock()
		return
	}
	prevPhase := s.session.Phase
	s.applyMediaStatusLocked(s.session, ms)
	if ms.PlayerState == "IDLE" && ms.IdleReason == "FINISHED" {
		completed = true
		shouldSave = true
	} else if s.session.Phase == PhasePlaying && time.Since(s.lastProgressSave) >= s.cfg.ProgressEvery {
		shouldSave = true
		s.lastProgressSave = time.Now()
	} else if prevPhase != s.session.Phase && (s.session.Phase == PhasePaused || s.session.Phase == PhaseIdle) {
		shouldSave = true
		s.lastProgressSave = time.Now()
	}
	out = *s.session
	s.mu.Unlock()

	if shouldSave {
		s.persistProgress(context.Background(), out, completed)
	}
	s.broadcastSession(out)
}

func (s *Service) handleDisconnect(sessionID string, err error) {
	s.mu.Lock()
	if s.session == nil || s.session.SessionID != sessionID {
		s.mu.Unlock()
		return
	}
	cancel := s.pollCancel
	s.pollCancel = nil
	s.client = nil
	out := *s.session
	out.Phase = PhaseError
	if err != nil {
		out.ErrorMessage = err.Error()
	}
	out.UpdatedAt = time.Now().UTC()
	s.session = nil
	s.queueTracks = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.persistProgress(context.Background(), out, false)
	s.broadcastSession(out)
}

func (s *Service) applyMediaStatusLocked(snap *SessionSnapshot, ms MediaStatus) {
	if ms.PlaybackRate > 0 {
		snap.Speed = ms.PlaybackRate
	}
	if ms.Duration > 0 && len(s.queueTracks) <= 1 {
		snap.DurationSeconds = ms.Duration
	}
	if len(s.queueTracks) > 1 && ms.CurrentItemID > 0 {
		for _, tr := range s.queueTracks {
			if tr.ItemID == ms.CurrentItemID {
				snap.PositionSeconds = tr.GlobalBegin + ms.CurrentTime
				break
			}
		}
	} else if ms.CurrentTime >= 0 {
		snap.PositionSeconds = ms.CurrentTime
	}

	switch strings.ToUpper(ms.PlayerState) {
	case "PLAYING":
		snap.Phase = PhasePlaying
	case "PAUSED":
		snap.Phase = PhasePaused
	case "BUFFERING":
		snap.Phase = PhaseBuffering
	case "IDLE":
		snap.Phase = PhaseIdle
	}
	if len(ms.ActiveTrackIDs) > 0 {
		snap.CaptionsEnabled = true
	}
	snap.UpdatedAt = time.Now().UTC()
}

func (s *Service) persistProgress(ctx context.Context, snap SessionSnapshot, isCompleted bool) {
	if s.cfg.Repo == nil || snap.UserID == "" || snap.BookID == "" {
		return
	}
	prog := &repository.AudiobookProgress{
		UserID:          snap.UserID,
		BookID:          snap.BookID,
		PositionSeconds: snap.PositionSeconds,
		Speed:           snap.Speed,
		IsCompleted:     isCompleted,
	}
	if err := s.cfg.Repo.UpsertAudiobookProgress(ctx, prog); err != nil {
		s.logger.Warn("cast: failed to persist audiobook progress", "book_id", snap.BookID, "error", err)
	}
}

func (s *Service) broadcastSession(snap SessionSnapshot) {
	if s.cfg.Hub == nil {
		return
	}
	s.cfg.Hub.Broadcast(events.Event{
		Type: events.EventCastStatus,
		Data: snap,
	})
}

func (s *Service) findDevice(ctx context.Context, deviceID string) (Device, error) {
	devices, err := s.ListDevices(ctx, false)
	if err == nil {
		for _, d := range devices {
			if d.ID == deviceID {
				return d, nil
			}
		}
	}
	devices, err = s.ListDevices(ctx, true)
	if err != nil {
		return Device{}, err
	}
	for _, d := range devices {
		if d.ID == deviceID {
			return d, nil
		}
	}
	return Device{}, ErrDeviceNotFound
}

func (s *Service) resolveLANBaseURL(deviceHost string) string {
	if strings.TrimSpace(s.cfg.LANBaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(s.cfg.LANBaseURL), "/")
	}
	port := s.cfg.ServerPort
	if port <= 0 {
		port = 8080
	}
	if ip := detectOutboundIP(deviceHost); ip != "" {
		return fmt.Sprintf("http://%s:%d", ip, port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func detectOutboundIP(targetHost string) string {
	if targetHost == "127.0.0.1" || targetHost == "localhost" || targetHost == "::1" {
		return "127.0.0.1"
	}
	conn, err := net.DialTimeout("udp4", net.JoinHostPort(targetHost, "8009"), 500*time.Millisecond)
	if err != nil {
		return ""
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP != nil {
		return addr.IP.String()
	}
	return ""
}

func (s *Service) resolveMultiTrackSMIL(book *repository.Book) ([]QueueTrackOffset, float64) {
	if book == nil || !strings.HasSuffix(strings.ToLower(book.FilePath), ".epub") {
		return nil, 0
	}
	epubPath := s.resolveFilePath(book.FilePath)
	if epubPath == "" {
		return nil, 0
	}
	reader, err := epub.Open(epubPath)
	if err != nil {
		return nil, 0
	}
	defer reader.Close()

	manifest, err := reader.ExtractMediaOverlays()
	if err != nil || manifest == nil || len(manifest.Segments) == 0 {
		return nil, 0
	}

	var tracks []QueueTrackOffset
	indexBySrc := make(map[string]int)
	maxEnd := 0.0
	for _, seg := range manifest.Segments {
		if seg.AudioSrc == "" {
			continue
		}
		if seg.GlobalEnd > maxEnd {
			maxEnd = seg.GlobalEnd
		}
		idx, ok := indexBySrc[seg.AudioSrc]
		if !ok {
			indexBySrc[seg.AudioSrc] = len(tracks)
			tracks = append(tracks, QueueTrackOffset{
				ItemID:      len(tracks) + 1,
				AudioSrc:    seg.AudioSrc,
				GlobalBegin: seg.GlobalBegin,
				GlobalEnd:   seg.GlobalEnd,
			})
			continue
		}
		if seg.GlobalBegin < tracks[idx].GlobalBegin {
			tracks[idx].GlobalBegin = seg.GlobalBegin
		}
		if seg.GlobalEnd > tracks[idx].GlobalEnd {
			tracks[idx].GlobalEnd = seg.GlobalEnd
		}
	}
	return tracks, maxEnd
}

func (s *Service) resolveAudioMIME(ctx context.Context, book *repository.Book, fileID string) string {
	if fileID != "" && s.cfg.Repo != nil {
		if bf, err := s.cfg.Repo.GetBookFileByID(ctx, fileID); err == nil && bf != nil {
			if bf.MimeType != nil && *bf.MimeType != "" {
				return *bf.MimeType
			}
			return detectAudioMIME(bf.FilePath)
		}
	}
	if s.cfg.Repo != nil {
		if files, err := s.cfg.Repo.GetBookFilesByBookID(ctx, book.ID); err == nil {
			for _, f := range files {
				if f.FileType == "audiobook" || f.FileType == "audio" {
					if f.MimeType != nil && *f.MimeType != "" {
						return *f.MimeType
					}
					return detectAudioMIME(f.FilePath)
				}
			}
		}
	}
	return detectAudioMIME(book.FilePath)
}

func (s *Service) resolveFilePath(relOrAbs string) string {
	if filepath.IsAbs(relOrAbs) {
		if _, err := os.Stat(relOrAbs); err == nil {
			return relOrAbs
		}
	}
	if s.cfg.LibraryDir != "" {
		p := filepath.Join(s.cfg.LibraryDir, relOrAbs)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if s.cfg.DataDir != "" {
		p := filepath.Join(s.cfg.DataDir, relOrAbs)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func detectAudioMIME(filePath string) string {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".m4b", ".m4a":
		return "audio/mp4"
	case ".ogg", ".oga":
		return "audio/ogg"
	case ".opus":
		return "audio/opus"
	case ".flac":
		return "audio/flac"
	case ".wav":
		return "audio/wav"
	default:
		return "audio/mpeg"
	}
}

func appendToken(rawURL, token string) string {
	if token == "" {
		return rawURL
	}
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + "token=" + url.QueryEscape(token)
}
