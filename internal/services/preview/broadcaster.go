package preview

import "github.com/thomas-elliott/damson/internal/broadcast"

// Broadcaster fans preview rollup events out to per-library subscribers.
type Broadcaster = broadcast.Broadcaster[Event]

func NewBroadcaster() *Broadcaster {
	return broadcast.New("preview", func(ev Event) []any {
		return []any{"version_id", ev.VersionID, "status", ev.Status}
	})
}
