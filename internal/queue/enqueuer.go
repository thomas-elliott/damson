package queue

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"

	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/preview"
)

// ImportEnqueuer inserts import jobs into River. It satisfies the enqueuer
// interface the API handlers depend on. Enqueue is fire-and-forget (matching the
// old channel pool): a failure is logged, and the startup reconcile re-enqueues
// any domain row left without a job.
type ImportEnqueuer struct{ q *Queue }

// Enqueue inserts an import job on the import queue.
func (e ImportEnqueuer) Enqueue(job imports.Job) {
	_, err := e.q.client.Insert(context.Background(),
		ImportArgs{ImportID: job.ImportID, LibraryID: job.LibraryID},
		&river.InsertOpts{Queue: QueueImport},
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
// FileID == 0) on the preview queue.
func (e PreviewEnqueuer) Enqueue(job preview.Job) {
	_, err := e.q.client.Insert(context.Background(),
		PreviewArgs{LibraryID: job.LibraryID, VersionID: job.VersionID, FileID: job.FileID},
		&river.InsertOpts{Queue: QueuePreview},
	)
	if err != nil {
		slog.Error("queue: enqueue preview", "version_id", job.VersionID, "file_id", job.FileID, "err", err)
	}
}

// EnqueueVersion inserts a preview job covering every queued file in a version.
func (e PreviewEnqueuer) EnqueueVersion(libraryID, versionID uint) {
	e.Enqueue(preview.Job{LibraryID: libraryID, VersionID: versionID})
}
