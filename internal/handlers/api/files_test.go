package api

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/services/storage"
)

// An original downloads under the extension of the archive it is.
func TestGetFile_OriginalKeepsItsArchiveExtension(t *testing.T) {
	for kind, want := range map[string]string{
		"unitypackage": `attachment; filename="Pack.unitypackage"`,
		"zip":          `attachment; filename="Pack.zip"`,
	} {
		t.Run(kind, func(t *testing.T) {
			h, db := newTestHandler(t)
			u := seedUser(t, db, "t@example.com")
			lib := seedLibraryForUser(t, db, u, "Personal")
			key, _, err := h.storage.PutOriginal(context.Background(), bytes.NewReader([]byte(kind)), storage.ObjectMeta{})
			if err != nil {
				t.Fatal(err)
			}
			_, v := seedAssetWithVersion(t, db, lib.ID, "Pack", "v1")
			if err := db.Model(v).Updates(map[string]any{"package_kind": kind, "original_package_object_key": key}).Error; err != nil {
				t.Fatal(err)
			}

			w := performGET(setupRouter(h, u), "/api/files/"+key)
			requireStatus(t, w, http.StatusOK)
			if got := w.Header().Get("Content-Disposition"); got != want {
				t.Errorf("Content-Disposition = %q, want %q", got, want)
			}
		})
	}
}
