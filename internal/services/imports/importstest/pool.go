// Package importstest provides a synchronous import enqueuer for tests.
package importstest

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/services/imports"
)

// Pool runs the import pipeline inline on the calling goroutine, so a test
// sees the import terminal as soon as Enqueue returns, without a running River
// client. The pipeline runs in the enqueuing transaction, as if the job ran
// once it committed. It satisfies imports.Enqueuer.
type Pool struct {
	deps imports.Deps
}

// NewSync returns a pool whose Enqueue runs the pipeline on the calling
// goroutine.
func NewSync(deps imports.Deps) *Pool {
	return &Pool{deps: deps}
}

// Enqueue runs the pipeline inline, in tx. A pipeline error is logged, as a
// job's would be, not returned.
func (p *Pool) Enqueue(tx *gorm.DB, job imports.Job) error {
	deps := p.deps
	deps.DB = tx
	if err := imports.Run(context.Background(), deps, job); err != nil {
		slog.Error("sync import pipeline failed", "import_id", job.ImportID, "library_id", job.LibraryID, "err", err)
	}
	return nil
}
