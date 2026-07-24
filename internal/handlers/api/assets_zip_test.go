package api

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

func buildZipArchive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPostImport_Zip_ExtractsFilesAsOneAsset(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	_ = seedLibraryAndSwitch(t, h, u)

	arc := buildZipArchive(t, map[string][]byte{
		"kit/readme.txt":     []byte("hello"),
		"kit/model.fbx":      []byte("fbx-bytes"),
		"kit/textures/a.png": []byte("\x89PNG\r\n\x1a\nfake"),
	})

	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "Loose Asset Kit.zip", arc)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)

	resp := jsonResponse[importEnqueueResponse](t, w)
	if resp.AssetID == 0 || resp.ImportID == 0 {
		t.Fatalf("ids missing in response: %+v", resp)
	}

	var imp models.AssetImport
	if err := db.First(&imp, resp.ImportID).Error; err != nil {
		t.Fatal(err)
	}
	if imp.Stage != "done" {
		t.Fatalf("import stage: got %q, want done; err=%q", imp.Stage, imp.Error)
	}

	// One coherent asset — not one per loose file.
	var assetCount int64
	db.Model(&models.Asset{}).Count(&assetCount)
	if assetCount != 1 {
		t.Fatalf("expected exactly 1 asset for the whole folder, got %d", assetCount)
	}

	var asset models.Asset
	if err := db.First(&asset, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.CurrentVersionID == nil {
		t.Fatalf("current_version_id not set: %+v", asset)
	}
	versionID := *asset.CurrentVersionID

	// Every zip entry becomes an asset_files row under the single version.
	var files []models.AssetFile
	if err := db.Where("asset_version_id = ?", versionID).
		Order("relative_path asc").Find(&files).Error; err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 asset_files, got %d: %+v", len(files), files)
	}
	for _, f := range files {
		if f.ContentHash == "" || f.ContentHash != f.ObjectKey {
			t.Fatalf("hash/key: %+v", f)
		}
	}

	// The archive is classified zip and retained under originals/, never blobs/.
	var version models.AssetVersion
	if err := db.First(&version, versionID).Error; err != nil {
		t.Fatal(err)
	}
	if version.PackageKind != "zip" || version.OriginalPackageObjectKey == "" {
		t.Fatalf("version not flagged as zip: %+v", version)
	}
	ctx := context.Background()
	if _, ok, _ := h.storage.HeadOriginal(ctx, version.OriginalPackageObjectKey); !ok {
		t.Fatalf("zip archive not stored under originals/")
	}
	if _, ok, _ := h.storage.HeadBlob(ctx, version.OriginalPackageObjectKey); ok {
		t.Fatalf("zip archive leaked into blobs/ — routing regressed to PutBlob")
	}

	// The bundled image is harvested as the asset cover.
	if asset.ThumbnailObjectKey == "" || asset.ThumbnailContentType != "image/png" {
		t.Fatalf("cover not harvested onto asset: %+v", asset)
	}
}
