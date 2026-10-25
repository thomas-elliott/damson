package imports

import (
	"time"

	"github.com/thomas-elliott/damson/internal/broadcast"
	"github.com/thomas-elliott/damson/internal/importstage"
)

// Stage values match the asset_imports.stage enum. Defined in
// internal/importstage so the CLI can share them; aliased here because this is
// where the pipeline reads them.
const (
	StageUploaded     = importstage.Uploaded
	StageExtracting   = importstage.Extracting
	StageDetecting    = importstage.Detecting
	StageThumbnailing = importstage.Thumbnailing
	StageEnriching    = importstage.Enriching
	StageDone         = importstage.Done
	StageFailed       = importstage.Failed
)

// IsTerminal reports whether a stage represents the end of the pipeline.
func IsTerminal(stage string) bool { return importstage.IsTerminal(stage) }

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

// Broadcaster fans import stage events out to per-library subscribers.
type Broadcaster = broadcast.Broadcaster[Event]

func NewBroadcaster() *Broadcaster {
	return broadcast.New("import", func(ev Event) []any {
		return []any{"import_id", ev.ID, "stage", ev.Stage}
	})
}
