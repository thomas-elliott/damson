package enrich

import (
	"encoding/json"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/licence"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// listingLicence turns a listing's licence text or hint into an unsaved
// licence. Text runs through the detector; a bare SPDX or EULA hint gets its
// known attributes and a stand-in original_text, which must not be blank.
// It returns nil when the listing says nothing about a licence.
func listingLicence(meta *metadata.Metadata) *models.Licence {
	text := meta.LicenceText
	var detType string
	var attrs licence.Attributes
	var conf float32
	var canon string
	switch {
	case text != "":
		r := licence.Detect(text)
		detType = r.DetectedType
		attrs = r.Attributes
		conf = r.DetectionConfidence
		canon = r.CanonicalDiff
	case meta.LicenceHint != "":
		detType = meta.LicenceHint
		attrs = licence.AttributesForType(detType)
		store := meta.Store
		if store == "" {
			store = "source"
		}
		text = "Licence: " + meta.LicenceHint + " (from " + store + " listing)"
	default:
		return nil
	}
	attrsJSON, err := json.Marshal(attrs)
	if err != nil {
		attrsJSON = []byte("{}")
	}
	return &models.Licence{
		DetectedType:        detType,
		DetectionConfidence: conf,
		CanonicalDiff:       canon,
		OriginalText:        text,
		AttributesJSON:      attrsJSON,
	}
}
