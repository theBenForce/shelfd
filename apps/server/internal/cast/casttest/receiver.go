package casttest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/shelfd/shelfd/internal/cast"
)

// Receiver is an in-process TLS + Protobuf CastV2 loopback receiver for automated verification.
type Receiver struct {
	listener net.Listener
	Host     string
	Port     int

	mu                sync.Mutex
	appID             string
	transportID       string
	sessionID         string
	mediaSessionID    int
	playerState       string
	currentTime       float64
	duration          float64
	playbackRate      float64
	volume            float64
	muted             bool
	activeTrackIDs    []int
	currentItemID     int
	loadedContentIDs  []string
	fetchedURLs       map[string]int
	receivedCommands  []string
	lastUpdate        time.Time
	verifyHTTPFetches bool
}

func StartReceiver(verifyHTTPFetches bool) (*Receiver, error) {
	cert, err := generateSelfSignedCert()
	if err != nil {
		return nil, err
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		return nil, err
	}

	addr := ln.Addr().(*net.TCPAddr)
	r := &Receiver{
		listener:          ln,
		Host:              "127.0.0.1",
		Port:              addr.Port,
		transportID:       "web-casttest-1",
		sessionID:         "sess-casttest-1",
		mediaSessionID:    1,
		playerState:       "IDLE",
		playbackRate:      1.0,
		volume:            1.0,
		fetchedURLs:       make(map[string]int),
		lastUpdate:        time.Now(),
		verifyHTTPFetches: verifyHTTPFetches,
	}

	go r.acceptLoop()
	return r, nil
}

func (r *Receiver) Close() error {
	return r.listener.Close()
}

func (r *Receiver) Snapshot() (commands []string, contentIDs []string, fetched map[string]int, activeTracks []int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	commands = append([]string(nil), r.receivedCommands...)
	contentIDs = append([]string(nil), r.loadedContentIDs...)
	fetched = make(map[string]int, len(r.fetchedURLs))
	for k, v := range r.fetchedURLs {
		fetched[k] = v
	}
	activeTracks = append([]int(nil), r.activeTrackIDs...)
	return
}

func (r *Receiver) acceptLoop() {
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			return
		}
		go r.handleConn(conn)
	}
}

func (r *Receiver) handleConn(conn net.Conn) {
	defer conn.Close()
	for {
		msg, err := cast.ReadFrame(conn)
		if err != nil {
			return
		}
		if msg.PayloadType != cast.PayloadTypeString {
			continue
		}
		var req map[string]any
		if err := json.Unmarshal([]byte(msg.PayloadUTF8), &req); err != nil {
			continue
		}
		reqType, _ := req["type"].(string)
		reqID, _ := req["requestId"].(float64)

		r.mu.Lock()
		r.receivedCommands = append(r.receivedCommands, reqType)
		r.mu.Unlock()

		switch msg.Namespace {
		case cast.NamespaceHeartbeat:
			if reqType == "PING" {
				r.send(conn, msg.DestinationID, msg.SourceID, cast.NamespaceHeartbeat, map[string]any{"type": "PONG"})
			}
		case cast.NamespaceConnection:
			if reqType == "CLOSE" {
				return
			}
		case cast.NamespaceReceiver:
			r.handleReceiverCmd(conn, msg, req, reqType, int64(reqID))
		case cast.NamespaceMedia:
			r.handleMediaCmd(conn, msg, req, reqType, int64(reqID))
		}
	}
}

func (r *Receiver) handleReceiverCmd(conn net.Conn, msg *cast.CastMessage, req map[string]any, reqType string, reqID int64) {
	r.mu.Lock()
	switch reqType {
	case "LAUNCH":
		r.appID, _ = req["appId"].(string)
	case "SET_VOLUME":
		if vol, _ := req["volume"].(map[string]any); vol != nil {
			if lvl, ok := vol["level"].(float64); ok {
				r.volume = lvl
			}
			if m, ok := vol["muted"].(bool); ok {
				r.muted = m
			}
		}
	case "STOP":
		r.playerState = "IDLE"
	}
	status := map[string]any{
		"type":      "RECEIVER_STATUS",
		"requestId": reqID,
		"status": map[string]any{
			"volume": map[string]any{"level": r.volume, "muted": r.muted},
			"applications": []map[string]any{{
				"appId":       r.appID,
				"sessionId":   r.sessionID,
				"transportId": r.transportID,
			}},
		},
	}
	r.mu.Unlock()
	r.send(conn, cast.PlatformReceiverID, msg.SourceID, cast.NamespaceReceiver, status)
}

func (r *Receiver) handleMediaCmd(conn net.Conn, msg *cast.CastMessage, req map[string]any, reqType string, reqID int64) {
	var urlsToProbe []string

	r.mu.Lock()
	r.advanceClockLocked()

	switch reqType {
	case "LOAD":
		r.playerState = "PLAYING"
		r.currentTime, _ = req["currentTime"].(float64)
		r.currentItemID = 1
		r.activeTrackIDs = parseIntSlice(req["activeTrackIds"])
		if media, _ := req["media"].(map[string]any); media != nil {
			if cid, _ := media["contentId"].(string); cid != "" {
				r.loadedContentIDs = append(r.loadedContentIDs, cid)
				urlsToProbe = append(urlsToProbe, cid)
			}
			if d, ok := media["duration"].(float64); ok && d > 0 {
				r.duration = d
			}
			if meta, _ := media["metadata"].(map[string]any); meta != nil {
				if imgs, _ := meta["images"].([]any); len(imgs) > 0 {
					if img, _ := imgs[0].(map[string]any); img != nil {
						if u, _ := img["url"].(string); u != "" {
							urlsToProbe = append(urlsToProbe, u)
						}
					}
				}
			}
			if trks, _ := media["tracks"].([]any); len(trks) > 0 && len(r.activeTrackIDs) > 0 {
				if trk, _ := trks[0].(map[string]any); trk != nil {
					if u, _ := trk["trackContentId"].(string); u != "" {
						urlsToProbe = append(urlsToProbe, u)
					}
				}
			}
		}
	case "QUEUE_LOAD":
		r.playerState = "PLAYING"
		startIdx := 0
		if si, ok := req["startIndex"].(float64); ok {
			startIdx = int(si)
		}
		if items, _ := req["items"].([]any); len(items) > 0 {
			if startIdx >= len(items) {
				startIdx = 0
			}
			for _, raw := range items {
				if item, _ := raw.(map[string]any); item != nil {
					if media, _ := item["media"].(map[string]any); media != nil {
						if cid, _ := media["contentId"].(string); cid != "" {
							r.loadedContentIDs = append(r.loadedContentIDs, cid)
						}
					}
				}
			}
			if activeItem, _ := items[startIdx].(map[string]any); activeItem != nil {
				if iid, ok := activeItem["itemId"].(float64); ok {
					r.currentItemID = int(iid)
				} else {
					r.currentItemID = startIdx + 1
				}
				r.currentTime, _ = activeItem["startTime"].(float64)
				r.activeTrackIDs = parseIntSlice(activeItem["activeTrackIds"])
				if media, _ := activeItem["media"].(map[string]any); media != nil {
					if cid, _ := media["contentId"].(string); cid != "" {
						urlsToProbe = append(urlsToProbe, cid)
					}
				}
			}
		}
	case "QUEUE_UPDATE":
		if iid, ok := req["currentItemId"].(float64); ok {
			r.currentItemID = int(iid)
			r.currentTime = 0
		}
	case "PLAY":
		r.playerState = "PLAYING"
	case "PAUSE":
		r.playerState = "PAUSED"
	case "STOP":
		r.playerState = "IDLE"
	case "SEEK":
		if ct, ok := req["currentTime"].(float64); ok {
			r.currentTime = ct
		}
	case "SET_PLAYBACK_RATE":
		if pr, ok := req["playbackRate"].(float64); ok && pr > 0 {
			r.playbackRate = pr
		}
	case "EDIT_TRACKS_INFO":
		r.activeTrackIDs = parseIntSlice(req["activeTrackIds"])
	}
	r.lastUpdate = time.Now()

	status := map[string]any{
		"type":      "MEDIA_STATUS",
		"requestId": reqID,
		"status": []map[string]any{{
			"mediaSessionId": r.mediaSessionID,
			"playerState":    r.playerState,
			"currentTime":    r.currentTime,
			"playbackRate":   r.playbackRate,
			"currentItemId":  r.currentItemID,
			"activeTrackIds": r.activeTrackIDs,
			"media":          map[string]any{"duration": r.duration},
		}},
	}
	verify := r.verifyHTTPFetches
	r.mu.Unlock()

	if verify {
		for _, u := range urlsToProbe {
			r.probeURL(u)
		}
	}
	r.send(conn, r.transportID, msg.SourceID, cast.NamespaceMedia, status)
}

func (r *Receiver) probeURL(rawURL string) {
	httpReq, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return
	}
	httpReq.Header.Set("Range", "bytes=0-63")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(httpReq)
	if err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	r.mu.Lock()
	r.fetchedURLs[rawURL] = resp.StatusCode
	r.mu.Unlock()
}

func (r *Receiver) advanceClockLocked() {
	if r.playerState == "PLAYING" {
		elapsed := time.Since(r.lastUpdate).Seconds()
		r.currentTime += elapsed * r.playbackRate
	}
}

func (r *Receiver) send(conn net.Conn, src, dst, ns string, payload map[string]any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = cast.WriteFrame(conn, &cast.CastMessage{
		ProtocolVersion: cast.ProtocolVersionCastV2,
		SourceID:        src,
		DestinationID:   dst,
		Namespace:       ns,
		PayloadType:     cast.PayloadTypeString,
		PayloadUTF8:     string(raw),
	})
}

func parseIntSlice(v any) []int {
	raw, _ := v.([]any)
	out := make([]int, 0, len(raw))
	for _, item := range raw {
		if n, ok := item.(float64); ok {
			out = append(out, int(n))
		}
	}
	return out
}

func generateSelfSignedCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate ecdsa key: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "shelfd-casttest"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create cert: %w", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}, nil
}
