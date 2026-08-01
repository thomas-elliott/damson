package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imports"
)

const cachePath = "Demigiant/ScriptingAnimation/DOTween HOTween v2.unitypackage"

func postCacheImport(t *testing.T, h *Handler, u *models.User, filename string, fields map[string]string) *models.AssetVersion {
	t.Helper()
	pkg := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x21), pathname: "Assets/x.bin", content: []byte("xyz")},
	})
	body, ctype := buildMultipartWithFields(t, "file", filename, pkg, fields)
	w := postMultipart(setupRouter(h, u), "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)

	var v models.AssetVersion
	if err := h.db.Where("asset_id = ?", resp.AssetID).First(&v).Error; err != nil {
		t.Fatalf("load version: %v", err)
	}
	return &v
}

func TestPostImport_RecordsSourceFields(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "src@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	v := postCacheImport(t, h, u, "DOTween HOTween v2.unitypackage", map[string]string{
		"source_kind":          imports.SourceKindUnityCache,
		"source_relative_path": cachePath,
	})

	if v.SourceKind == nil || *v.SourceKind != imports.SourceKindUnityCache {
		t.Errorf("SourceKind = %v, want %q", v.SourceKind, imports.SourceKindUnityCache)
	}
	if v.SourceRelativePath == nil || *v.SourceRelativePath != cachePath {
		t.Errorf("SourceRelativePath = %v, want %q", v.SourceRelativePath, cachePath)
	}
	if v.SourceSizeBytes == nil || *v.SourceSizeBytes <= 0 {
		t.Errorf("SourceSizeBytes = %v, want the stored blob size", v.SourceSizeBytes)
	}
}

// The cache carries no version information, so a "v2" in a product name must
// not become a store version — store_version is a per-asset uniqueness key, so
// a phantom there blocks the real version later.
func TestPostImport_UnityCacheSuppressesFilenameVersion(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "ver@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	cached := postCacheImport(t, h, u, "DOTween HOTween v2.unitypackage", map[string]string{
		"source_kind":          imports.SourceKindUnityCache,
		"source_relative_path": cachePath,
	})
	if cached.StoreVersion != "" {
		t.Errorf("cache import StoreVersion = %q, want blank", cached.StoreVersion)
	}

	// Without the source hint the shipped filename parser still applies.
	plain := postCacheImport(t, h, u, "Other Pack v2.unitypackage", nil)
	if plain.StoreVersion != "2" {
		t.Errorf("plain import StoreVersion = %q, want 2", plain.StoreVersion)
	}

	// An explicit store_version still wins over the suppression.
	explicit := postCacheImport(t, h, u, "Third Pack v2.unitypackage", map[string]string{
		"source_kind":          imports.SourceKindUnityCache,
		"source_relative_path": "Pub/Audio/Third Pack v2.unitypackage",
		"store_version":        "3.1.0",
	})
	if explicit.StoreVersion != "3.1.0" {
		t.Errorf("explicit StoreVersion = %q, want 3.1.0", explicit.StoreVersion)
	}
}

func TestPostImport_RejectsBadSource(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "bad@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	cases := []struct {
		name   string
		fields map[string]string
	}{
		{"absolute path", map[string]string{
			"source_kind":          imports.SourceKindUnityCache,
			"source_relative_path": "/etc/passwd.unitypackage",
		}},
		{"traversal", map[string]string{
			"source_kind":          imports.SourceKindUnityCache,
			"source_relative_path": "Pub/../../etc/Pack.unitypackage",
		}},
		{"newline", map[string]string{
			"source_kind":          imports.SourceKindUnityCache,
			"source_relative_path": "Pub/Au\ndio/Pack.unitypackage",
		}},
		{"unknown kind", map[string]string{
			"source_kind":          "dropbox",
			"source_relative_path": "Pub/Audio/Pack.unitypackage",
		}},
		{"path without kind", map[string]string{
			"source_relative_path": "Pub/Audio/Pack.unitypackage",
		}},
		{"kind without path", map[string]string{
			"source_kind": imports.SourceKindUnityCache,
		}},
	}

	pkg := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x22), pathname: "Assets/y.bin", content: []byte("y")},
	})
	for _, c := range cases {
		body, ctype := buildMultipartWithFields(t, "file", "Pack.unitypackage", pkg, c.fields)
		w := postMultipart(setupRouter(h, u), "/api/imports", body, ctype)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", c.name, w.Code)
		}
	}

	// A rejected hint must not leave a stub asset behind.
	var assets int64
	if err := db.Model(&models.Asset{}).Count(&assets).Error; err != nil {
		t.Fatalf("count assets: %v", err)
	}
	if assets != 0 {
		t.Errorf("rejected imports created %d assets, want 0", assets)
	}
}

func TestPostImport_NoSourceFieldsLeavesColumnsNull(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "plain@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	v := postCacheImport(t, h, u, "Hand Dragged.unitypackage", nil)
	if v.SourceKind != nil || v.SourceRelativePath != nil || v.SourceSizeBytes != nil {
		t.Errorf("hand-dragged upload wrote source columns: %v %v %v",
			v.SourceKind, v.SourceRelativePath, v.SourceSizeBytes)
	}
}
