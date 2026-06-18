package imports

import (
	"log/slog"
	"sync"
	"time"
)

// Stage values match the asset_imports.stage enum.
const (
	StageUploaded     = "uploaded"
	StageExtracting   = "extracting"
	StageDetecting    = "detecting"
	StageThumbnailing = "thumbnailing"
	StagePreviewing   = "previewing"
	StageDone         = "done"
	StageFailed       = "failed"
)

// IsTerminal reports whether a stage represents the end of the pipeline.
func IsTerminal(stage string) bool {
	return stage == StageDone || stage == StageFailed
}

// Event is the SSE wire shape — kept identical to api.importRow so the
// frontend can use a single type for snapshot fetches and live updates.
type Event struct {
	ID          uint      `json:"id"`
	AssetID     uint      `json:"asset_id"`
	AssetName   string    `json:"asset_name"`
	Stage       string    `json:"stage"`
	Error       string    `json:"error,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
	NeedsReview bool      `json:"needs_review"`
}

// Broadcaster fans events out to per-library subscribers in-process. The DB
// is the source of truth; the broadcaster is just a notification channel.
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

// Subscribe returns a buffered channel of events for the given library and
// an unsubscribe func. Callers must call the unsubscribe func to release
// the subscription.
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
// non-blocking — a subscriber that can't keep up loses the event and gets
// a warn line. The DB row is authoritative, so a dropped event is recoverable
// by the next stage transition or a refresh.
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
			slog.Warn("import event dropped: subscriber buffer full", "library_id", libraryID, "import_id", ev.ID, "stage", ev.Stage)
		}
	}
}
