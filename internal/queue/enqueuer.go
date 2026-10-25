package queue

import (
	"github.com/riverqueue/river/rivertype"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/services/ingest"
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

// ImportEnqueuer inserts import jobs into River. It satisfies
// ingest.Enqueuer.
type ImportEnqueuer struct{ q *Queue }

// Enqueue inserts an import job in tx, deduped by args across active states.
func (e ImportEnqueuer) Enqueue(tx *gorm.DB, job ingest.Job) error {
	return e.q.insert(tx, ImportArgs{ImportID: job.ImportID, LibraryID: job.LibraryID}, QueueImport)
}

// PreviewEnqueuer inserts preview jobs into River. It satisfies
// preview.Enqueuer.
type PreviewEnqueuer struct{ q *Queue }

// Enqueue inserts a preview job for a specific file (or the whole version when
// FileID == 0) in tx, deduped by args across active states.
func (e PreviewEnqueuer) Enqueue(tx *gorm.DB, job preview.Job) error {
	return e.q.insert(tx, PreviewArgs{LibraryID: job.LibraryID, VersionID: job.VersionID, FileID: job.FileID}, QueuePreview)
}

// StoreLookupEnqueuer inserts store lookup jobs into River. It satisfies
// enrich.LookupEnqueuer.
type StoreLookupEnqueuer struct{ q *Queue }

// EnqueueStoreLookup inserts a job for one store_lookups row in tx, deduped
// by args across active states.
func (e StoreLookupEnqueuer) EnqueueStoreLookup(tx *gorm.DB, lookupID uint) error {
	return e.q.insert(tx, StoreLookupArgs{LookupID: lookupID}, QueueStoreLookup)
}
