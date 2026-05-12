package notify

import "sync"

// Hub is a fanout for live notification events. Subscribers receive every
// successfully-fired notification. Single-process only — multi-instance fanout
// would need a pub/sub layer underneath, but Northstar is single-server.
type Hub struct {
	mu   sync.Mutex
	subs map[chan PreparedNotification]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: map[chan PreparedNotification]struct{}{}}
}

// Subscribe returns a buffered channel + an unsubscribe function. Callers must
// call unsubscribe when done (defer is the obvious pattern). The channel is
// closed by unsubscribe; further publishes are no-ops for this subscriber.
func (h *Hub) Subscribe() (<-chan PreparedNotification, func()) {
	ch := make(chan PreparedNotification, 16)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

// Publish delivers to every subscriber. Drops on a subscriber's full buffer
// rather than blocking — that subscriber is a stuck device and we don't want
// to back up the sender for everyone else.
func (h *Hub) Publish(n PreparedNotification) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- n:
		default:
			// subscriber is slow; drop this event for them
		}
	}
}

// SubscriberCount is for diagnostics/tests.
func (h *Hub) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
