package imports

import (
	"sync"
	"testing"
	"time"
)

func TestBroadcaster_FansOutToSubscribers(t *testing.T) {
	b := NewBroadcaster()

	chA, unA := b.Subscribe(1)
	defer unA()
	chB, unB := b.Subscribe(1)
	defer unB()

	b.Publish(1, Event{ID: 7, Stage: StageExtracting})

	for i, ch := range []<-chan Event{chA, chB} {
		select {
		case ev := <-ch:
			if ev.ID != 7 || ev.Stage != StageExtracting {
				t.Fatalf("subscriber %d: %+v", i, ev)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}
}

func TestBroadcaster_IgnoresOtherLibraries(t *testing.T) {
	b := NewBroadcaster()
	ch, unsub := b.Subscribe(1)
	defer unsub()

	b.Publish(2, Event{ID: 9})

	select {
	case ev := <-ch:
		t.Fatalf("library 1 received event for library 2: %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBroadcaster_UnsubscribeStopsDelivery(t *testing.T) {
	b := NewBroadcaster()
	ch, unsub := b.Subscribe(1)

	unsub()
	// Channel should be closed immediately.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("channel should be closed after unsubscribe")
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("channel was not closed")
	}

	// Publishing after unsubscribe is a no-op (must not panic on closed
	// channel, must not block).
	b.Publish(1, Event{ID: 1})
}

func TestBroadcaster_DropsOnFullBuffer(t *testing.T) {
	b := NewBroadcaster()
	ch, unsub := b.Subscribe(1)
	defer unsub()

	// Fill the buffered channel (cap is 32) plus several extra publishes.
	const overflow = 50
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < overflow; i++ {
			b.Publish(1, Event{ID: uint(i)})
		}
	}()
	wg.Wait()

	// Drain whatever made it through. The point is we don't hang or panic;
	// some events are dropped on a full buffer by design.
	count := 0
loop:
	for {
		select {
		case <-ch:
			count++
		case <-time.After(50 * time.Millisecond):
			break loop
		}
	}
	if count == 0 {
		t.Fatalf("expected at least one delivered event, got 0")
	}
}
