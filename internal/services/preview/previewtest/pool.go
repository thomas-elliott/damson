// Package previewtest provides a synchronous preview enqueuer for tests.
package previewtest

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/services/preview"
)

// Pool runs preview processing inline on the calling goroutine, so status
// transitions are deterministic without a running River client. It satisfies
// preview.Enqueuer.
type Pool struct {
	deps preview.Deps
}

// NewSync returns a pool whose Enqueue runs processing on the calling
// goroutine.
func NewSync(deps preview.Deps) *Pool {
	return &Pool{deps: deps}
}

// Enqueue runs processing inline, in tx, as if the job ran once tx
// committed. A processing error is logged, as a job's would be, not returned.
func (p *Pool) Enqueue(tx *gorm.DB, job preview.Job) error {
	deps := p.deps
	deps.DB = tx
	if err := preview.ProcessVersion(context.Background(), deps, job.LibraryID, job.VersionID, job.FileID); err != nil {
		slog.Warn("sync preview pipeline failed", "version_id", job.VersionID, "err", err)
	}
	return nil
}
