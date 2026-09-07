package mcp

import (
	"sync"

	"github.com/google/uuid"
)

// Session represents an active HTTP/SSE client connection.
type Session struct {
	ID     string
	sendCh chan []byte
	doneCh chan struct{}
	once   sync.Once
}

// NewSession creates a new session with a buffered message channel.
func NewSession() *Session {
	return &Session{
		ID:     uuid.NewString(),
		sendCh: make(chan []byte, 64),
		doneCh: make(chan struct{}),
	}
}

// Send enqueues a message for delivery over the SSE stream.
func (s *Session) Send(msg []byte) bool {
	select {
	case <-s.doneCh:
		return false
	case s.sendCh <- msg:
		return true
	default:
		return false
	}
}

// Close gracefully closes the session channel.
func (s *Session) Close() {
	s.once.Do(func() {
		close(s.doneCh)
	})
}

// SessionManager manages active client sessions across the server.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionManager initializes a new SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// Create creates and tracks a new active session.
func (sm *SessionManager) Create() *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s := NewSession()
	sm.sessions[s.ID] = s
	return s
}

// Get retrieves an active session by ID.
func (sm *SessionManager) Get(id string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s, ok := sm.sessions[id]
	return s, ok
}

// Remove closes and deletes an active session.
func (sm *SessionManager) Remove(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.sessions[id]; ok {
		s.Close()
		delete(sm.sessions, id)
	}
}

// CloseAll closes all active sessions.
func (sm *SessionManager) CloseAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, s := range sm.sessions {
		s.Close()
		delete(sm.sessions, id)
	}
}
