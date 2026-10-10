package api

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

// buildOrderedMultipart writes parts in exactly the order given, so a test can
// put the file before or after the text fields.
func buildOrderedMultipart(t *testing.T, parts []struct{ Name, Filename, Value string }) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for _, p := range parts {
		if p.Filename == "" {
			if err := w.WriteField(p.Name, p.Value); err != nil {
				t.Fatal(err)
			}
			continue
		}
		fw, err := w.CreateFormFile(p.Name, p.Filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(fw, p.Value); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return body, w.FormDataContentType()
}

type formPart = struct{ Name, Filename, Value string }

func requestFor(body *bytes.Buffer, ctype string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/imports", bytes.NewReader(body.Bytes()))
	r.Header.Set("Content-Type", ctype)
	return r
}

// TestReadUploadForm_ReturnsFilePartUnread is the property the whole stage
// rests on: the file's bytes must still be in the reader when the caller gets
// the part, because that is what lets them stream to storage instead of being
// spooled to scratch first.
func TestReadUploadForm_ReturnsFilePartUnread(t *testing.T) {
	body, ctype := buildOrderedMultipart(t, []formPart{
		{Name: "asset_id", Value: "42"},
		{Name: "source_kind", Value: "unity_cache"},
		{Name: "file", Filename: "pack.unitypackage", Value: "PAYLOAD-BYTES"},
	})

	form, part, err := readUploadForm(requestFor(body, ctype))
	if err != nil {
		t.Fatalf("readUploadForm: %v", err)
	}
	defer part.Close()

	if got := form.get("asset_id"); got != "42" {
		t.Errorf("asset_id = %q, want %q", got, "42")
	}
	if got := form.get("source_kind"); got != "unity_cache" {
		t.Errorf("source_kind = %q, want %q", got, "unity_cache")
	}
	if got := part.FileName(); got != "pack.unitypackage" {
		t.Errorf("FileName() = %q", got)
	}

	got, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("reading the file part: %v", err)
	}
	if string(got) != "PAYLOAD-BYTES" {
		t.Errorf("file part = %q, want the payload still unconsumed", got)
	}
}

// TestReadUploadForm_StopsAtTheFilePart proves nothing past the file has been
// touched — a reader that had drained the body to collect trailing fields
// would have buffered the whole upload, which is the bug being removed.
func TestReadUploadForm_StopsAtTheFilePart(t *testing.T) {
	body, ctype := buildOrderedMultipart(t, []formPart{
		{Name: "file", Filename: "a.bin", Value: "AAAA"},
		{Name: "store_version", Value: "2.0"},
	})

	form, part, err := readUploadForm(requestFor(body, ctype))
	if err != nil {
		t.Fatalf("readUploadForm: %v", err)
	}
	defer part.Close()

	if got := form.get("store_version"); got != "" {
		t.Errorf("store_version = %q, want empty — a field after the file cannot have been read", got)
	}
}

// TestUploadForm_TrailingFieldIsReported covers the failure mode the ordering
// requirement introduces: a field sent after the file used to be honoured and
// would now be silently dropped, so it must be an error instead.
func TestUploadForm_TrailingFieldIsReported(t *testing.T) {
	t.Run("recognised field after the file", func(t *testing.T) {
		body, ctype := buildOrderedMultipart(t, []formPart{
			{Name: "file", Filename: "a.bin", Value: "AAAA"},
			{Name: "asset_id", Value: "7"},
		})
		form, part, err := readUploadForm(requestFor(body, ctype))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, part)
		_ = part.Close()

		err = form.checkNoTrailingFields()
		if !errors.Is(err, errFieldAfterFile) {
			t.Fatalf("err = %v, want errFieldAfterFile", err)
		}
		if !strings.Contains(err.Error(), "asset_id") {
			t.Errorf("err = %q, want it to name the offending field", err)
		}
	})

	t.Run("unrecognised trailing part is tolerated", func(t *testing.T) {
		body, ctype := buildOrderedMultipart(t, []formPart{
			{Name: "file", Filename: "a.bin", Value: "AAAA"},
			{Name: "some_future_field", Value: "x"},
		})
		form, part, err := readUploadForm(requestFor(body, ctype))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, part)
		_ = part.Close()

		if err := form.checkNoTrailingFields(); err != nil {
			t.Errorf("an unknown trailing part must not fail the upload; got %v", err)
		}
	})
}

// TestReadUploadForm_NoFilePart keeps the genuine client mistake reporting as
// itself rather than as one of the new transport failures.
func TestReadUploadForm_NoFilePart(t *testing.T) {
	body, ctype := buildOrderedMultipart(t, []formPart{{Name: "asset_id", Value: "1"}})
	_, _, err := readUploadForm(requestFor(body, ctype))
	if !errors.Is(err, http.ErrMissingFile) {
		t.Fatalf("err = %v, want http.ErrMissingFile", err)
	}
}

// TestReadUploadForm_RejectsOversizeField bounds the in-memory field buffer, so
// a hostile body can't turn a "text field" into an unbounded allocation.
func TestReadUploadForm_RejectsOversizeField(t *testing.T) {
	body, ctype := buildOrderedMultipart(t, []formPart{
		{Name: "source_relative_path", Value: strings.Repeat("x", maxUploadFieldBytes+1)},
		{Name: "file", Filename: "a.bin", Value: "AAAA"},
	})
	_, _, err := readUploadForm(requestFor(body, ctype))
	if err == nil {
		t.Fatal("an oversize text field must be rejected")
	}
	if !strings.Contains(err.Error(), "source_relative_path") {
		t.Errorf("err = %q, want it to name the field", err)
	}
}

// TestPostImport_FieldsBeforeFileAreHonoured is the end-to-end check that the
// streaming reader still applies the text fields — a regression here would
// silently create a new asset instead of a new version of an existing one.
func TestPostImport_FieldsBeforeFileAreHonoured(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)

	existing := models.Asset{LibraryID: lib.ID, Name: "Existing Pack"}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	body, ctype := buildOrderedMultipart(t, []formPart{
		{Name: "asset_id", Value: itoa(existing.ID)},
		{Name: "store_version", Value: "v3.1"},
		{Name: "file", Filename: "Existing Pack.unitypackage", Value: "not-a-real-package"},
	})
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)

	var versions []models.AssetVersion
	if err := db.Where("asset_id = ?", existing.ID).Find(&versions).Error; err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("want 1 version on the targeted asset, got %d — asset_id was not applied", len(versions))
	}
	if versions[0].StoreVersion != "3.1" {
		t.Errorf("store_version = %q, want %q", versions[0].StoreVersion, "3.1")
	}

	var assets int64
	if err := db.Model(&models.Asset{}).Where("library_id = ?", lib.ID).Count(&assets).Error; err != nil {
		t.Fatal(err)
	}
	if assets != 1 {
		t.Errorf("want no new asset created, got %d assets in the library", assets)
	}
}

// TestPostImport_FieldAfterFileIsRejected pins the loud failure: dropping
// asset_id would attach the upload to the wrong place with no warning.
func TestPostImport_FieldAfterFileIsRejected(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)

	existing := models.Asset{LibraryID: lib.ID, Name: "Existing Pack"}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	body, ctype := buildOrderedMultipart(t, []formPart{
		{Name: "file", Filename: "Existing Pack.unitypackage", Value: "not-a-real-package"},
		{Name: "asset_id", Value: itoa(existing.ID)},
	})
	w := postMultipart(r, "/api/imports", body, ctype)

	requireStatus(t, w, http.StatusBadRequest)
	if !strings.Contains(w.Body.String(), "asset_id") {
		t.Errorf("body = %s, want the offending field named", w.Body.String())
	}

	// The rows created before the failure must not survive it.
	var imports int64
	if err := db.Model(&models.AssetImport{}).Count(&imports).Error; err != nil {
		t.Fatal(err)
	}
	if imports != 0 {
		t.Errorf("want the failed upload's import row cleaned up, got %d", imports)
	}
	// Nor may the audit log claim the deleted asset was created.
	var audits int64
	if err := db.Model(&models.AuditLog{}).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if audits != 0 {
		t.Errorf("want no audit row for the failed upload, got %d", audits)
	}
}

func itoa(v uint) string { return strconv.FormatUint(uint64(v), 10) }
