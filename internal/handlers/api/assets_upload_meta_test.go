package api

import (
	"context"
	"io"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/preview"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

type metaRecordingStorage struct {
	*storage.LocalFS
	lastOriginalMeta storage.ObjectMeta
	lastBlobMeta     storage.ObjectMeta
}

func (m *metaRecordingStorage) PutOriginal(ctx context.Context, r io.Reader, meta storage.ObjectMeta) (string, int64, error) {
	m.lastOriginalMeta = meta
	return m.LocalFS.PutOriginal(ctx, r, meta)
}

func (m *metaRecordingStorage) PutBlob(ctx context.Context, r io.Reader, meta storage.ObjectMeta) (string, int64, error) {
	m.lastBlobMeta = meta
	return m.LocalFS.PutBlob(ctx, r, meta)
}

func newRecordingHandler(t *testing.T) (*Handler, *metaRecordingStorage, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	lfs, err := storage.NewLocalFS(t.TempDir(), "http://localhost:8080")
	if err != nil {
		t.Fatalf("new local storage: %v", err)
	}
	store := &metaRecordingStorage{LocalFS: lfs}
	bc := imports.NewBroadcaster()
	pbc := preview.NewBroadcaster()
	previewPool := preview.NewSync(preview.Deps{DB: db, Storage: store, Broadcaster: pbc})
	pool := imports.NewSync(imports.Deps{DB: db, Storage: store, Broadcaster: bc, Previews: previewPool})
	h := New(Deps{
		DB:                 db,
		AuthMode:           config.AuthModeOIDC,
		Storage:            store,
		ImportPool:         pool,
		Broadcaster:        bc,
		PreviewPool:        previewPool,
		PreviewBroadcaster: pbc,
		OIDCAuthority:      "https://auth.example.com",
		OIDCClientID:       "damson",
	})
	return h, store, db
}

func TestPostImport_StampsMetadata(t *testing.T) {
	h, store, db := newRecordingHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	// Unity → PutOriginal carries filename + all owning IDs + kind.
	pkg := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x06), pathname: "Assets/x.bin", content: []byte("x")},
	})
	body, ctype := buildMultipart(t, "file", "Kit.unitypackage", pkg)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)

	got := store.lastOriginalMeta
	if got.Filename != "Kit.unitypackage" {
		t.Fatalf("original meta filename: got %q", got.Filename)
	}
	if got.AssetID != resp.AssetID {
		t.Fatalf("original meta asset id: got %d, want %d", got.AssetID, resp.AssetID)
	}
	if got.VersionID == 0 || got.LibraryID == 0 {
		t.Fatalf("original meta missing version/library id: %+v", got)
	}
	if got.Kind != "unitypackage" {
		t.Fatalf("original meta kind: got %q", got.Kind)
	}

	// Loose → PutBlob carries filename + owning IDs + loose kind.
	lb, lc := buildMultipart(t, "file", "logo.png", []byte("png"))
	lw := postMultipart(r, "/api/imports", lb, lc)
	requireStatus(t, lw, http.StatusAccepted)
	lresp := jsonResponse[importEnqueueResponse](t, lw)

	lm := store.lastBlobMeta
	if lm.Filename != "logo.png" || lm.Kind != "loose" {
		t.Fatalf("loose meta filename/kind: %+v", lm)
	}
	if lm.AssetID != lresp.AssetID || lm.VersionID == 0 || lm.LibraryID == 0 {
		t.Fatalf("loose meta missing ids: %+v", lm)
	}
}
