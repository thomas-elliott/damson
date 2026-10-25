// Package previewtest provides a synchronous preview enqueuer for tests.
package previewtest

import (
	"context"
	"log/slog"

	"github.com/thomas-elliott/damson/internal/services/preview"
)

// Pool runs preview processing inline on the calling goroutine, so status
// transitions are deterministic without a running River client. It satisfies
// the api PreviewEnqueuer seam and the import pipeline's PreviewEnqueuer.
type Pool struct {
	deps preview.Deps
}

// NewSync returns a pool whose Enqueue runs processing on the calling
// goroutine.
func NewSync(deps preview.Deps) *Pool {
	return &Pool{deps: deps}
}

// Enqueue runs processing inline.
func (p *Pool) Enqueue(job preview.Job) {
	if err := preview.ProcessVersion(context.Background(), p.deps, job.LibraryID, job.VersionID, job.FileID); err != nil {
		slog.Warn("sync preview pipeline failed", "version_id", job.VersionID, "err", err)
	}
}

// EnqueueVersion enqueues every queued file in a version.
func (p *Pool) EnqueueVersion(libraryID, versionID uint) {
	p.Enqueue(preview.Job{LibraryID: libraryID, VersionID: versionID})
}
