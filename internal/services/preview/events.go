package preview

import "time"

// Event is the wire shape for the preview processing queue — a per-version
// rollup of its files' preview statuses. Kept identical between the snapshot
// fetch (GET /api/previews) and the live SSE stream so the frontend uses one
// type for both, mirroring ingest.Event.
type Event struct {
	VersionID   uint      `json:"version_id"`
	AssetID     uint      `json:"asset_id"`
	AssetName   string    `json:"asset_name"`
	Status      string    `json:"status"` // processing | queued | done (terminal)
	Total       int       `json:"total"`
	Done        int       `json:"done"`
	Failed      int       `json:"failed"`
	Processing  int       `json:"processing"`
	Queued      int       `json:"queued"`
	Unsupported int       `json:"unsupported"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// InFlight reports whether the rollup still has work in the queue.
func (e Event) InFlight() bool {
	return e.Queued > 0 || e.Processing > 0
}
