package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// buildUnityPackage assembles a minimal .unitypackage-shaped tar.gz from the
// given (guid, pathname, content, preview) tuples. A zero-length `preview`
// means no preview.png entry is emitted for that GUID.
func buildUnityPackage(t *testing.T, entries []unityPkgEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	write := func(name string, data []byte) {
		hdr := &tar.Header{
			Name:     name,
			Size:     int64(len(data)),
			Mode:     0o644,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range entries {
		write(e.guid+"/pathname", []byte(e.pathname))
		write(e.guid+"/asset", e.content)
		if len(e.preview) > 0 {
			write(e.guid+"/preview.png", e.preview)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type unityPkgEntry struct {
	guid     string
	pathname string
	content  []byte
	preview  []byte
}

func testGUID(b byte) string {
	return hex.EncodeToString(bytes.Repeat([]byte{b}, 16))
}

func TestPostImport_UnityPackage_ExtractsFiles(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	pkg := buildUnityPackage(t, []unityPkgEntry{
		{
			guid:     testGUID(0x01),
			pathname: "Assets/Textures/grass.png",
			content:  []byte("grass-bytes"),
			preview:  []byte("\x89PNG\r\n\x1a\nfakepreview"),
		},
		{
			guid:     testGUID(0x02),
			pathname: "Assets/Models/Tree.fbx",
			content:  []byte("fbx-bytes"),
		},
	})

	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "Environment Kit.unitypackage", pkg)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)

	resp := jsonResponse[importEnqueueResponse](t, w)
	if resp.AssetID == 0 || resp.ImportID == 0 {
		t.Fatalf("ids missing in response: %+v", resp)
	}

	// Sync test pool runs the pipeline inline, so by now extract + thumbnail
	// + finalize are all done.
	var imp models.AssetImport
	if err := db.First(&imp, resp.ImportID).Error; err != nil {
		t.Fatal(err)
	}
	if imp.Stage != "done" {
		t.Fatalf("import stage: got %q, want done; err=%q", imp.Stage, imp.Error)
	}

	var asset models.Asset
	if err := db.First(&asset, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.CurrentVersionID == nil {
		t.Fatalf("current_version_id not set on asset: %+v", asset)
	}
	versionID := *asset.CurrentVersionID

	// Each extracted file becomes an asset_files row with the Unity GUID set.
	var files []models.AssetFile
	if err := db.Where("asset_version_id = ?", versionID).
		Order("relative_path asc").Find(&files).Error; err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 asset_files, got %d", len(files))
	}
	for _, f := range files {
		if f.UnityGUID == "" {
			t.Fatalf("unity_guid missing on %+v", f)
		}
		if f.ContentHash == "" || f.ContentHash != f.ObjectKey {
			t.Fatalf("hash/key: %+v", f)
		}
	}
	if files[0].RelativePath != "Assets/Models/Tree.fbx" ||
		files[1].RelativePath != "Assets/Textures/grass.png" {
		t.Fatalf("unexpected paths: %+v %+v", files[0], files[1])
	}

	// Version retains the archive so re-extraction is possible later.
	var version models.AssetVersion
	if err := db.First(&version, versionID).Error; err != nil {
		t.Fatal(err)
	}
	if version.PackageKind != "unitypackage" || version.OriginalPackageObjectKey == "" {
		t.Fatalf("version not flagged as unitypackage: %+v", version)
	}

	// Preview harvested → asset.thumbnail_object_key points into thumbs/.
	if asset.ThumbnailObjectKey == "" || asset.ThumbnailContentType != "image/png" {
		t.Fatalf("preview not harvested onto asset: %+v", asset)
	}
}

func TestUploadRouting_UnityToOriginals_LooseToBlobs(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)
	ctx := context.Background()

	// A Unity archive lands under originals/, never in the blob pool.
	pkg := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x05), pathname: "Assets/a.bin", content: []byte("a")},
	})
	body, ctype := buildMultipart(t, "file", "Kit.unitypackage", pkg)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	unityResp := jsonResponse[importEnqueueResponse](t, w)

	var uv models.AssetVersion
	if err := db.Where("asset_id = ?", unityResp.AssetID).First(&uv).Error; err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := h.storage.HeadOriginal(ctx, uv.OriginalPackageObjectKey); !ok {
		t.Fatalf("unity archive not stored under originals/")
	}
	if _, ok, _ := h.storage.HeadBlob(ctx, uv.OriginalPackageObjectKey); ok {
		t.Fatalf("unity archive leaked into blobs/ — routing regressed to PutBlob")
	}

	// A loose upload's content stays in blobs/ and is absent from originals/.
	lbody, lctype := buildMultipart(t, "file", "logo.png", []byte("png-bytes"))
	lw := postMultipart(r, "/api/imports", lbody, lctype)
	requireStatus(t, lw, http.StatusAccepted)
	looseResp := jsonResponse[importEnqueueResponse](t, lw)

	var lv models.AssetVersion
	if err := db.Where("asset_id = ?", looseResp.AssetID).First(&lv).Error; err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := h.storage.HeadBlob(ctx, lv.OriginalPackageObjectKey); !ok {
		t.Fatalf("loose content not stored under blobs/")
	}
	if _, ok, _ := h.storage.HeadOriginal(ctx, lv.OriginalPackageObjectKey); ok {
		t.Fatalf("loose content leaked into originals/")
	}
}

func TestPostImport_SuggestsMatchForSameName(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	pkg1 := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x41), pathname: "Assets/a.bin", content: []byte("a")},
	})
	b1, ct1 := buildMultipart(t, "file", "Fantasy Pack 1.0.unitypackage", pkg1)
	w1 := postMultipart(r, "/api/imports", b1, ct1)
	requireStatus(t, w1, http.StatusAccepted)
	first := jsonResponse[importEnqueueResponse](t, w1)
	// First upload of a fresh name has nothing to match against.
	if first.SuggestedMatch != nil {
		t.Fatalf("unexpected match on first upload: %+v", first.SuggestedMatch)
	}

	// A second same-named upload (defaulting to a new asset) surfaces the
	// first as a suggested match for the "add as a version of…" prompt.
	pkg2 := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x42), pathname: "Assets/b.bin", content: []byte("b")},
	})
	b2, ct2 := buildMultipart(t, "file", "Fantasy Pack 2.0.unitypackage", pkg2)
	w2 := postMultipart(r, "/api/imports", b2, ct2)
	requireStatus(t, w2, http.StatusAccepted)
	second := jsonResponse[importEnqueueResponse](t, w2)
	if second.SuggestedMatch == nil || second.SuggestedMatch.AssetID != first.AssetID {
		t.Fatalf("expected suggested match to point at the first asset, got %+v", second.SuggestedMatch)
	}
	// Default behaviour is unchanged: a distinct new asset was still created.
	if second.AssetID == first.AssetID {
		t.Fatalf("match detection must not attach; a new asset should be created")
	}
	var count int64
	db.Model(&models.Asset{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 assets, got %d", count)
	}
}

func TestPostImport_AddsVersionToExistingAsset(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	// First upload creates the asset (store version 1.0).
	pkg1 := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x21), pathname: "Assets/a.bin", content: []byte("a")},
	})
	b1, ct1 := buildMultipart(t, "file", "Weapon System 1.0.unitypackage", pkg1)
	w1 := postMultipart(r, "/api/imports", b1, ct1)
	requireStatus(t, w1, http.StatusAccepted)
	first := jsonResponse[importEnqueueResponse](t, w1)

	// Second upload targets that asset — a new version, not a new asset.
	pkg2 := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x22), pathname: "Assets/b.bin", content: []byte("b")},
	})
	b2, ct2 := buildMultipartWithFields(t, "file", "Weapon System 2.0.unitypackage", pkg2,
		map[string]string{"asset_id": uintStr(first.AssetID)})
	w2 := postMultipart(r, "/api/imports", b2, ct2)
	requireStatus(t, w2, http.StatusAccepted)
	second := jsonResponse[importEnqueueResponse](t, w2)

	if second.AssetID != first.AssetID {
		t.Fatalf("upload should attach to the target asset: got %d, want %d", second.AssetID, first.AssetID)
	}
	var assetCount int64
	db.Model(&models.Asset{}).Count(&assetCount)
	if assetCount != 1 {
		t.Fatalf("expected 1 asset, got %d (a new asset was created)", assetCount)
	}
	var versionCount int64
	db.Model(&models.AssetVersion{}).Where("asset_id = ?", first.AssetID).Count(&versionCount)
	if versionCount != 2 {
		t.Fatalf("expected 2 versions on the asset, got %d", versionCount)
	}
	var stores []string
	db.Model(&models.AssetVersion{}).Where("asset_id = ?", first.AssetID).Order("store_version").Pluck("store_version", &stores)
	if len(stores) != 2 || stores[0] != "1.0" || stores[1] != "2.0" {
		t.Fatalf("store versions: got %v", stores)
	}
}

func TestPostImport_DuplicateStoreVersionOnTargetBlocked(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	pkg1 := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x31), pathname: "Assets/a.bin", content: []byte("a")},
	})
	b1, ct1 := buildMultipart(t, "file", "Pack 1.0.unitypackage", pkg1)
	w1 := postMultipart(r, "/api/imports", b1, ct1)
	requireStatus(t, w1, http.StatusAccepted)
	first := jsonResponse[importEnqueueResponse](t, w1)

	// A second upload with the same store version is rejected, and the target
	// asset survives intact (the failure must not wipe it).
	pkg2 := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x32), pathname: "Assets/b.bin", content: []byte("b")},
	})
	b2, ct2 := buildMultipartWithFields(t, "file", "Pack 1.0.unitypackage", pkg2,
		map[string]string{"asset_id": uintStr(first.AssetID)})
	w2 := postMultipart(r, "/api/imports", b2, ct2)
	requireStatus(t, w2, http.StatusConflict)

	var assetCount, versionCount int64
	db.Model(&models.Asset{}).Count(&assetCount)
	db.Model(&models.AssetVersion{}).Where("asset_id = ?", first.AssetID).Count(&versionCount)
	if assetCount != 1 || versionCount != 1 {
		t.Fatalf("target asset must survive a rejected duplicate: assets=%d versions=%d", assetCount, versionCount)
	}
}

func TestPostImport_VersionFromFilenameNotAutoLabel(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	pkg := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x05), pathname: "Assets/foo.bin", content: []byte("foo")},
	})
	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "Weapon System v3.4.unitypackage", pkg)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)

	var version models.AssetVersion
	if err := db.Where("asset_id = ?", resp.AssetID).First(&version).Error; err != nil {
		t.Fatal(err)
	}
	// The store version is parsed from the filename; the label is left blank
	// (no auto v1) — a no-op regression to "v1" fails the second assertion.
	if version.StoreVersion != "3.4" {
		t.Fatalf("store_version: got %q, want 3.4", version.StoreVersion)
	}
	if version.VersionLabel != "" {
		t.Fatalf("version_label should be blank, got %q", version.VersionLabel)
	}
}

func TestPostReExtract_UpdatesVersionInPlace(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	pkg := buildUnityPackage(t, []unityPkgEntry{
		{
			guid:     testGUID(0x03),
			pathname: "Assets/foo.bin",
			content:  []byte("foo"),
		},
	})
	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "Pack.unitypackage", pkg)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	first := jsonResponse[importEnqueueResponse](t, w)

	var firstAsset models.Asset
	if err := db.First(&firstAsset, first.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if firstAsset.CurrentVersionID == nil {
		t.Fatalf("first import did not set current_version_id")
	}
	firstVersionID := *firstAsset.CurrentVersionID

	var versionsBefore int64
	db.Model(&models.AssetVersion{}).Where("asset_id = ?", first.AssetID).Count(&versionsBefore)

	// Re-extract using the retained original package.
	path := "/api/assets/" + uintStr(first.AssetID) +
		"/versions/" + uintStr(firstVersionID) + "/reextract"
	w2 := performRequest(r, http.MethodPost, path, nil)
	requireStatus(t, w2, http.StatusCreated)
	second := jsonResponse[importResponse](t, w2)

	// Re-extract updates the same version in place — no new row, same id.
	if second.AssetVersionID != firstVersionID {
		t.Fatalf("re-extract should reuse the version id: got %d, want %d", second.AssetVersionID, firstVersionID)
	}
	if second.FileCount != 1 {
		t.Fatalf("expected 1 file, got %d", second.FileCount)
	}
	var versionsAfter int64
	db.Model(&models.AssetVersion{}).Where("asset_id = ?", first.AssetID).Count(&versionsAfter)
	if versionsAfter != versionsBefore {
		t.Fatalf("re-extract created a new version row: before=%d after=%d", versionsBefore, versionsAfter)
	}

	// The version keeps exactly its re-extracted files (no duplication).
	var files int64
	if err := db.Model(&models.AssetFile{}).
		Where("asset_version_id = ?", firstVersionID).
		Count(&files).Error; err != nil {
		t.Fatal(err)
	}
	if files != 1 {
		t.Fatalf("expected 1 file after in-place re-extract, got %d", files)
	}

	var asset models.Asset
	if err := db.First(&asset, first.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.CurrentVersionID == nil || *asset.CurrentVersionID != firstVersionID {
		t.Fatalf("current_version_id should remain the re-extracted version: %+v", asset.CurrentVersionID)
	}
}

// importTwoVersions imports "Pack 2.0" and then, into the same asset, "Pack
// 1.0", returning the asset and the two versions' ids.
func importTwoVersions(t *testing.T, r http.Handler, db *gorm.DB) (assetID, v2, v1 uint) {
	t.Helper()
	pkg2 := buildUnityPackage(t, []unityPkgEntry{{guid: testGUID(0x41), pathname: "Assets/a.bin", content: []byte("a")}})
	b2, ct2 := buildMultipart(t, "file", "Pack 2.0.unitypackage", pkg2)
	w2 := postMultipart(r, "/api/imports", b2, ct2)
	requireStatus(t, w2, http.StatusAccepted)
	assetID = jsonResponse[importEnqueueResponse](t, w2).AssetID

	pkg1 := buildUnityPackage(t, []unityPkgEntry{{guid: testGUID(0x42), pathname: "Assets/b.bin", content: []byte("b")}})
	b1, ct1 := buildMultipartWithFields(t, "file", "Pack 1.0.unitypackage", pkg1,
		map[string]string{"asset_id": uintStr(assetID)})
	w1 := postMultipart(r, "/api/imports", b1, ct1)
	requireStatus(t, w1, http.StatusAccepted)

	var versions []models.AssetVersion
	if err := db.Where("asset_id = ?", assetID).Find(&versions).Error; err != nil {
		t.Fatal(err)
	}
	for _, v := range versions {
		switch v.StoreVersion {
		case "2.0":
			v2 = v.ID
		case "1.0":
			v1 = v.ID
		}
	}
	if v2 == 0 || v1 == 0 {
		t.Fatalf("versions after two imports: %+v", versions)
	}
	return assetID, v2, v1
}

func requireCurrentVersion(t *testing.T, db *gorm.DB, assetID, want uint) {
	t.Helper()
	var asset models.Asset
	if err := db.First(&asset, assetID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.CurrentVersionID == nil || *asset.CurrentVersionID != want {
		t.Fatalf("current version = %v, want %d", asset.CurrentVersionID, want)
	}
}

// Importing an older store version after a newer one leaves the newer one
// current, though the older one was imported last.
func TestPostImport_OlderStoreVersionLeavesNewerCurrent(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	assetID, v2, _ := importTwoVersions(t, r, db)

	requireCurrentVersion(t, db, assetID, v2)
}

// Re-extracting an older version rewrites its files without making it
// current.
func TestPostReExtract_OlderVersionDoesNotBecomeCurrent(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)
	assetID, v2, v1 := importTwoVersions(t, r, db)

	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(assetID)+"/versions/"+uintStr(v1)+"/reextract", nil)
	requireStatus(t, w, http.StatusCreated)

	requireCurrentVersion(t, db, assetID, v2)
}

// A version whose import failed after committing its files keeps them, so it
// stays current when another version changes.
func TestPostReExtract_VersionFailedAfterFinalizeStaysCurrent(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)
	assetID, v2, v1 := importTwoVersions(t, r, db)
	if err := db.Model(&models.AssetImport{}).Where("asset_version_id = ?", v2).Update("stage", "failed").Error; err != nil {
		t.Fatal(err)
	}

	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(assetID)+"/versions/"+uintStr(v1)+"/reextract", nil)
	requireStatus(t, w, http.StatusCreated)

	requireCurrentVersion(t, db, assetID, v2)
}

// A version whose import failed before committing its files never becomes
// current, even when it has the highest store version.
func TestPostReExtract_VersionFailedBeforeFinalizeIsPassedOver(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	pkg := buildUnityPackage(t, []unityPkgEntry{{guid: testGUID(0x43), pathname: "Assets/c.bin", content: []byte("c")}})
	b1, ct1 := buildMultipart(t, "file", "Pack 1.0.unitypackage", pkg)
	w := postMultipart(r, "/api/imports", b1, ct1)
	requireStatus(t, w, http.StatusAccepted)
	assetID := jsonResponse[importEnqueueResponse](t, w).AssetID
	var v1 models.AssetVersion
	if err := db.Where("asset_id = ?", assetID).First(&v1).Error; err != nil {
		t.Fatal(err)
	}
	b2, ct2 := buildMultipartWithFields(t, "file", "Pack 2.0.unitypackage", []byte("not a package"),
		map[string]string{"asset_id": uintStr(assetID)})
	requireStatus(t, postMultipart(r, "/api/imports", b2, ct2), http.StatusAccepted)

	w = performRequest(r, http.MethodPost, "/api/assets/"+uintStr(assetID)+"/versions/"+uintStr(v1.ID)+"/reextract", nil)
	requireStatus(t, w, http.StatusCreated)

	requireCurrentVersion(t, db, assetID, v1.ID)
}

func TestPostReExtract_RejectsNonUnityVersion(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	r := setupRouter(h, u)
	// A loose upload has no original package retained.
	body, ctype := buildMultipart(t, "file", "logo.png", []byte("png"))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)

	var asset models.Asset
	if err := db.First(&asset, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.CurrentVersionID == nil {
		t.Fatalf("loose import did not finalize a version")
	}
	path := "/api/assets/" + uintStr(resp.AssetID) +
		"/versions/" + uintStr(*asset.CurrentVersionID) + "/reextract"
	w2 := performRequest(r, http.MethodPost, path, nil)
	requireStatus(t, w2, http.StatusConflict)
}

func TestPostReExtract_NotFoundInOtherLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	// Seed an asset in an unrelated library.
	otherLib := seedLibrary(t, db, "Other")
	stranger := models.Asset{LibraryID: otherLib.ID, Name: "stranger"}
	if err := db.Create(&stranger).Error; err != nil {
		t.Fatal(err)
	}
	strangerVer := models.AssetVersion{
		AssetID:                  stranger.ID,
		VersionLabel:             "v1",
		PackageKind:              "unitypackage",
		OriginalPackageObjectKey: "deadbeef",
	}
	if err := db.Create(&strangerVer).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	path := "/api/assets/" + uintStr(stranger.ID) +
		"/versions/" + uintStr(strangerVer.ID) + "/reextract"
	w := performRequest(r, http.MethodPost, path, nil)
	requireStatus(t, w, http.StatusNotFound)
}

func uintStr(u uint) string {
	return strconv.FormatUint(uint64(u), 10)
}

// Re-extract rewrites a version's files, which resets their preview state;
// it must queue fresh previews as an import does, or the version is left
// with none and nothing queued.
func TestPostReExtract_QueuesFreshPreviews(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"><rect width="8" height="8" fill="red"/></svg>`)
	pkg := buildUnityPackage(t, []unityPkgEntry{{guid: testGUID(0x07), pathname: "Assets/icon.svg", content: svg}})
	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "Icons.unitypackage", pkg)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	imp := jsonResponse[importEnqueueResponse](t, w)

	var asset models.Asset
	if err := db.First(&asset, imp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	path := "/api/assets/" + uintStr(asset.ID) + "/versions/" + uintStr(*asset.CurrentVersionID) + "/reextract"
	requireStatus(t, performRequest(r, http.MethodPost, path, nil), http.StatusCreated)

	var f models.AssetFile
	if err := db.Where("asset_version_id = ?", *asset.CurrentVersionID).First(&f).Error; err != nil {
		t.Fatal(err)
	}
	if f.PreviewStatus != "done" || f.PreviewObjectKey == "" {
		t.Fatalf("after re-extract the SVG's preview is %q (key %q), want done", f.PreviewStatus, f.PreviewObjectKey)
	}
}

// The Unity preview setting governs the cover harvested from a package's
// preview.png files: off, the package gets none (the store cover or the
// placeholder applies); on, it does. A re-extract runs the import's own stages,
// so it follows the setting too.
func TestUnityPreviewSetting_ControlsTheCoverOnImportAndReextract(t *testing.T) {
	for _, on := range []bool{true, false} {
		t.Run(fmt.Sprintf("unity preview %v", on), func(t *testing.T) {
			h, db := newTestHandler(t)
			u := seedUser(t, db, "librarian@example.com")
			lib := seedLibraryAndSwitch(t, h, u)
			s := models.LibrarySettings{LibraryID: lib.ID}
			if err := db.Create(&s).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&s).Update("thumbnails_unity_preview", on).Error; err != nil {
				t.Fatal(err)
			}

			pkg := buildUnityPackage(t, []unityPkgEntry{{
				guid: testGUID(0x09), pathname: "Assets/Textures/grass.png",
				content: makeTestPNG(t, 16, 16), preview: makeTestPNG(t, 8, 8),
			}})
			body, ctype := buildMultipart(t, "file", "Grass.unitypackage", pkg)
			r := setupRouter(h, u)
			w := postMultipart(r, "/api/imports", body, ctype)
			requireStatus(t, w, http.StatusAccepted)

			var a models.Asset
			if err := db.First(&a, jsonResponse[importEnqueueResponse](t, w).AssetID).Error; err != nil {
				t.Fatal(err)
			}
			if got := a.ThumbnailObjectKey != ""; got != on {
				t.Fatalf("import: cover set = %v (key %q), want %v", got, a.ThumbnailObjectKey, on)
			}

			if err := db.Model(&a).Update("thumbnail_object_key", "").Error; err != nil {
				t.Fatal(err)
			}
			w = performRequest(r, http.MethodPost, "/api/assets/"+uintStr(a.ID)+"/versions/"+uintStr(*a.CurrentVersionID)+"/reextract", nil)
			requireStatus(t, w, http.StatusCreated)
			if err := db.First(&a, a.ID).Error; err != nil {
				t.Fatal(err)
			}
			if got := a.ThumbnailObjectKey != ""; got != on {
				t.Fatalf("re-extract: cover set = %v (key %q), want %v", got, a.ThumbnailObjectKey, on)
			}
		})
	}
}
