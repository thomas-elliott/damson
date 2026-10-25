package api

import (
	"context"
	"io"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/ingest"
	"github.com/thomas-elliott/damson/internal/ingest/ingesttest"
	"github.com/thomas-elliott/damson/internal/preview"
	"github.com/thomas-elliott/damson/internal/preview/previewtest"
	"github.com/thomas-elliott/damson/internal/storage"
)

type metaRecordingStorage struct {
	*storage.LocalFS
	lastOriginalMeta storage.ObjectMeta
	lastBlobMeta     storage.ObjectMeta
}

func (m *metaRecordingStorage) Put(ctx context.Context, prefix storage.Prefix, r io.Reader, meta storage.ObjectMeta) (string, int64, error) {
	switch prefix {
	case storage.Originals:
		m.lastOriginalMeta = meta
	case storage.Blobs:
		m.lastBlobMeta = meta
	}
	return m.LocalFS.Put(ctx, prefix, r, meta)
}

func newRecordingHandler(t *testing.T) (*Handler, *metaRecordingStorage, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	lfs, err := storage.NewLocalFS(t.TempDir())
	if err != nil {
		t.Fatalf("new local storage: %v", err)
	}
	store := &metaRecordingStorage{LocalFS: lfs}
	bc := ingest.NewBroadcaster()
	pbc := preview.NewBroadcaster()
	previewPool := previewtest.NewSync(preview.Deps{DB: db, Storage: store, Broadcaster: pbc})
	pool := ingesttest.NewSync(ingest.Deps{DB: db, Storage: store, Broadcaster: bc, Previews: previewPool})
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
