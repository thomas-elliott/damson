package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/handlers/api"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/testdb"
)

// A full-size chunk must get through the global middleware, not only through
// the handler: API tests use newTestHandler, which skips the body cap. Both
// clients authenticate differently (the SPA with a session, dmsn with a
// token), so each is driven with its own bearer.
func TestUploadChunk_FullSizeChunkPassesTheServerStack(t *testing.T) {
	db := testdb.NewPostgres(t)
	store, err := storage.NewLocalFS(t.TempDir(), "")
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	local, err := auth.NewLocalAuthenticator(db, []byte("test-signing-key-at-least-thirty-two-bytes"))
	if err != nil {
		t.Fatalf("local auth: %v", err)
	}

	lib := models.Library{Name: "Main"}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatalf("library: %v", err)
	}
	user := models.User{Email: "up@example.com", DisplayName: "Up", CurrentLibraryID: &lib.ID}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("user: %v", err)
	}
	if err := db.Create(&models.LibraryMembership{LibraryID: lib.ID, UserID: user.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("membership: %v", err)
	}
	session, _, err := local.Refresh(&user)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	secret, hash, prefix, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	tok := models.APIToken{UserID: user.ID, LibraryID: lib.ID, Label: "cli", TokenHash: hash, Prefix: prefix, Scope: auth.ScopeWrite}
	if err := db.Create(&tok).Error; err != nil {
		t.Fatalf("token row: %v", err)
	}

	h := api.New(api.Deps{
		DB: db, AuthMode: config.AuthModeLocal, Local: local, Storage: store,
		ImportPool:  imports.NewSync(imports.Deps{DB: db, Storage: store, Broadcaster: imports.NewBroadcaster()}),
		Broadcaster: imports.NewBroadcaster(),
	})
	r, err := newRouter("", nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	api.AddRoutes(r.Group("/api"), auth.Middleware(local, auth.NewTokenAuthenticator(db)), h)

	const chunk = 32 << 20
	for i, tc := range []struct{ client, bearer string }{
		{"web app", session},
		{"dmsn", secret},
	} {
		t.Run(tc.client, func(t *testing.T) {
			// Two chunks, so the first is a full-size one; a distinct digest per
			// client keeps the second open from deduping onto the first.
			open, _ := json.Marshal(map[string]any{
				"filename":   fmt.Sprintf("big-%d.zip", i),
				"size_bytes": chunk + 1,
				"sha256":     strings.Repeat(fmt.Sprint(i), 64),
			})
			w := serve(r, http.MethodPost, "/api/imports/uploads", "application/json", tc.bearer, open)
			if w.Code != http.StatusCreated && w.Code != http.StatusOK {
				t.Fatalf("open: %d %s", w.Code, w.Body.String())
			}
			var opened struct {
				UploadID uint `json:"upload_id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &opened); err != nil {
				t.Fatalf("decode open: %v", err)
			}

			path := fmt.Sprintf("/api/imports/uploads/%d/chunks/0", opened.UploadID)
			w = serve(r, http.MethodPut, path, "application/octet-stream", tc.bearer, make([]byte, chunk))
			if w.Code != http.StatusOK {
				t.Fatalf("chunk: %d %s", w.Code, w.Body.String())
			}
		})
	}

	// The exemption is for the chunk route alone: any other route keeps the cap.
	big, _ := json.Marshal(map[string]string{"display_name": strings.Repeat("x", maxJSONBodyBytes)})
	if w := serve(r, http.MethodPatch, "/api/me", "application/json", session, big); w.Code != http.StatusBadRequest && w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("an oversized JSON body got %d; the 1 MiB cap is gone", w.Code)
	}
}

func serve(r http.Handler, method, path, contentType, bearer string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+bearer)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
