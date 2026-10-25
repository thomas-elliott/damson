package preview

// Preview status values mirror the asset_files.preview_status column. A file's
// status is the first-class state of its backend preview derivative, so the
// processing queue and requeue can observe and act on it.
//
//	unsupported  no backend renderer for this file type (the common case)
//	queued       supported, awaiting a preview worker
//	processing   a worker is rendering it
//	done         preview_object_key is set
//	failed       the render errored (retryable via requeue)
const (
	StatusUnsupported = "unsupported"
	StatusQueued      = "queued"
	StatusProcessing  = "processing"
	StatusDone        = "done"
	StatusFailed      = "failed"
)

// rollupStatus derives a version's overall preview state from its per-file
// counts: processing if any file is mid-render, else queued while any remain,
// else terminal ("done" — failures show in the Failed count, not the status).
func rollupStatus(processing, queued int) string {
	switch {
	case processing > 0:
		return StatusProcessing
	case queued > 0:
		return StatusQueued
	default:
		return StatusDone
	}
}
