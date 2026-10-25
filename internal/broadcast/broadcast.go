// Package broadcast fans events out to per-library subscribers in-process.
// The DB is the source of truth; a broadcaster is only a notification channel,
// so a dropped event is recoverable by the next transition or a snapshot
// refresh.
package broadcast

import (
	"log/slog"
	"sync"
)

// Broadcaster fans events of type T out to per-library subscribers.
type Broadcaster[T any] struct {
	name     string
	describe func(T) []any
	mu       sync.Mutex
	subs     map[uint]map[chan T]struct{}
	closed   bool
}

// New returns a broadcaster. name and describe label the warn line logged
// when a slow subscriber drops an event.
func New[T any](name string, describe func(T) []any) *Broadcaster[T] {
	return &Broadcaster[T]{
		name:     name,
		describe: describe,
		subs:     make(map[uint]map[chan T]struct{}),
	}
}

// Subscribe returns a buffered channel of events for the given library and an
// unsubscribe func that callers must invoke to release the subscription.
func (b *Broadcaster[T]) Subscribe(libraryID uint) (<-chan T, func()) {
	ch := make(chan T, 32)
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		close(ch)
		return ch, func() {}
	}
	if b.subs[libraryID] == nil {
		b.subs[libraryID] = make(map[chan T]struct{})
	}
	b.subs[libraryID][ch] = struct{}{}
	b.mu.Unlock()
	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		set := b.subs[libraryID]
		if _, ok := set[ch]; !ok {
			return // already closed by Close
		}
		delete(set, ch)
		if len(set) == 0 {
			delete(b.subs, libraryID)
		}
		close(ch)
	}
	return ch, unsub
}

// Publish fans an event out to every subscriber for the library. Sends are
// non-blocking — a subscriber that can't keep up loses the event and gets a
// warn line.
func (b *Broadcaster[T]) Publish(libraryID uint, ev T) {
	// Sends happen under the lock, so an unsubscribe cannot close a channel
	// mid-send. They never block, so holding it is cheap.
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[libraryID] {
		select {
		case ch <- ev:
		default:
			attrs := append([]any{"library_id", libraryID}, b.describe(ev)...)
			slog.Warn(b.name+" event dropped: subscriber buffer full", attrs...)
		}
	}
}

// Close ends every subscription and refuses new ones, so the SSE handlers
// return and the HTTP server can shut down without waiting out its timeout.
func (b *Broadcaster[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for _, set := range b.subs {
		for ch := range set {
			close(ch)
		}
	}
	b.subs = map[uint]map[chan T]struct{}{}
}
