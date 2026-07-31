package metadata

import (
	"strings"
	"testing"
	"time"
)

func itchFixtureMetadata(t *testing.T) *Metadata {
	t.Helper()
	body := readFixture(t, "itch_info_panel.html")
	page := "https://kenney.itch.io/kenney-game-assets"
	m := ParseHTML(body, page)
	m.Store = StoreItch
	rows := itchInfoRows(body)
	if label, href := itchLicenceFromRows(rows); label != "" {
		if spdx := firstNonEmpty(spdxFromLicenceURL(href), spdxFromLabel(label)); spdx != "" {
			m.LicenceHint = spdx
		} else if m.LicenceHint == "" {
			m.LicenceHint = label
		}
	}
	m.Tags = dedupeTags(append(m.Tags, itchTagsFromRows(rows)...))
	itchApplyRows(m, rows, page)
	return m
}

func TestItchInfoRowsHarvestsEveryRow(t *testing.T) {
	rows := itchInfoRows(readFixture(t, "itch_info_panel.html"))
	var labels []string
	for _, r := range rows {
		labels = append(labels, r.Label)
	}
	joined := strings.Join(labels, ",")
	for _, want := range []string{"updated", "status", "category", "author", "tags", "asset license"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("rows = %v, want a %q row", labels, want)
		}
	}
}

func TestItchApplyRowsMapsTypedFields(t *testing.T) {
	m := itchFixtureMetadata(t)

	if m.Publisher != "Kenney" {
		t.Fatalf("Publisher = %q, want the Author row", m.Publisher)
	}
	if m.PublisherURL != "https://kenney.itch.io" {
		t.Fatalf("PublisherURL = %q, want the Author row's href", m.PublisherURL)
	}
	// The Updated row renders as "14 days ago"; the real timestamp is only in
	// the abbr title, so reading the cell text would lose the date entirely.
	want := time.Date(2026, 7, 16, 19, 45, 0, 0, time.UTC)
	if m.StoreUpdatedAt == nil || !m.StoreUpdatedAt.Equal(want) {
		t.Fatalf("StoreUpdatedAt = %v, want %v from the abbr title", m.StoreUpdatedAt, want)
	}
	if m.Rating != 5.0 || m.RatingCount != 579 {
		t.Fatalf("rating = %v (%d), want 5.0 (579) from the JSON-LD aggregateRating", m.Rating, m.RatingCount)
	}
	if m.Price != "19.95" || m.Currency != "USD" {
		t.Fatalf("price = %q %q", m.Price, m.Currency)
	}
	if m.LicenceHint == "" {
		t.Fatal("licence hint not mapped from the Asset license row")
	}
	if got := strings.Join(m.Tags, ","); !strings.Contains(got, "2D") {
		t.Fatalf("tags = %q, want the Tags row's anchors", got)
	}
}

func TestItchApplyRowsRecordsRemainingRowsAsAttributes(t *testing.T) {
	m := itchFixtureMetadata(t)

	got := map[string]string{}
	for _, a := range m.Attributes {
		got[a.Key] = a.Value
	}
	if got["status"] != "In development" {
		t.Fatalf("status attribute = %q (all: %v)", got["status"], got)
	}
	if got["category"] != "Assets" {
		t.Fatalf("category attribute = %q", got["category"])
	}
	// Rows that fed a typed field must not be duplicated as attributes.
	for _, key := range []string{"author", "updated", "tags", "rating", "asset_license"} {
		if _, dup := got[key]; dup {
			t.Fatalf("row %q was mapped to a typed field and duplicated as an attribute", key)
		}
	}
}

// The listing's description contains a two-cell prose table; without a label
// length guard its first cell becomes a garbage attribute label.
func TestItchInfoRowsSkipsLongProseLabels(t *testing.T) {
	body := []byte(`<html><body><table><tbody>
	<tr><td>Status</td><td>Released</td></tr>
	<tr><td>Depending on the asset file formats include PNG, SVG, OBJ and OGG compatible with all engines.</td><td>Some other prose entirely.</td></tr>
	</tbody></table></body></html>`)
	rows := itchInfoRows(body)
	for _, r := range rows {
		if len(r.Label) > itchMaxLabelLen {
			t.Fatalf("harvested a prose row as an info row: %q", r.Label)
		}
	}
	if len(rows) == 0 {
		t.Fatal("the real Status row was dropped too")
	}
}

func TestItchAttributeKey(t *testing.T) {
	for in, want := range map[string]string{
		"made with":       "made_with",
		"average session": "average_session",
		"file size":       "file_size",
		"inputs":          "inputs",
		"":                "",
		"a/b":             "a_b",
	} {
		if got := itchAttributeKey(in); got != want {
			t.Fatalf("itchAttributeKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestItchRowTimePrefersAbbrTitle(t *testing.T) {
	r := itchRow{Value: "14 days ago", AbbrTitle: "16 July 2026 @ 19:45 UTC"}
	got := itchRowTime(r)
	if got == nil || !got.Equal(time.Date(2026, 7, 16, 19, 45, 0, 0, time.UTC)) {
		t.Fatalf("itchRowTime = %v", got)
	}
	// A relative date with no abbr title yields nothing rather than a wrong date.
	if got := itchRowTime(itchRow{Value: "14 days ago"}); got != nil {
		t.Fatalf("itchRowTime(relative only) = %v, want nil", got)
	}
	if got := itchRowTime(itchRow{Value: "2024-03-01"}); got == nil {
		t.Fatal("an absolute cell value should still parse")
	}
}

// A whole-page walk harvests the "More information" disclosure wrapper — whose
// two child divs look like a row carrying the entire table as its value — and
// the community post list, which uses the same two-div shape. Both were found
// on a real listing, not in a hand-built fixture, so this uses the real page.
func TestItchInfoRowsIgnoresEverythingOutsideTheInfoPanel(t *testing.T) {
	body := readFixture(t, "itch_listing_page.html")
	page := "https://kenney.itch.io/kenney-game-assets"
	m := ParseHTML(body, page)
	rows := itchInfoRows(body)
	itchApplyRows(m, rows, page)

	got := map[string]string{}
	for _, a := range m.Attributes {
		got[a.Key] = a.Value
	}
	// The wrapper's label is short enough to pass the length guard, so only
	// scoping to the info panel excludes it.
	if v, present := got["more_information"]; present {
		t.Errorf("harvested the disclosure wrapper as an attribute: %q", v)
	}
	for key, value := range got {
		if strings.Contains(value, "total ratings") || strings.Contains(value, "posts") {
			t.Errorf("attribute %q carries content from outside the info panel: %q", key, value)
		}
	}
	// The real rows still land.
	if got["status"] != "In development" {
		t.Errorf("status = %q", got["status"])
	}
	if len(m.Attributes) > 5 {
		t.Errorf("harvested %d attributes, expected only the info panel's few: %v", len(m.Attributes), got)
	}
}
