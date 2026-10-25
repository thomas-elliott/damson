package imports

import (
	"sort"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestParseStoreVersion(t *testing.T) {
	if _, ok := ParseStoreVersion(""); ok {
		t.Error("empty string should not parse")
	}
	if _, ok := ParseStoreVersion("1.x"); ok {
		t.Error("non-numeric segment should not parse")
	}
	parts, ok := ParseStoreVersion("v1.10")
	if !ok || len(parts) != 2 || parts[0] != 1 || parts[1] != 10 {
		t.Errorf("ParseStoreVersion(v1.10) = %v, %v", parts, ok)
	}
}

func TestCompareStoreVersions(t *testing.T) {
	p := func(s string) []int { v, _ := ParseStoreVersion(s); return v }
	if CompareStoreVersions(p("1.10"), p("1.9")) != 1 {
		t.Error("1.10 should be greater than 1.9")
	}
	if CompareStoreVersions(p("1.1"), p("1.10")) != -1 {
		t.Error("1.1 should be less than 1.10")
	}
	if CompareStoreVersions(p("1.0"), p("1")) != 0 {
		t.Error("1.0 should equal 1")
	}
}

func TestSemanticOrdering(t *testing.T) {
	got := []string{"1.1", "1.10", "1.9"}
	sort.Slice(got, func(i, j int) bool {
		a, _ := ParseStoreVersion(got[i])
		b, _ := ParseStoreVersion(got[j])
		return CompareStoreVersions(a, b) > 0
	})
	want := []string{"1.10", "1.9", "1.1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordering = %v, want %v", got, want)
		}
	}
}

func TestStoreUpdateAvailable(t *testing.T) {
	cases := []struct {
		name          string
		latest        string
		held          []string
		wantAvailable bool
		wantHighest   string
	}{
		{"newer store version", "1.24", []string{"1.23"}, true, "1.23"},
		{"same version", "1.24", []string{"1.24"}, false, "1.24"},
		{"held is newer", "1.23", []string{"1.24"}, false, "1.24"},
		// Textual comparison would call 1.9 newer than 1.10.
		{"double-digit minor", "1.10", []string{"1.9"}, true, "1.9"},
		{"reverse double-digit", "1.9", []string{"1.10"}, false, "1.10"},
		{"compares against the highest held", "1.24", []string{"1.9", "1.23", "1.10"}, true, "1.23"},
		{"already holds the newest", "1.24", []string{"1.9", "1.24", "1.10"}, false, "1.24"},
		// Trailing zeros are not a difference.
		{"trailing zero equal", "1.0", []string{"1"}, false, "1"},
		{"trailing zero reverse", "1", []string{"1.0"}, false, "1.0"},
		{"v prefix", "v1.24", []string{"V1.23"}, true, "1.23"},
		// Nothing to compare against: never report an update.
		{"no held versions", "1.24", nil, false, ""},
		{"held versions unorderable", "1.24", []string{"", "latest", "2020.3f1"}, false, ""},
		{"latest unorderable", "1.24b", []string{"1.23"}, false, ""},
		{"latest missing", "", []string{"1.23"}, false, ""},
		// An unorderable version among orderable ones is skipped, not fatal.
		{"mixed orderability", "1.24", []string{"nightly", "1.23"}, true, "1.23"},
	}
	for _, c := range cases {
		gotAvailable, gotHighest := StoreUpdateAvailable(c.latest, c.held)
		if gotAvailable != c.wantAvailable {
			t.Errorf("%s: available = %v, want %v", c.name, gotAvailable, c.wantAvailable)
		}
		if gotHighest != c.wantHighest {
			t.Errorf("%s: highest held = %q, want %q", c.name, gotHighest, c.wantHighest)
		}
	}
}

// The asset's current version is its highest parseable store version, then
// its newest import, passing over a version whose latest import has not
// finished unless that import is the one finishing.
func TestChooseCurrent(t *testing.T) {
	db := enrichTestDB(t)
	seedEnrichAsset(t, db, "seed", false)
	now := time.Now()

	type version struct {
		store   string
		age     time.Duration
		pending bool
	}
	cases := []struct {
		name     string
		versions []version
		want     int // index into versions, or -1 for none
	}{
		{"store version beats a newer import", []version{{"2.0", 2 * time.Hour, false}, {"1.0", time.Hour, false}}, 0},
		{"a store version tie goes to the newer import", []version{{"1.0", 2 * time.Hour, false}, {"1", time.Hour, false}}, 1},
		{"a parseable store version beats none", []version{{"", time.Hour, false}, {"1.0", 2 * time.Hour, false}}, 1},
		{"without store versions the newer import wins", []version{{"", 2 * time.Hour, false}, {"", time.Hour, false}}, 1},
		{"a version with pending files is passed over", []version{{"1.0", 2 * time.Hour, false}, {"2.0", time.Hour, true}}, 0},
		{"no version with files leaves no current version", []version{{"1.0", time.Hour, true}}, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			asset := models.Asset{LibraryID: 1, Name: tc.name}
			if err := db.Create(&asset).Error; err != nil {
				t.Fatal(err)
			}
			ids := make([]uint, len(tc.versions))
			for i, v := range tc.versions {
				row := models.AssetVersion{AssetID: asset.ID, StoreVersion: v.store, ImportDate: now.Add(-v.age), FilesPending: v.pending}
				if err := db.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
				ids[i] = row.ID
			}

			if _, err := ChooseCurrent(db, asset.ID); err != nil {
				t.Fatal(err)
			}

			var got models.Asset
			if err := db.First(&got, asset.ID).Error; err != nil {
				t.Fatal(err)
			}
			switch {
			case tc.want < 0 && got.CurrentVersionID != nil:
				t.Errorf("current version = %d, want none", *got.CurrentVersionID)
			case tc.want >= 0 && (got.CurrentVersionID == nil || *got.CurrentVersionID != ids[tc.want]):
				t.Errorf("current version = %v, want %d (%q)", got.CurrentVersionID, ids[tc.want], tc.versions[tc.want].store)
			}
		})
	}
}
