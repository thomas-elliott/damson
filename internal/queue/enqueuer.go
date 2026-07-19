package queue

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/preview"
)

// maxAttempts bounds retries per job. Transient failures retry with exponential
// backoff up to this many attempts; terminal failures are cancelled well before
// it via river.JobCancel.
const maxAttempts = 5

// activeStates are the non-terminal River job states. Uniqueness is enforced
// only across these, so a fresh enqueue is skipped while an equivalent job is
// still pending/active, but a new one proceeds once the previous job has
// completed, failed, or been cancelled. This makes the startup reconcile safe to
// run every boot and safe alongside the handler enqueue paths.
var activeStates = []rivertype.JobState{
	rivertype.JobStateAvailable,
	rivertype.JobStatePending,
	rivertype.JobStateRunning,
	rivertype.JobStateRetryable,
	rivertype.JobStateScheduled,
}

// ImportEnqueuer inserts import jobs into River. It satisfies the enqueuer
// interface the API handlers depend on. Enqueue is fire-and-forget (matching the
// old channel pool): a failure is logged, and the startup reconcile re-enqueues
// any domain row left without a job.
type ImportEnqueuer struct{ q *Queue }

// Enqueue inserts an import job on the import queue, deduped by args across
// active states.
func (e ImportEnqueuer) Enqueue(job imports.Job) {
	_, err := e.q.client.Insert(context.Background(),
		ImportArgs{ImportID: job.ImportID, LibraryID: job.LibraryID},
		&river.InsertOpts{
			Queue:       QueueImport,
			MaxAttempts: maxAttempts,
			UniqueOpts:  river.UniqueOpts{ByArgs: true, ByState: activeStates},
		},
	)
	if err != nil {
		slog.Error("queue: enqueue import", "import_id", job.ImportID, "library_id", job.LibraryID, "err", err)
	}
}

// PreviewEnqueuer inserts preview jobs into River. Satisfies both the API
// handlers' enqueuer interface and the import pipeline's PreviewEnqueuer (the
// import→preview handoff).
type PreviewEnqueuer struct{ q *Queue }

// Enqueue inserts a preview job for a specific file (or the whole version when
// FileID == 0) on the preview queue, deduped by args across active states.
func (e PreviewEnqueuer) Enqueue(job preview.Job) {
	_, err := e.q.client.Insert(context.Background(),
		PreviewArgs{LibraryID: job.LibraryID, VersionID: job.VersionID, FileID: job.FileID},
		&river.InsertOpts{
			Queue:       QueuePreview,
			MaxAttempts: maxAttempts,
			UniqueOpts:  river.UniqueOpts{ByArgs: true, ByState: activeStates},
		},
	)
	if err != nil {
		slog.Error("queue: enqueue preview", "version_id", job.VersionID, "file_id", job.FileID, "err", err)
	}
}

// EnqueueVersion inserts a preview job covering every queued file in a version.
func (e PreviewEnqueuer) EnqueueVersion(libraryID, versionID uint) {
	e.Enqueue(preview.Job{LibraryID: libraryID, VersionID: versionID})
}
