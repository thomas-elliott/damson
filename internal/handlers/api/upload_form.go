package api

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// uploadFileField carries the bytes. Every other recognised part is a text
// field and must precede it, because the file part is handed back unread.
const uploadFileField = "file"

// maxUploadFieldBytes bounds one text field, which is read into memory.
const maxUploadFieldBytes = 64 << 10

// Text fields postImport reads. Anything else in the body is skipped.
var uploadFormFields = map[string]struct{}{
	"asset_id":             {},
	"store_version":        {},
	"source_kind":          {},
	"source_relative_path": {},
}

var errFieldAfterFile = errors.New("form fields must be sent before the file part")

type uploadForm struct {
	values map[string]string
	reader *multipart.Reader
}

func (f uploadForm) get(name string) string { return f.values[name] }

// readUploadForm walks a multipart body far enough to collect the text fields
// and hand back the file part unread, so its bytes stream straight to storage.
// The caller must Close the returned part.
func readUploadForm(r *http.Request) (uploadForm, *multipart.Part, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return uploadForm{}, nil, err
	}
	form := uploadForm{values: map[string]string{}, reader: mr}

	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return uploadForm{}, nil, http.ErrMissingFile
		}
		if err != nil {
			return uploadForm{}, nil, err
		}
		if p.FormName() == uploadFileField {
			return form, p, nil
		}
		if _, wanted := uploadFormFields[p.FormName()]; !wanted {
			_ = p.Close()
			continue
		}
		v, err := io.ReadAll(io.LimitReader(p, maxUploadFieldBytes+1))
		_ = p.Close()
		if err != nil {
			return uploadForm{}, nil, err
		}
		if len(v) > maxUploadFieldBytes {
			return uploadForm{}, nil, fmt.Errorf("form field %q exceeds %d bytes", p.FormName(), maxUploadFieldBytes)
		}
		form.values[p.FormName()] = string(v)
	}
}

// checkNoTrailingFields drains the parts after the file. A recognised field
// there was skipped rather than applied, so it is an error, not a no-op.
func (f uploadForm) checkNoTrailingFields() error {
	if f.reader == nil {
		return nil
	}
	for {
		p, err := f.reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := p.FormName()
		_ = p.Close()
		if _, wanted := uploadFormFields[name]; wanted {
			return fmt.Errorf("%w: %q arrived after %q", errFieldAfterFile, name, uploadFileField)
		}
	}
}
