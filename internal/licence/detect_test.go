package licence

import (
	"reflect"
	"strings"
	"testing"
)

const mitText = `MIT License

Copyright (c) 2024 Example Author

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`

func TestDetect_EmptyTextIsUnknown(t *testing.T) {
	got := Detect("  \n ")
	if got.DetectedType != "unknown" || got.Attributes != (Attributes{}) {
		t.Fatalf("got %+v, want unknown with no attributes", got)
	}
}

func TestDetect_UnityEULA(t *testing.T) {
	got := Detect("Unity Asset Store End User License Agreement\n\nThis EULA governs…")
	if got.DetectedType != "unity-eula" {
		t.Fatalf("type = %q, want unity-eula", got.DetectedType)
	}
	if got.Attributes.Redistribution == nil || *got.Attributes.Redistribution {
		t.Errorf("redistribution = %v, want false", got.Attributes.Redistribution)
	}
}

func TestDetect_CanonicalMITHasNoDiff(t *testing.T) {
	got := Detect(mitText)
	if got.DetectedType != "MIT" {
		t.Fatalf("type = %q, want MIT", got.DetectedType)
	}
	if got.CanonicalDiff != "" {
		t.Errorf("canonical diff = %q, want none", got.CanonicalDiff)
	}
	if got.Attributes.CommercialUse == nil || !*got.Attributes.CommercialUse {
		t.Errorf("commercial use = %v, want true", got.Attributes.CommercialUse)
	}
}

func TestDetect_ModifiedMITReportsCoverage(t *testing.T) {
	extra := strings.Repeat("Except that this asset may not be resold on its own. ", 4)
	got := Detect(mitText + "\n" + extra)
	if got.DetectedType != "MIT" {
		t.Fatalf("type = %q, want MIT", got.DetectedType)
	}
	if !strings.HasPrefix(got.CanonicalDiff, "coverage ") || !strings.HasSuffix(got.CanonicalDiff, "%") {
		t.Errorf("canonical diff = %q, want a coverage note", got.CanonicalDiff)
	}
}

func TestDetect_UnrelatedTextIsUnknown(t *testing.T) {
	got := Detect("Use this however you like, just don't sell it as-is.")
	if got.DetectedType != "unknown" {
		t.Fatalf("type = %q, want unknown", got.DetectedType)
	}
}

func TestCoverageNote(t *testing.T) {
	cases := map[float64]string{
		90:   "coverage 90%",
		93.6: "coverage 93%",
		50:   "coverage 50%",
		97.9: "coverage 97%",
	}
	for pct, want := range cases {
		if got := coverageNote(pct); got != want {
			t.Errorf("coverageNote(%v) = %q, want %q", pct, got, want)
		}
	}
}

// The store lookups record CC 3.0 licences under their own ids, so each needs
// the same attributes as its 4.0 counterpart.
func TestAttributesForType_CC30MatchesCC40(t *testing.T) {
	for _, base := range []string{"CC-BY", "CC-BY-SA", "CC-BY-NC", "CC-BY-NC-SA"} {
		v3, v4 := AttributesForType(base+"-3.0"), AttributesForType(base+"-4.0")
		if v3.CommercialUse == nil || !reflect.DeepEqual(v3, v4) {
			t.Errorf("%s-3.0 attributes = %+v, want those of %s-4.0", base, v3, base)
		}
	}
}
