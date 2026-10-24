package preview

import (
	"log/slog"
	"sync"
)

// Broadcaster fans preview rollup events out to per-library subscribers
// in-process, mirroring imports.Broadcaster. The DB is the source of truth; the
// broadcaster is only a notification channel, so a dropped event is recoverable
// by the next transition or a snapshot refresh.
type Broadcaster struct {
	mu     sync.Mutex
	subs   map[uint]map[*subscription]struct{}
	closed bool
}

type subscription struct {
	ch chan Event
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: make(map[uint]map[*subscription]struct{})}
}

// Subscribe returns a buffered channel of events for the given library and an
// unsubscribe func that callers must invoke to release the subscription.
func (b *Broadcaster) Subscribe(libraryID uint) (<-chan Event, func()) {
	s := &subscription{ch: make(chan Event, 32)}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		close(s.ch)
		return s.ch, func() {}
	}
	if b.subs[libraryID] == nil {
		b.subs[libraryID] = make(map[*subscription]struct{})
	}
	b.subs[libraryID][s] = struct{}{}
	b.mu.Unlock()
	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		set := b.subs[libraryID]
		if _, ok := set[s]; !ok {
			return // already closed by Close
		}
		delete(set, s)
		if len(set) == 0 {
			delete(b.subs, libraryID)
		}
		close(s.ch)
	}
	return s.ch, unsub
}

// Publish fans an event out to every subscriber for the library. Sends are
// non-blocking — a subscriber that can't keep up loses the event and gets a
// warn line.
func (b *Broadcaster) Publish(libraryID uint, ev Event) {
	// Sends happen under the lock, so an unsubscribe cannot close a channel
	// mid-send. They never block, so holding it is cheap.
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs[libraryID] {
		select {
		case s.ch <- ev:
		default:
			slog.Warn("preview event dropped: subscriber buffer full", "library_id", libraryID, "version_id", ev.VersionID, "status", ev.Status)
		}
	}
}

// Close ends every subscription and refuses new ones, so the SSE handlers
// return and the HTTP server can shut down without waiting out its timeout.
func (b *Broadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for _, set := range b.subs {
		for s := range set {
			close(s.ch)
		}
	}
	b.subs = map[uint]map[*subscription]struct{}{}
}
