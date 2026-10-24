package events

import (
	"sync"
	"time"
)

type EventType string

const (
	EventExperimentStarted EventType = "experiment_started"
	EventTargetSelected    EventType = "target_selected"
	EventPodKilled         EventType = "pod_killed"
	EventRecovering        EventType = "recovering"
	EventRecovered         EventType = "recovered"
	EventAborted           EventType = "aborted"
	EventBlocked           EventType = "blocked"
	EventCompleted         EventType = "completed"
)

// Event represents a system lifecycle or audit event streamed to clients.
type Event struct {
	ID           string                 `json:"id,omitempty"`
	ExperimentID string                 `json:"experiment_id,omitempty"`
	Type         EventType              `json:"type"`
	Level        string                 `json:"level"` // "info", "warn", "error"
	Message      string                 `json:"message"`
	Timestamp    string                 `json:"timestamp"`
	Service      string                 `json:"service,omitempty"`
	Score        *int                   `json:"score,omitempty"`
	RecoveryMs   *int                   `json:"recovery_ms,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// EventPublisher is an interface for emitting lifecycle events.
type EventPublisher interface {
	Publish(event Event)
}

// Hub manages pub/sub subscriptions for events with non-blocking delivery.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
	closed      bool
}

// NewHub creates a new in-memory Event Hub.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe registers a new subscriber channel with the given buffer size.
// Returns the receive-only channel and an unsubscribe cleanup function.
func (h *Hub) Subscribe(bufSize int) (<-chan Event, func()) {
	if bufSize <= 0 {
		bufSize = 64
	}
	ch := make(chan Event, bufSize)

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		close(ch)
		return ch, func() {}
	}

	h.subscribers[ch] = struct{}{}

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if _, exists := h.subscribers[ch]; exists {
				delete(h.subscribers, ch)
				close(ch)
			}
		})
	}

	return ch, unsubscribe
}

// Publish sends an event to all active subscribers.
// Delivery is non-blocking. If a subscriber channel is full (slow client),
// the subscriber is dropped and closed so it does not block the engine.
func (h *Hub) Publish(event Event) {
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}

	var slowSubscribers []chan Event
	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
			// Client is too slow to consume; drop subscriber
			slowSubscribers = append(slowSubscribers, ch)
		}
	}

	for _, ch := range slowSubscribers {
		delete(h.subscribers, ch)
		close(ch)
	}
}

// SubscriberCount returns the count of currently connected subscribers.
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

// Close closes all subscriber channels and marks the hub as closed.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}
	h.closed = true

	for ch := range h.subscribers {
		close(ch)
	}
	h.subscribers = make(map[chan Event]struct{})
}
