package main

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReferences_AreTheSameOnEveryRun(t *testing.T) {
	if !reflect.DeepEqual(references(), references()) {
		t.Fatal("two runs generated different references")
	}
}

func TestReferences_Shape(t *testing.T) {
	refs := references()
	if n := len(refs); n < 280 || n > 320 {
		t.Fatalf("generated %d references, want about 300", n)
	}

	var pathOnly, linkOnly, both, needsReview int
	paths, links := map[string]bool{}, map[string]bool{}
	for _, r := range refs {
		switch {
		case r.Path != "" && r.Link != "":
			both++
		case r.Path != "":
			pathOnly++
		case r.Link != "":
			linkOnly++
		default:
			t.Fatalf("%q has neither a path nor a link", r.Name)
		}
		if r.NeedsReview {
			needsReview++
		}
		if r.Path != "" {
			if paths[r.Path] {
				t.Errorf("path %s generated twice", r.Path)
			}
			paths[r.Path] = true
			want := "/mnt/assets/" + r.Publisher + "/" + r.Category + "/"
			if !strings.HasPrefix(r.Path, want) {
				t.Errorf("path %s does not start with %s", r.Path, want)
			}
		}
		if r.Link != "" {
			if links[r.Link] {
				t.Errorf("link %s generated twice", r.Link)
			}
			links[r.Link] = true
		}
	}
	third := len(refs) / 3
	for name, n := range map[string]int{"path-only": pathOnly, "link-only": linkOnly, "path and link": both} {
		if n < third*8/10 || n > third*12/10 {
			t.Errorf("%d %s references, want about %d", n, name, third)
		}
	}
	if share := float64(needsReview) / float64(len(refs)); share < 0.10 || share > 0.20 {
		t.Errorf("%.0f%% need review, want about 15%%", share*100)
	}
}

// Every row carries what the filters and the panel read; only a row with a
// link carries what a store listing gives.
func TestReferences_Fields(t *testing.T) {
	storeHosts := map[string]func(host string) bool{
		"unity":     func(h string) bool { return h == "assetstore.unity.com" },
		"itch":      func(h string) bool { return strings.HasSuffix(h, ".itch.io") },
		"sketchfab": func(h string) bool { return h == "sketchfab.com" },
		"generic":   func(h string) bool { return h == "www.fab.com" },
	}
	for _, r := range references() {
		if r.Name == "" || r.Publisher == "" || r.Category == "" || r.Store == "" ||
			r.ReleasedAt.IsZero() || r.LatestStoreVersion == "" || r.Licence == "" {
			t.Errorf("%q is missing a field every row carries: %+v", r.Name, r)
		}
		listing := []string{r.StoreProductID, r.PublisherURL, r.Engine, r.MinEngineVersion, r.Price, r.PriceCurrency}
		if r.Link == "" {
			for _, v := range listing {
				if v != "" {
					t.Errorf("path-only %q carries listing field %q", r.Name, v)
				}
			}
			if r.StoreUpdatedAt != nil || r.Rating != nil || r.RatingCount != nil || r.StoreDeprecated {
				t.Errorf("path-only %q carries listing fields: %+v", r.Name, r)
			}
			continue
		}
		for _, v := range listing {
			if v == "" {
				t.Errorf("linked %q is missing a listing field: %+v", r.Name, r)
				break
			}
		}
		if r.StoreUpdatedAt == nil || r.Rating == nil || r.RatingCount == nil {
			t.Errorf("linked %q is missing a listing field: %+v", r.Name, r)
		}
		u, err := url.Parse(r.Link)
		if err != nil {
			t.Fatalf("link %q: %v", r.Link, err)
		}
		if matches, ok := storeHosts[r.Store]; !ok || !matches(u.Hostname()) {
			t.Errorf("%q has store %q but links to %s", r.Name, r.Store, u.Hostname())
		}
	}
}

func TestReferences_NamesRepeat(t *testing.T) {
	refs := references()
	seen := map[string]int{}
	for _, r := range refs {
		seen[r.Name]++
	}
	var duplicate bool
	for _, n := range seen {
		if n > 1 {
			duplicate = true
		}
	}
	if !duplicate {
		t.Error("no two references share a name")
	}
	stem, _, _ := strings.Cut(nearDuplicateStem, " ")
	var versions, urp int
	for name := range seen {
		if strings.HasPrefix(name, nearDuplicateStem+" v1.") {
			versions++
		}
		if strings.HasSuffix(name, "_URP") && seen[strings.TrimSuffix(name, "_URP")] > 0 {
			urp++
		}
	}
	if versions < 2 || urp == 0 || stem == "" {
		t.Errorf("want %q in two versions and a _URP twin; got %d versions, %d twins", nearDuplicateStem, versions, urp)
	}
}

// Each filtered field splits the set: some rows match, not all.
func TestReferences_FiltersSplitTheSet(t *testing.T) {
	refs := references()
	split := func(name string, match func(reference) bool) {
		n := 0
		for _, r := range refs {
			if match(r) {
				n++
			}
		}
		if n == 0 || n == len(refs) {
			t.Errorf("%s matches %d of %d", name, n, len(refs))
		}
	}
	first := refs[0]
	split("needs review", func(r reference) bool { return r.NeedsReview })
	split("publisher", func(r reference) bool { return r.Publisher == first.Publisher })
	split("store", func(r reference) bool { return r.Store == first.Store })
	split("released after", func(r reference) bool { return !r.ReleasedAt.Before(releasedAfterProbe) })
	for _, licence := range []string{"MIT", "unity-eula", "custom", "unknown"} {
		split("licence "+licence, func(r reference) bool { return r.Licence == licence })
	}
	split("not downloaded", func(r reference) bool { return r.Path == "" })
}

// releasedAfterProbe sits inside the generated release dates.
var releasedAfterProbe = time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
