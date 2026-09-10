package events

import "sync"

// EventType identifies the nature of a real-time server event.
type EventType string

const (
	EventBookAdded   EventType = "book_added"
	EventBookUpdated EventType = "book_updated"
	EventQueueStatus EventType = "queue_status"
	EventScanStatus  EventType = "scan_status"
)

// Event represents an event payload to broadcast to connected clients.
type Event struct {
	Type EventType `json:"type"`
	Data any       `json:"data"`
}

// Hub manages subscriber channels and dispatches events asynchronously.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

// NewHub creates a new thread-safe Hub instance.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe registers a new subscriber channel and returns an unsubscription closure.
func (h *Hub) Subscribe() (chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan Event, 64)
	h.subscribers[ch] = struct{}{}

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subscribers[ch]; ok {
			delete(h.subscribers, ch)
			close(ch)
		}
	}

	return ch, unsubscribe
}

// Broadcast sends an event to all subscribers without blocking.
// Slow subscribers with full channel buffers have older unconsumed events dropped.
func (h *Hub) Broadcast(evt Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subscribers {
		select {
		case ch <- evt:
		default:
			// Non-blocking: buffer full, skip to avoid slowing down daemon
		}
	}
}
