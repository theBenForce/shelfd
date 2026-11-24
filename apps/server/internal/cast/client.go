package cast

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type ReceiverStatus struct {
	SessionID   string
	TransportID string
	AppID       string
	Volume      float64
	Muted       bool
}

type MediaStatus struct {
	MediaSessionID int
	PlayerState    string
	IdleReason     string
	CurrentTime    float64
	Duration       float64
	PlaybackRate   float64
	CurrentItemID  int
	ActiveTrackIDs []int
}

type ClientCallbacks struct {
	OnReceiverStatus func(ReceiverStatus)
	OnMediaStatus    func(MediaStatus)
	OnDisconnect     func(error)
}

type Client struct {
	host      string
	port      int
	senderID  string
	callbacks ClientCallbacks

	conn    net.Conn
	writeMu sync.Mutex

	stateMu     sync.RWMutex
	transportID string
	sessionID   string
	mediaSessID int
	closed      bool

	reqSeq  atomic.Int64
	pendMu  sync.Mutex
	pending map[int64]chan map[string]any

	stopCh   chan struct{}
	stopOnce sync.Once
	lastPong atomic.Int64
}

func DialClient(ctx context.Context, host string, port int, cb ClientCallbacks) (*Client, error) {
	if port <= 0 {
		port = 8009
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	rawConn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		return nil, fmt.Errorf("dial cast %s:%d: %w", host, port, err)
	}
	tlsConn := tls.Client(rawConn, &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // Cast devices present device-signed peer certificates on LAN port 8009.
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("tls handshake %s:%d: %w", host, port, err)
	}

	c := &Client{
		host:      host,
		port:      port,
		senderID:  DefaultSenderID,
		callbacks: cb,
		conn:      tlsConn,
		pending:   make(map[int64]chan map[string]any),
		stopCh:    make(chan struct{}),
	}
	c.lastPong.Store(time.Now().UnixNano())

	go c.readLoop()
	go c.heartbeatLoop()

	if err := c.sendJSON(PlatformReceiverID, NamespaceConnection, map[string]any{"type": "CONNECT"}); err != nil {
		c.Close()
		return nil, fmt.Errorf("platform connect: %w", err)
	}
	return c, nil
}

func (c *Client) Close() error {
	var err error
	c.stopOnce.Do(func() {
		c.stateMu.Lock()
		c.closed = true
		tid := c.transportID
		c.stateMu.Unlock()

		close(c.stopCh)
		if tid != "" {
			_ = c.sendJSON(tid, NamespaceConnection, map[string]any{"type": "CLOSE"})
		}
		_ = c.sendJSON(PlatformReceiverID, NamespaceConnection, map[string]any{"type": "CLOSE"})
		err = c.conn.Close()

		c.pendMu.Lock()
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
		c.pendMu.Unlock()
	})
	return err
}

func (c *Client) Launch(ctx context.Context, appID string) (ReceiverStatus, error) {
	if appID == "" {
		appID = DefaultMediaReceiverAppID
	}
	resp, err := c.request(ctx, PlatformReceiverID, NamespaceReceiver, map[string]any{
		"type":  "LAUNCH",
		"appId": appID,
	})
	if err != nil {
		return ReceiverStatus{}, err
	}
	rs := parseReceiverStatus(resp)
	if rs.TransportID == "" {
		return ReceiverStatus{}, errors.New("cast receiver launched without transportId")
	}

	c.stateMu.Lock()
	c.transportID = rs.TransportID
	c.sessionID = rs.SessionID
	c.stateMu.Unlock()

	if err := c.sendJSON(rs.TransportID, NamespaceConnection, map[string]any{"type": "CONNECT"}); err != nil {
		return ReceiverStatus{}, fmt.Errorf("connect transport %s: %w", rs.TransportID, err)
	}
	return rs, nil
}

func (c *Client) Load(ctx context.Context, media MediaInformation, currentTime float64, activeTrackIDs []int) (MediaStatus, error) {
	tid := c.getTransportID()
	if tid == "" {
		return MediaStatus{}, errors.New("no active receiver transport")
	}
	payload := map[string]any{
		"type":        "LOAD",
		"autoplay":    true,
		"currentTime": currentTime,
		"media":       media,
	}
	if len(activeTrackIDs) > 0 {
		payload["activeTrackIds"] = activeTrackIDs
	}
	resp, err := c.request(ctx, tid, NamespaceMedia, payload)
	if err != nil {
		return MediaStatus{}, err
	}
	ms := parseMediaStatus(resp)
	c.setMediaSessionID(ms.MediaSessionID)
	return ms, nil
}

func (c *Client) QueueLoad(ctx context.Context, items []QueueItem, startIndex int) (MediaStatus, error) {
	tid := c.getTransportID()
	if tid == "" {
		return MediaStatus{}, errors.New("no active receiver transport")
	}
	payload := map[string]any{
		"type":       "QUEUE_LOAD",
		"items":      items,
		"startIndex": startIndex,
		"repeatMode": "REPEAT_OFF",
	}
	resp, err := c.request(ctx, tid, NamespaceMedia, payload)
	if err != nil {
		return MediaStatus{}, err
	}
	ms := parseMediaStatus(resp)
	c.setMediaSessionID(ms.MediaSessionID)
	return ms, nil
}

func (c *Client) Play(ctx context.Context) (MediaStatus, error) {
	return c.mediaCommand(ctx, map[string]any{"type": "PLAY"})
}

func (c *Client) Pause(ctx context.Context) (MediaStatus, error) {
	return c.mediaCommand(ctx, map[string]any{"type": "PAUSE"})
}

func (c *Client) Stop(ctx context.Context) error {
	tid := c.getTransportID()
	if tid == "" {
		return nil
	}
	if c.getMediaSessionID() > 0 {
		_, _ = c.mediaCommand(ctx, map[string]any{"type": "STOP"})
	}
	c.stateMu.RLock()
	sid := c.sessionID
	c.stateMu.RUnlock()
	if sid != "" {
		_, _ = c.request(ctx, PlatformReceiverID, NamespaceReceiver, map[string]any{
			"type":      "STOP",
			"sessionId": sid,
		})
	}
	return nil
}

func (c *Client) Seek(ctx context.Context, positionSeconds float64) (MediaStatus, error) {
	return c.mediaCommand(ctx, map[string]any{
		"type":        "SEEK",
		"currentTime": positionSeconds,
	})
}

func (c *Client) QueueJump(ctx context.Context, itemID int, trackTime float64) (MediaStatus, error) {
	ms, err := c.mediaCommand(ctx, map[string]any{
		"type":          "QUEUE_UPDATE",
		"currentItemId": itemID,
	})
	if err != nil {
		return MediaStatus{}, err
	}
	if trackTime > 0 {
		return c.Seek(ctx, trackTime)
	}
	return ms, nil
}

func (c *Client) SetPlaybackRate(ctx context.Context, rate float64) (MediaStatus, error) {
	return c.mediaCommand(ctx, map[string]any{
		"type":         "SET_PLAYBACK_RATE",
		"playbackRate": rate,
	})
}

func (c *Client) EditTracksInfo(ctx context.Context, activeTrackIDs []int) (MediaStatus, error) {
	if activeTrackIDs == nil {
		activeTrackIDs = []int{}
	}
	return c.mediaCommand(ctx, map[string]any{
		"type":           "EDIT_TRACKS_INFO",
		"activeTrackIds": activeTrackIDs,
	})
}

func (c *Client) GetMediaStatus(ctx context.Context) (MediaStatus, error) {
	tid := c.getTransportID()
	if tid == "" {
		return MediaStatus{}, errors.New("no active receiver transport")
	}
	resp, err := c.request(ctx, tid, NamespaceMedia, map[string]any{"type": "GET_STATUS"})
	if err != nil {
		return MediaStatus{}, err
	}
	ms := parseMediaStatus(resp)
	if ms.MediaSessionID > 0 {
		c.setMediaSessionID(ms.MediaSessionID)
	}
	return ms, nil
}

func (c *Client) SetVolume(ctx context.Context, level *float64, muted *bool) (ReceiverStatus, error) {
	vol := map[string]any{}
	if level != nil {
		v := *level
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		vol["level"] = v
	}
	if muted != nil {
		vol["muted"] = *muted
	}
	resp, err := c.request(ctx, PlatformReceiverID, NamespaceReceiver, map[string]any{
		"type":   "SET_VOLUME",
		"volume": vol,
	})
	if err != nil {
		return ReceiverStatus{}, err
	}
	return parseReceiverStatus(resp), nil
}

func (c *Client) mediaCommand(ctx context.Context, payload map[string]any) (MediaStatus, error) {
	tid := c.getTransportID()
	msid := c.getMediaSessionID()
	if tid == "" || msid <= 0 {
		return MediaStatus{}, errors.New("no active media session")
	}
	payload["mediaSessionId"] = msid
	resp, err := c.request(ctx, tid, NamespaceMedia, payload)
	if err != nil {
		return MediaStatus{}, err
	}
	ms := parseMediaStatus(resp)
	if ms.MediaSessionID > 0 {
		c.setMediaSessionID(ms.MediaSessionID)
	}
	return ms, nil
}

func (c *Client) request(ctx context.Context, dest, ns string, payload map[string]any) (map[string]any, error) {
	reqID := c.reqSeq.Add(1)
	payload["requestId"] = reqID

	ch := make(chan map[string]any, 1)
	c.pendMu.Lock()
	c.pending[reqID] = ch
	c.pendMu.Unlock()

	defer func() {
		c.pendMu.Lock()
		delete(c.pending, reqID)
		c.pendMu.Unlock()
	}()

	if err := c.sendJSON(dest, ns, payload); err != nil {
		return nil, err
	}

	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, errors.New("cast connection closed")
		}
		if t, _ := resp["type"].(string); t == "LOAD_FAILED" || t == "INVALID_REQUEST" || t == "INVALID_PLAYER_STATE" {
			return nil, fmt.Errorf("cast receiver error: %s", t)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.stopCh:
		return nil, errors.New("cast connection closed")
	case <-timer.C:
		return nil, fmt.Errorf("cast request %d timed out", reqID)
	}
}

func (c *Client) sendJSON(dest, ns string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := &CastMessage{
		ProtocolVersion: ProtocolVersionCastV2,
		SourceID:        c.senderID,
		DestinationID:   dest,
		Namespace:       ns,
		PayloadType:     PayloadTypeString,
		PayloadUTF8:     string(raw),
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return WriteFrame(c.conn, msg)
}

func (c *Client) readLoop() {
	for {
		msg, err := ReadFrame(c.conn)
		if err != nil {
			c.stateMu.RLock()
			alreadyClosed := c.closed
			c.stateMu.RUnlock()
			_ = c.Close()
			if !alreadyClosed && c.callbacks.OnDisconnect != nil {
				c.callbacks.OnDisconnect(err)
			}
			return
		}
		if msg.PayloadType != PayloadTypeString || msg.PayloadUTF8 == "" {
			continue
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(msg.PayloadUTF8), &body); err != nil {
			continue
		}
		msgType, _ := body["type"].(string)

		switch msg.Namespace {
		case NamespaceHeartbeat:
			if msgType == "PING" {
				_ = c.sendJSON(msg.SourceID, NamespaceHeartbeat, map[string]any{"type": "PONG"})
			} else if msgType == "PONG" {
				c.lastPong.Store(time.Now().UnixNano())
			}
			continue
		case NamespaceReceiver:
			if msgType == "RECEIVER_STATUS" && c.callbacks.OnReceiverStatus != nil {
				c.callbacks.OnReceiverStatus(parseReceiverStatus(body))
			}
		case NamespaceMedia:
			if msgType == "MEDIA_STATUS" {
				ms := parseMediaStatus(body)
				if ms.MediaSessionID > 0 {
					c.setMediaSessionID(ms.MediaSessionID)
				}
				if c.callbacks.OnMediaStatus != nil {
					c.callbacks.OnMediaStatus(ms)
				}
			}
		}

		if rid, ok := body["requestId"].(float64); ok && rid > 0 {
			c.pendMu.Lock()
			if ch, exists := c.pending[int64(rid)]; exists {
				select {
				case ch <- body:
				default:
				}
			}
			c.pendMu.Unlock()
		}
	}
}

func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if time.Since(time.Unix(0, c.lastPong.Load())) > 15*time.Second {
				_ = c.Close()
				if c.callbacks.OnDisconnect != nil {
					c.callbacks.OnDisconnect(errors.New("cast heartbeat timeout"))
				}
				return
			}
			_ = c.sendJSON(PlatformReceiverID, NamespaceHeartbeat, map[string]any{"type": "PING"})
		}
	}
}

func (c *Client) getTransportID() string {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.transportID
}

func (c *Client) getMediaSessionID() int {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.mediaSessID
}

func (c *Client) setMediaSessionID(id int) {
	if id <= 0 {
		return
	}
	c.stateMu.Lock()
	c.mediaSessID = id
	c.stateMu.Unlock()
}

func parseReceiverStatus(body map[string]any) ReceiverStatus {
	var rs ReceiverStatus
	status, _ := body["status"].(map[string]any)
	if status == nil {
		return rs
	}
	if vol, _ := status["volume"].(map[string]any); vol != nil {
		if lvl, ok := vol["level"].(float64); ok {
			rs.Volume = lvl
		}
		if muted, ok := vol["muted"].(bool); ok {
			rs.Muted = muted
		}
	}
	if apps, _ := status["applications"].([]any); len(apps) > 0 {
		if app, _ := apps[0].(map[string]any); app != nil {
			rs.SessionID, _ = app["sessionId"].(string)
			rs.TransportID, _ = app["transportId"].(string)
			rs.AppID, _ = app["appId"].(string)
		}
	}
	return rs
}

func parseMediaStatus(body map[string]any) MediaStatus {
	var ms MediaStatus
	statuses, _ := body["status"].([]any)
	if len(statuses) == 0 {
		return ms
	}
	entry, _ := statuses[0].(map[string]any)
	if entry == nil {
		return ms
	}
	if v, ok := entry["mediaSessionId"].(float64); ok {
		ms.MediaSessionID = int(v)
	}
	ms.PlayerState, _ = entry["playerState"].(string)
	ms.IdleReason, _ = entry["idleReason"].(string)
	if v, ok := entry["currentTime"].(float64); ok {
		ms.CurrentTime = v
	}
	if v, ok := entry["playbackRate"].(float64); ok && v > 0 {
		ms.PlaybackRate = v
	} else {
		ms.PlaybackRate = 1.0
	}
	if v, ok := entry["currentItemId"].(float64); ok {
		ms.CurrentItemID = int(v)
	}
	if media, _ := entry["media"].(map[string]any); media != nil {
		if d, ok := media["duration"].(float64); ok {
			ms.Duration = d
		}
	}
	if ids, _ := entry["activeTrackIds"].([]any); len(ids) > 0 {
		for _, raw := range ids {
			if n, ok := raw.(float64); ok {
				ms.ActiveTrackIDs = append(ms.ActiveTrackIDs, int(n))
			}
		}
	}
	return ms
}
