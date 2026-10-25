// Package importstest provides a synchronous import enqueuer for tests.
package importstest

import (
	"context"
	"log/slog"

	"github.com/thomas-elliott/damson/internal/services/imports"
)

// Pool runs the import pipeline inline on the calling goroutine, so a test
// sees the import terminal as soon as Enqueue returns, without a running River
// client. It satisfies the api ImportEnqueuer seam.
type Pool struct {
	deps imports.Deps
}

// NewSync returns a pool whose Enqueue runs the pipeline on the calling
// goroutine.
func NewSync(deps imports.Deps) *Pool {
	return &Pool{deps: deps}
}

// Enqueue runs the pipeline inline.
func (p *Pool) Enqueue(job imports.Job) {
	if err := imports.Run(context.Background(), p.deps, job); err != nil {
		slog.Error("sync import pipeline failed", "import_id", job.ImportID, "library_id", job.LibraryID, "err", err)
	}
}
