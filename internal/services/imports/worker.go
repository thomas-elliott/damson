package imports

import (
	"context"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/services/metadata"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// MetadataEnricher is the storefront-metadata dependency the auto-enrich
// stage uses. *metadata.Registry satisfies it. Optional on Deps — a nil
// enricher skips enrichment entirely.
type MetadataEnricher interface {
	Search(ctx context.Context, query string) ([]metadata.Candidate, error)
	Fetch(ctx context.Context, rawURL string) (*metadata.Metadata, error)
	FetchImage(ctx context.Context, rawURL string) ([]byte, string, error)
}

// Job is the unit of work for the import pipeline. It carries only the
// import row's id (and the library id for broadcaster routing); everything
// else the pipeline needs is loaded from the DB. This keeps Enqueue cheap
// and makes resume-on-startup trivial — feed in the IDs of stuck rows and
// the pipeline figures out what's left to do.
type Job struct {
	ImportID  uint
	LibraryID uint
}

// PreviewEnqueuer hands a committed version off to the preview processing
// queue. Defined here (not as a concrete *preview.Pool) so the import package
// depends only on the enqueue behaviour, keeping the two pipelines decoupled.
type PreviewEnqueuer interface {
	EnqueueVersion(libraryID, versionID uint)
}

// Deps are the long-lived collaborators every pipeline run needs. They live
// on the Pool and are passed into Run.
type Deps struct {
	DB          *gorm.DB
	Storage     storage.Service
	Broadcaster *Broadcaster
	// Previews receives each version after its asset_files are committed, so
	// preview generation runs on its own queue instead of blocking the import.
	// Optional — a nil enqueuer simply skips the handoff.
	Previews PreviewEnqueuer
	// Metadata drives the auto-enrichment stage. Optional — nil skips it.
	Metadata MetadataEnricher
}
