// Package licence runs licence detection against an asset's candidate
// licence text. It wraps github.com/google/licensecheck for SPDX matching
// and ships a small Unity Asset Store EULA fingerprint so the common Asset
// Store case doesn't end up as 'unknown'.
package licence

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/google/licensecheck"
)

// Result is what Detect returns. Confidence is 0–1 where 1 is an exact
// canonical match. DetectedType is an SPDX id, "unity-eula", "custom" (set
// by the caller when the user is entering a bespoke licence), or "unknown".
type Result struct {
	DetectedType        string
	DetectionConfidence float32
	Attributes          Attributes
	CanonicalDiff       string // empty if exact canonical match
}

// Attributes captures the structured booleans the UI surfaces as rows.
// Nulls are represented by absent pointers — the UI distinguishes "unknown"
// from "no".
type Attributes struct {
	CommercialUse       *bool   `json:"commercial_use"`
	Modification        *bool   `json:"modification"`
	Redistribution      *bool   `json:"redistribution"`
	AttributionRequired *bool   `json:"attribution_required"`
	ShareAlike          *bool   `json:"share_alike"`
	OpenSourceRequired  *bool   `json:"open_source_required"`
	CustomTerms         *string `json:"custom_terms,omitempty"`
}

// Detect runs licence detection on raw text. Empty input returns an
// "unknown" result with all-null attributes — the UI treats those as
// "user must confirm manually".
func Detect(text string) Result {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Result{DetectedType: "unknown"}
	}

	// Unity EULA fingerprint first — it's common, and licensecheck's SPDX
	// corpus doesn't cover it.
	if isUnityEULA(trimmed) {
		return Result{
			DetectedType:        "unity-eula",
			DetectionConfidence: 0.95,
			Attributes:          UnityEULAAttributes(),
		}
	}

	cov := licensecheck.Scan([]byte(trimmed))
	if len(cov.Match) == 0 || cov.Percent < 50 {
		return Result{DetectedType: "unknown"}
	}
	// Pick the longest match — multi-licence files are unusual but not
	// unheard of; length is a reasonable proxy for "primary".
	best := cov.Match[0]
	for _, m := range cov.Match[1:] {
		if m.End-m.Start > best.End-best.Start {
			best = m
		}
	}
	attrs := spdxAttributes(best.ID)
	result := Result{
		DetectedType:        best.ID,
		DetectionConfidence: float32(cov.Percent) / 100,
		Attributes:          attrs,
	}
	// Coverage under ~98% means the text has been modified from canonical.
	// Flag it but don't rewrite the original — canonical_diff in the UI is
	// advisory.
	if cov.Percent < 98 {
		result.CanonicalDiff = coverageNote(cov.Percent)
	}
	return result
}

func coverageNote(percent float64) string {
	return "coverage " + strconv.Itoa(int(percent)) + "%"
}

// AttributesForType returns the known attribute set for an SPDX id or
// "unity-eula", or an all-nil set for unrecognised types. Exported so callers
// applying a licence from a storefront hint (which has a type but no text to
// re-detect) get the same attribute knowledge as detection.
func AttributesForType(id string) Attributes {
	if id == "unity-eula" {
		return UnityEULAAttributes()
	}
	return spdxAttributes(id)
}

// SPDX → attribute map. Conservative: unknown attributes stay nil so the UI
// can render "unknown" rather than guessing. Boolean pointers avoid the
// false-vs-unknown ambiguity.
func spdxAttributes(id string) Attributes {
	t := boolPtr(true)
	f := boolPtr(false)
	switch id {
	case "MIT", "BSD-2-Clause", "BSD-3-Clause", "Apache-2.0", "Zlib", "ISC", "Unlicense":
		return Attributes{
			CommercialUse:       t,
			Modification:        t,
			Redistribution:      t,
			AttributionRequired: attributionForSPDX(id),
			ShareAlike:          f,
			OpenSourceRequired:  f,
		}
	case "CC0-1.0":
		return Attributes{
			CommercialUse:       t,
			Modification:        t,
			Redistribution:      t,
			AttributionRequired: f,
			ShareAlike:          f,
			OpenSourceRequired:  f,
		}
	case "CC-BY-4.0", "CC-BY-3.0", "OFL-1.1":
		return Attributes{
			CommercialUse:       t,
			Modification:        t,
			Redistribution:      t,
			AttributionRequired: t,
			ShareAlike:          f,
			OpenSourceRequired:  f,
		}
	case "CC-BY-SA-4.0", "CC-BY-SA-3.0":
		return Attributes{
			CommercialUse:       t,
			Modification:        t,
			Redistribution:      t,
			AttributionRequired: t,
			ShareAlike:          t,
			OpenSourceRequired:  f,
		}
	case "CC-BY-NC-4.0", "CC-BY-NC-3.0":
		return Attributes{
			CommercialUse:       f,
			Modification:        t,
			Redistribution:      t,
			AttributionRequired: t,
			ShareAlike:          f,
			OpenSourceRequired:  f,
		}
	case "CC-BY-NC-SA-4.0":
		return Attributes{
			CommercialUse:       f,
			Modification:        t,
			Redistribution:      t,
			AttributionRequired: t,
			ShareAlike:          t,
			OpenSourceRequired:  f,
		}
	case "GPL-2.0", "GPL-3.0", "AGPL-3.0", "LGPL-3.0":
		return Attributes{
			CommercialUse:       t,
			Modification:        t,
			Redistribution:      t,
			AttributionRequired: t,
			ShareAlike:          t,
			OpenSourceRequired:  t,
		}
	}
	return Attributes{}
}

func attributionForSPDX(id string) *bool {
	switch id {
	case "Unlicense":
		return boolPtr(false)
	}
	return boolPtr(true)
}

func boolPtr(b bool) *bool { return &b }

// ---------------- Unity EULA -----------------------------------------------

// Heuristic fingerprint: the Unity Asset Store EULA mentions both
// "Unity Asset Store" and either "End User License Agreement" or "EULA" in
// the first 4KB. Cheap and reliable enough for the common case — the user
// can override in the detail panel.
var unityHeadingRE = regexp.MustCompile(`(?i)unity[\s-]+asset[\s-]+store`)
var unityEULARE = regexp.MustCompile(`(?i)end[\s-]+user[\s-]+license[\s-]+agreement|\bEULA\b`)

func isUnityEULA(text string) bool {
	head := text
	if len(head) > 4096 {
		head = head[:4096]
	}
	return unityHeadingRE.MatchString(head) && unityEULARE.MatchString(head)
}

// UnityEULAAttributes returns the attribute set for the standard Unity Asset
// Store EULA: an asset may be used commercially and modified as part of a
// product, but not redistributed on its own, and needs no attribution. Seat
// terms differ by asset type and are not captured here; the EULA itself is
// the authority.
func UnityEULAAttributes() Attributes {
	return Attributes{
		CommercialUse:       boolPtr(true),
		Modification:        boolPtr(true),
		Redistribution:      boolPtr(false),
		AttributionRequired: boolPtr(false),
		ShareAlike:          boolPtr(false),
		OpenSourceRequired:  boolPtr(false),
	}
}
