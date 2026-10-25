package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/metadata"
	"github.com/thomas-elliott/damson/internal/models"
)

func ptr[T any](v T) *T { return &v }

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return ts
}

func TestValidateSource(t *testing.T) {
	ok := []struct {
		name     string
		kind     string
		path     string
		wantPath string
	}{
		{"no source at all", "", "", ""},
		{"unity cache", SourceKindUnityCache, "Pub/AudioSound FX/Pack.unitypackage", "Pub/AudioSound FX/Pack.unitypackage"},
		{"windows separators", SourceKindUnityCache, `Pub\Audio\Pack.unitypackage`, "Pub/Audio/Pack.unitypackage"},
		{"surrounding space", " " + SourceKindUnityCache + " ", " Pub/Audio/Pack.unitypackage ", "Pub/Audio/Pack.unitypackage"},
	}
	for _, c := range ok {
		got, err := ValidateSource(c.kind, c.path)
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
			continue
		}
		if got != c.wantPath {
			t.Errorf("%s: path = %q, want %q", c.name, got, c.wantPath)
		}
	}

	bad := []struct {
		name string
		kind string
		path string
	}{
		{"unknown kind", "dropbox", "Pub/Audio/Pack.unitypackage"},
		{"path with no kind", "", "Pub/Audio/Pack.unitypackage"},
		{"kind with no path", SourceKindUnityCache, ""},
		{"absolute", SourceKindUnityCache, "/Pub/Audio/Pack.unitypackage"},
		{"traversal", SourceKindUnityCache, "Pub/../../Pack.unitypackage"},
		{"control character", SourceKindUnityCache, "Pub/Au\ndio/Pack.unitypackage"},
	}
	for _, c := range bad {
		if _, err := ValidateSource(c.kind, c.path); err == nil {
			t.Errorf("%s: expected an error, got none", c.name)
		}
	}
}

func TestSuppressesFilenameVersion(t *testing.T) {
	if !SuppressesFilenameVersion(SourceKindUnityCache) {
		t.Error("unity_cache should suppress the filename version parse")
	}
	if SuppressesFilenameVersion("") {
		t.Error("a hand-dragged upload must keep the filename version parse")
	}
}

func seedSourceAsset(t *testing.T, db *gorm.DB, libraryID uint, name string) *models.Asset {
	t.Helper()
	if err := db.FirstOrCreate(&models.Library{ID: libraryID}, models.Library{ID: libraryID, Name: "lib"}).Error; err != nil {
		t.Fatalf("seed library: %v", err)
	}
	a := models.Asset{LibraryID: libraryID, Name: name}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return &a
}

func TestApplySourceHints(t *testing.T) {
	db := enrichTestDB(t)
	asset := seedSourceAsset(t, db, 1, "RPG Builder")
	version := models.AssetVersion{
		AssetID:            asset.ID,
		SourceKind:         ptr(SourceKindUnityCache),
		SourceRelativePath: ptr("Blink/Editor ExtensionsGame ToolkitsRPG Toolkits/RPG Builder.unitypackage"),
	}
	if err := db.Create(&version).Error; err != nil {
		t.Fatalf("seed version: %v", err)
	}
	jc := jobContext{asset: *asset, version: version, libraryID: 1}

	ApplySourceHints(context.Background(), Deps{DB: db}, jc)

	var got models.Asset
	if err := db.First(&got, asset.ID).Error; err != nil {
		t.Fatalf("reload asset: %v", err)
	}
	if got.Publisher == nil || *got.Publisher != "Blink" {
		t.Errorf("Publisher = %v, want Blink", got.Publisher)
	}

	// Attach links the leaf only, creating the ancestors as tag rows so the
	// hierarchy exists for the tag tree.
	var linked []string
	if err := db.Table("tags t").
		Joins("JOIN asset_tags at ON at.tag_id = t.id").
		Where("at.asset_id = ?", asset.ID).
		Order("t.path").Pluck("t.path", &linked).Error; err != nil {
		t.Fatalf("load linked tags: %v", err)
	}
	wantLinked := []string{"editor extensions/game toolkits/rpg toolkits"}
	if len(linked) != 1 || linked[0] != wantLinked[0] {
		t.Fatalf("linked tag paths = %#v, want %#v", linked, wantLinked)
	}

	var created []string
	if err := db.Table("tags").Where("library_id = ?", 1).
		Order("path").Pluck("path", &created).Error; err != nil {
		t.Fatalf("load tags: %v", err)
	}
	wantCreated := []string{
		"editor extensions",
		"editor extensions/game toolkits",
		"editor extensions/game toolkits/rpg toolkits",
	}
	if len(created) != len(wantCreated) {
		t.Fatalf("tag rows = %#v, want %#v", created, wantCreated)
	}
	for i, w := range wantCreated {
		if created[i] != w {
			t.Errorf("tag rows[%d] = %q, want %q", i, created[i], w)
		}
	}

	// Re-running (a resumed import) must be a no-op, not a duplicate or an
	// overwrite of a publisher the user has since corrected.
	if err := db.Model(&models.Asset{}).Where("id = ?", asset.ID).
		Update("publisher", "Blink Studios").Error; err != nil {
		t.Fatalf("set publisher: %v", err)
	}
	ApplySourceHints(context.Background(), Deps{DB: db}, jc)
	if err := db.First(&got, asset.ID).Error; err != nil {
		t.Fatalf("reload asset: %v", err)
	}
	if got.Publisher == nil || *got.Publisher != "Blink Studios" {
		t.Errorf("re-run overwrote a set publisher: %v", got.Publisher)
	}
	var tagCount int64
	if err := db.Table("asset_tags").Where("asset_id = ?", asset.ID).Count(&tagCount).Error; err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if tagCount != 1 {
		t.Errorf("re-run left %d tag links, want 1", tagCount)
	}
}

func TestApplySourceHints_IgnoresUnsourcedVersions(t *testing.T) {
	db := enrichTestDB(t)
	asset := seedSourceAsset(t, db, 1, "Hand Dragged")
	jc := jobContext{asset: *asset, version: models.AssetVersion{AssetID: asset.ID}, libraryID: 1}

	ApplySourceHints(context.Background(), Deps{DB: db}, jc)

	var got models.Asset
	if err := db.First(&got, asset.ID).Error; err != nil {
		t.Fatalf("reload asset: %v", err)
	}
	if got.Publisher != nil {
		t.Errorf("Publisher = %v, want nil", got.Publisher)
	}
}

func TestSourceMatches(t *testing.T) {
	db := enrichTestDB(t)
	asset := seedSourceAsset(t, db, 1, "RPG Builder")
	other := seedSourceAsset(t, db, 2, "RPG Builder")

	path := "Blink/Editor Extensions/RPG Builder.unitypackage"
	older := models.AssetVersion{
		AssetID: asset.ID, ImportDate: mustTime(t, "2026-01-01T00:00:00Z"),
		SourceKind: ptr(SourceKindUnityCache), SourceRelativePath: ptr(path), SourceSizeBytes: ptr(int64(100)),
	}
	newer := models.AssetVersion{
		AssetID: asset.ID, ImportDate: mustTime(t, "2026-06-01T00:00:00Z"),
		SourceKind: ptr(SourceKindUnityCache), SourceRelativePath: ptr(path), SourceSizeBytes: ptr(int64(200)),
	}
	crossLibrary := models.AssetVersion{
		AssetID: other.ID, ImportDate: mustTime(t, "2026-07-01T00:00:00Z"),
		SourceKind: ptr(SourceKindUnityCache), SourceRelativePath: ptr(path), SourceSizeBytes: ptr(int64(999)),
	}
	for _, v := range []*models.AssetVersion{&older, &newer, &crossLibrary} {
		if err := db.Create(v).Error; err != nil {
			t.Fatalf("seed version: %v", err)
		}
	}

	got, err := SourceMatches(db, 1, SourceKindUnityCache, []string{path, "Nobody/Has/This.unitypackage"})
	if err != nil {
		t.Fatalf("SourceMatches: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("SourceMatches returned %d entries, want 1: %#v", len(got), got)
	}
	m := got[path]
	if m.AssetID != asset.ID {
		t.Errorf("AssetID = %d, want %d (library 2's copy must not leak)", m.AssetID, asset.ID)
	}
	if m.SizeBytes != 200 {
		t.Errorf("SizeBytes = %d, want 200 (the newest version wins)", m.SizeBytes)
	}
	if m.VersionID != newer.ID {
		t.Errorf("VersionID = %d, want %d", m.VersionID, newer.ID)
	}

	// A different source kind on the same path is not a match.
	none, err := SourceMatches(db, 1, "dropbox", []string{path})
	if err != nil {
		t.Fatalf("SourceMatches: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("a different source_kind matched: %#v", none)
	}

	empty, err := SourceMatches(db, 1, SourceKindUnityCache, nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("empty manifest: got %#v, %v", empty, err)
	}
}

func TestExactMatchCandidate_PublisherHint(t *testing.T) {
	cands := []metadata.Candidate{
		{Title: "RPG Builder", Publisher: "Someone Else", ListingURL: "https://x/wrong"},
		{Title: "RPG Builder", Publisher: "Blink", ListingURL: "https://x/right"},
	}
	got, exact := exactMatchCandidate("RPG Builder", "Blink", cands)
	if got == nil || got.ListingURL != "https://x/right" {
		t.Errorf("publisher did not break the tie: %#v", got)
	}
	if !exact {
		t.Error("a title+publisher match should report exact")
	}

	// With no publisher known, behaviour is exactly as it was before the hint.
	got, exact = exactMatchCandidate("RPG Builder", "", cands)
	if got == nil || got.ListingURL != "https://x/wrong" {
		t.Errorf("no-publisher case should take the first title match: %#v", got)
	}
	if !exact {
		t.Error("a title match with no publisher should still report exact")
	}

	// A near title qualifies only on an exact publisher match, and is not exact.
	near := []metadata.Candidate{
		{Title: "DOTween HOTween v2 Pro Edition", Publisher: "Demigiant", ListingURL: "https://x/near"},
	}
	got, exact = exactMatchCandidate("DOTween HOTween v2", "Demigiant", near)
	if got == nil || got.ListingURL != "https://x/near" {
		t.Errorf("publisher-assisted near match not found: %#v", got)
	}
	if exact {
		t.Error("a near match must not report exact")
	}
	if got, _ := exactMatchCandidate("DOTween HOTween v2", "Someone Else", near); got != nil {
		t.Errorf("near match applied without a publisher match: %#v", got)
	}
}

func TestTitlesNear(t *testing.T) {
	if !titlesNear("DOTween HOTween", "DOTween HOTween Pro") {
		t.Error("a whole-word prefix should be near")
	}
	if titlesNear("Builder", "RPG Builder") {
		t.Error("a suffix must not be near — that is the loose match that misfires")
	}
	if titlesNear("Trees", "Trees and Rocks Mega Pack") {
		t.Error("a short generic name must not reach a whole catalogue")
	}
	if titlesNear("Fantasy Pack", "Fantasy Pack") {
		t.Error("identical titles are an exact match, not a near one")
	}
	if titlesNear("", "Anything At All") {
		t.Error("a blank name must never be near")
	}
}

func TestValidateSourceErrorIsSpecific(t *testing.T) {
	_, err := ValidateSource("dropbox", "a/b.unitypackage")
	if !errors.Is(err, ErrUnknownSourceKind) {
		t.Errorf("err = %v, want ErrUnknownSourceKind", err)
	}
}
