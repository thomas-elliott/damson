// Package importstage holds the import pipeline's stage machine. It is a leaf
// package — no gin, no gorm, no River — so the dmsn CLI can validate a --stage
// flag against the same enum the server advances, rather than mirroring the
// strings and silently rejecting a stage a newer server accepts.
package importstage

import "slices"

// Stage values match the asset_imports.stage enum.
const (
	Uploaded     = "uploaded"
	Extracting   = "extracting"
	Detecting    = "detecting"
	Thumbnailing = "thumbnailing"
	Enriching    = "enriching"
	Done         = "done"
	Failed       = "failed"
)

// ordered is the stage machine in pipeline order, which is the order a caller
// wants to read them in.
var ordered = []string{Uploaded, Extracting, Detecting, Thumbnailing, Enriching, Done, Failed}

// All returns every stage in pipeline order.
func All() []string { return slices.Clone(ordered) }

// Valid reports whether stage is a known stage.
func Valid(stage string) bool { return slices.Contains(ordered, stage) }

// IsTerminal reports whether a stage represents the end of the pipeline.
func IsTerminal(stage string) bool { return stage == Done || stage == Failed }
