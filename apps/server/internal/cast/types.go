package cast

import (
	"context"
	"time"
)

const (
	DefaultMediaReceiverAppID = "CC1AD845"

	NamespaceConnection = "urn:x-cast:com.google.cast.tp.connection"
	NamespaceHeartbeat  = "urn:x-cast:com.google.cast.tp.heartbeat"
	NamespaceReceiver   = "urn:x-cast:com.google.cast.receiver"
	NamespaceMedia      = "urn:x-cast:com.google.cast.media"

	PlatformReceiverID = "receiver-0"
	DefaultSenderID    = "sender-shelfd"
)

type SessionPhase string

const (
	PhaseIdle       SessionPhase = "idle"
	PhaseConnecting SessionPhase = "connecting"
	PhaseLaunching  SessionPhase = "launching"
	PhaseLoading    SessionPhase = "loading"
	PhasePlaying    SessionPhase = "playing"
	PhasePaused     SessionPhase = "paused"
	PhaseBuffering  SessionPhase = "buffering"
	PhaseError      SessionPhase = "error"
)

type Device struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Model    string `json:"model"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	HasVideo bool   `json:"has_video"`
	HasAudio bool   `json:"has_audio"`
	IsStatic bool   `json:"is_static,omitempty"`
}

type QueueTrackOffset struct {
	ItemID      int     `json:"item_id"`
	AudioSrc    string  `json:"audio_src"`
	GlobalBegin float64 `json:"global_begin"`
	GlobalEnd   float64 `json:"global_end"`
}

type SessionSnapshot struct {
	SessionID       string       `json:"session_id"`
	UserID          string       `json:"user_id"`
	Device          Device       `json:"device"`
	BookID          string       `json:"book_id"`
	FileID          string       `json:"file_id,omitempty"`
	Phase           SessionPhase `json:"phase"`
	PositionSeconds float64      `json:"position_seconds"`
	DurationSeconds float64      `json:"duration_seconds"`
	Speed           float64      `json:"speed"`
	Volume          float64      `json:"volume"`
	Muted           bool         `json:"muted"`
	CaptionsEnabled bool         `json:"captions_enabled"`
	ErrorMessage    string       `json:"error_message,omitempty"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

type StartSessionRequest struct {
	UserID          string  `json:"-"`
	Username        string  `json:"-"`
	DeviceID        string  `json:"device_id"`
	BookID          string  `json:"book_id"`
	FileID          string  `json:"file_id,omitempty"`
	PositionSeconds float64 `json:"position_seconds"`
	Speed           float64 `json:"speed"`
	CaptionsEnabled bool    `json:"captions_enabled"`
}

type ControlCommand struct {
	Action          string   `json:"action"`
	PositionSeconds *float64 `json:"position_seconds,omitempty"`
	Speed           *float64 `json:"speed,omitempty"`
	Volume          *float64 `json:"volume,omitempty"`
	Muted           *bool    `json:"muted,omitempty"`
	CaptionsEnabled *bool    `json:"captions_enabled,omitempty"`
}

type MediaTrack struct {
	TrackID          int    `json:"trackId"`
	Type             string `json:"type"`
	TrackContentID   string `json:"trackContentId"`
	TrackContentType string `json:"trackContentType"`
	Name             string `json:"name,omitempty"`
	Subtype          string `json:"subtype,omitempty"`
	Language         string `json:"language,omitempty"`
}

type MediaImage struct {
	URL string `json:"url"`
}

type MediaMetadata struct {
	MetadataType int          `json:"metadataType"`
	Title        string       `json:"title,omitempty"`
	Subtitle     string       `json:"subtitle,omitempty"`
	Artist       string       `json:"artist,omitempty"`
	Images       []MediaImage `json:"images,omitempty"`
}

type MediaInformation struct {
	ContentID   string        `json:"contentId"`
	ContentType string        `json:"contentType"`
	StreamType  string        `json:"streamType"`
	Duration    float64       `json:"duration,omitempty"`
	Metadata    MediaMetadata `json:"metadata"`
	Tracks      []MediaTrack  `json:"tracks,omitempty"`
}

type QueueItem struct {
	ItemID         int              `json:"itemId,omitempty"`
	Media          MediaInformation `json:"media"`
	Autoplay       bool             `json:"autoplay"`
	StartTime      float64          `json:"startTime,omitempty"`
	ActiveTrackIDs []int            `json:"activeTrackIds,omitempty"`
}

type Manager interface {
	ListDevices(ctx context.Context, refresh bool) ([]Device, error)
	CurrentSession() *SessionSnapshot
	StartSession(ctx context.Context, req StartSessionRequest) (*SessionSnapshot, error)
	Control(ctx context.Context, cmd ControlCommand) (*SessionSnapshot, error)
	StopSession(ctx context.Context) error
	Close() error
}
