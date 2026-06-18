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
	mu   sync.Mutex
	subs map[uint]map[*subscription]struct{}
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
	if b.subs[libraryID] == nil {
		b.subs[libraryID] = make(map[*subscription]struct{})
	}
	b.subs[libraryID][s] = struct{}{}
	b.mu.Unlock()
	unsub := func() {
		b.mu.Lock()
		if set, ok := b.subs[libraryID]; ok {
			delete(set, s)
			if len(set) == 0 {
				delete(b.subs, libraryID)
			}
		}
		b.mu.Unlock()
		close(s.ch)
	}
	return s.ch, unsub
}

// Publish fans an event out to every subscriber for the library. Sends are
// non-blocking — a subscriber that can't keep up loses the event and gets a
// warn line.
func (b *Broadcaster) Publish(libraryID uint, ev Event) {
	b.mu.Lock()
	set := b.subs[libraryID]
	targets := make([]*subscription, 0, len(set))
	for s := range set {
		targets = append(targets, s)
	}
	b.mu.Unlock()
	for _, s := range targets {
		select {
		case s.ch <- ev:
		default:
			slog.Warn("preview event dropped: subscriber buffer full", "library_id", libraryID, "version_id", ev.VersionID, "status", ev.Status)
		}
	}
}
