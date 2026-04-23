package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"net/http"
	"strconv"
	"testing"

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
	u := seedUser(t, db, "thomas@datch.io")
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
	requireStatus(t, w, http.StatusCreated)

	resp := jsonResponse[importResponse](t, w)
	if resp.PackageKind != "unitypackage" {
		t.Fatalf("package_kind: got %q, want %q", resp.PackageKind, "unitypackage")
	}
	if resp.FileCount != 2 {
		t.Fatalf("file_count: got %d, want 2", resp.FileCount)
	}

	// Each extracted file becomes an asset_files row with the Unity GUID set.
	var files []models.AssetFile
	if err := db.Where("asset_version_id = ?", resp.AssetVersionID).
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
	if err := db.First(&version, resp.AssetVersionID).Error; err != nil {
		t.Fatal(err)
	}
	if version.PackageKind != "unitypackage" || version.OriginalPackageObjectKey == "" {
		t.Fatalf("version not flagged as unitypackage: %+v", version)
	}

	// Preview harvested → asset.thumbnail_object_key points into thumbs/.
	var asset models.Asset
	if err := db.First(&asset, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.ThumbnailObjectKey == "" || asset.ThumbnailContentType != "image/png" {
		t.Fatalf("preview not harvested onto asset: %+v", asset)
	}
}

func TestPostReExtract_CreatesNewVersion(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
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
	requireStatus(t, w, http.StatusCreated)
	first := jsonResponse[importResponse](t, w)

	// Re-extract using the retained original package.
	path := "/api/assets/" + uintStr(first.AssetID) +
		"/versions/" + uintStr(first.AssetVersionID) + "/reextract"
	w2 := performRequest(r, http.MethodPost, path, nil)
	requireStatus(t, w2, http.StatusCreated)
	second := jsonResponse[importResponse](t, w2)

	if second.AssetID != first.AssetID {
		t.Fatalf("asset id changed: %d → %d", first.AssetID, second.AssetID)
	}
	if second.AssetVersionID == first.AssetVersionID {
		t.Fatalf("version id should differ: %d", second.AssetVersionID)
	}
	if second.FileCount != 1 {
		t.Fatalf("expected 1 file, got %d", second.FileCount)
	}

	// Old version rows are untouched.
	var firstFiles int64
	if err := db.Model(&models.AssetFile{}).
		Where("asset_version_id = ?", first.AssetVersionID).
		Count(&firstFiles).Error; err != nil {
		t.Fatal(err)
	}
	if firstFiles != 1 {
		t.Fatalf("first version lost its files: %d", firstFiles)
	}

	// Asset's current version switched to the new one; labels are v1 / v2.
	var asset models.Asset
	if err := db.First(&asset, first.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.CurrentVersionID == nil || *asset.CurrentVersionID != second.AssetVersionID {
		t.Fatalf("current_version_id not rolled forward: %+v", asset.CurrentVersionID)
	}
	var v2 models.AssetVersion
	if err := db.First(&v2, second.AssetVersionID).Error; err != nil {
		t.Fatal(err)
	}
	if v2.VersionLabel != "v2" {
		t.Fatalf("version label: got %q, want v2", v2.VersionLabel)
	}
	if v2.OriginalPackageObjectKey == "" {
		t.Fatalf("v2 did not inherit original package key")
	}
}

func TestPostReExtract_RejectsNonUnityVersion(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	_ = seedLibraryAndSwitch(t, h, u)

	r := setupRouter(h, u)
	// A loose upload has no original package retained.
	body, ctype := buildMultipart(t, "file", "logo.png", []byte("png"))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusCreated)
	resp := jsonResponse[importResponse](t, w)

	path := "/api/assets/" + uintStr(resp.AssetID) +
		"/versions/" + uintStr(resp.AssetVersionID) + "/reextract"
	w2 := performRequest(r, http.MethodPost, path, nil)
	requireStatus(t, w2, http.StatusConflict)
}

func TestPostReExtract_NotFoundInOtherLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
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
