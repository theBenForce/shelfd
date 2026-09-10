package events

import (
	"sync"
	"testing"
	"time"
)

func TestHub_SubscribeAndBroadcast(t *testing.T) {
	hub := NewHub()

	ch1, unsub1 := hub.Subscribe()
	defer unsub1()

	ch2, unsub2 := hub.Subscribe()
	defer unsub2()

	testEvt := Event{
		Type: EventBookAdded,
		Data: map[string]string{"title": "Dune"},
	}

	hub.Broadcast(testEvt)

	select {
	case evt := <-ch1:
		if evt.Type != EventBookAdded {
			t.Fatalf("expected EventBookAdded, got %s", evt.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event on ch1")
	}

	select {
	case evt := <-ch2:
		if evt.Type != EventBookAdded {
			t.Fatalf("expected EventBookAdded, got %s", evt.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event on ch2")
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	hub := NewHub()

	ch, unsub := hub.Subscribe()
	unsub()

	hub.Broadcast(Event{
		Type: EventQueueStatus,
		Data: "status",
	})

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed after unsubscribe")
		}
	default:
		// channel closed or empty
	}
}

func TestHub_SlowSubscriberDoesNotBlock(t *testing.T) {
	hub := NewHub()

	// Fill subscriber channel buffer
	_, unsub := hub.Subscribe()
	defer unsub()

	// Fill buffer completely
	for i := 0; i < 64; i++ {
		hub.Broadcast(Event{Type: "ping", Data: i})
	}

	// Broadcasting should not block even if ch is full
	done := make(chan bool)
	go func() {
		hub.Broadcast(Event{Type: "overflow", Data: "test"})
		done <- true
	}()

	select {
	case <-done:
		// Success: broadcast did not block
	case <-time.After(500 * time.Millisecond):
		t.Fatal("broadcast blocked on full subscriber channel")
	}
}

func TestHub_ConcurrentBroadcastAndSubscribe(t *testing.T) {
	hub := NewHub()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, unsub := hub.Subscribe()
			defer unsub()
			select {
			case <-ch:
			case <-time.After(100 * time.Millisecond):
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hub.Broadcast(Event{Type: EventScanStatus, Data: "active"})
		}()
	}

	wg.Wait()
}
