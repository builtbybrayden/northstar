package notify

import (
	"sync"
	"testing"
	"time"
)

func TestHub_FanoutToMultipleSubscribers(t *testing.T) {
	h := NewHub()

	chA, unsubA := h.Subscribe()
	chB, unsubB := h.Subscribe()
	defer unsubA()
	defer unsubB()

	if got := h.SubscriberCount(); got != 2 {
		t.Fatalf("subscriber count = %d, want 2", got)
	}

	n := PreparedNotification{ID: "x1", Category: CatPurchase, Title: "test"}
	h.Publish(n)

	for _, ch := range []<-chan PreparedNotification{chA, chB} {
		select {
		case got := <-ch:
			if got.ID != "x1" {
				t.Errorf("unexpected id: %s", got.ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("subscriber did not receive event")
		}
	}
}

func TestHub_UnsubscribeStopsDelivery(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe()
	unsub()
	if got := h.SubscriberCount(); got != 0 {
		t.Fatalf("count after unsub = %d, want 0", got)
	}
	// publish goes nowhere
	h.Publish(PreparedNotification{ID: "x"})
	// channel should be closed → recv yields zero value immediately
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel closed after unsub")
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("unsub did not close channel")
	}
}

func TestHub_DropsOnFullBuffer(t *testing.T) {
	h := NewHub()
	_, unsub := h.Subscribe()
	defer unsub()
	// Publish many — buffer is 16; the rest should drop, not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			h.Publish(PreparedNotification{ID: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Publish appears to be blocking on full buffer")
	}
}

func TestHub_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, unsub := h.Subscribe()
			h.Publish(PreparedNotification{ID: "x"})
			unsub()
		}()
	}
	wg.Wait()
	if got := h.SubscriberCount(); got != 0 {
		t.Fatalf("subscriber count after churn = %d, want 0", got)
	}
}
