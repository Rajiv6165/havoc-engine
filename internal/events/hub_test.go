package events

import (
	"testing"
	"time"
)

func TestHub_PubSub(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	ch1, unsub1 := hub.Subscribe(10)
	defer unsub1()

	ch2, unsub2 := hub.Subscribe(10)
	defer unsub2()

	if count := hub.SubscriberCount(); count != 2 {
		t.Fatalf("expected 2 subscribers, got %d", count)
	}

	event := Event{
		Type:    EventExperimentStarted,
		Level:   "info",
		Message: "test experiment started",
		Service: "payment-service",
	}

	hub.Publish(event)

	select {
	case e := <-ch1:
		if e.Message != event.Message || e.Service != "payment-service" {
			t.Errorf("ch1 received unexpected event: %+v", e)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ch1 timed out waiting for event")
	}

	select {
	case e := <-ch2:
		if e.Message != event.Message {
			t.Errorf("ch2 received unexpected event: %+v", e)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ch2 timed out waiting for event")
	}
}

func TestHub_SlowClientDropped(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	// Slow client with a buffer size of 1
	slowCh, slowUnsub := hub.Subscribe(1)
	defer slowUnsub()

	// Fast client with a buffer size of 10
	fastCh, fastUnsub := hub.Subscribe(10)
	defer fastUnsub()

	// 1st publish fills the buffer of the slow client
	hub.Publish(Event{Message: "msg 1"})

	// 2nd publish should be non-blocking.
	// Since slowCh is full, slow subscriber should be dropped rather than blocking the hub.
	done := make(chan struct{})
	go func() {
		hub.Publish(Event{Message: "msg 2"})
		close(done)
	}()

	select {
	case <-done:
		// Non-blocking publish succeeded
	case <-time.After(1 * time.Second):
		t.Fatal("hub.Publish blocked on slow client!")
	}

	// Verify slow client channel was closed and removed from subscribers
	if count := hub.SubscriberCount(); count != 1 {
		t.Errorf("expected 1 subscriber remaining after dropping slow client, got %d", count)
	}

	// Read first message from slowCh
	<-slowCh
	// slowCh should now be closed
	_, ok := <-slowCh
	if ok {
		t.Error("expected slowCh to be closed after being dropped")
	}

	// Verify fast client received both messages
	m1 := <-fastCh
	if m1.Message != "msg 1" {
		t.Errorf("expected msg 1, got %s", m1.Message)
	}
	m2 := <-fastCh
	if m2.Message != "msg 2" {
		t.Errorf("expected msg 2, got %s", m2.Message)
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	ch, unsub := hub.Subscribe(5)
	if hub.SubscriberCount() != 1 {
		t.Fatalf("expected 1 subscriber, got %d", hub.SubscriberCount())
	}

	unsub()

	if hub.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers after unsub, got %d", hub.SubscriberCount())
	}

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsub")
	}
}
