package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// multipartBackend lets one contract run against both implementations, with
// the two backend-specific questions the contract cares about behind funcs.
type multipartBackend struct {
	name string
	svc  Service
	// object returns the bytes finally stored at a content key.
	object func(prefix, key string) ([]byte, bool)
	// pending reports whether in-progress state for an upload id still exists.
	pending func(id string) bool
}

func multipartBackends(t *testing.T) []multipartBackend {
	t.Helper()

	root := t.TempDir()
	lfs, err := NewLocalFS(root)
	if err != nil {
		t.Fatal(err)
	}
	s3svc, fake := newTestS3(t)

	return []multipartBackend{
		{
			name: "localfs",
			svc:  lfs,
			object: func(prefix, key string) ([]byte, bool) {
				b, err := os.ReadFile(filepath.Join(root, prefix, key[:2], key))
				return b, err == nil
			},
			pending: func(id string) bool {
				_, err := os.Stat(filepath.Join(root, uploadsDir, id))
				return err == nil
			},
		},
		{
			name: "s3",
			svc:  s3svc,
			object: func(prefix, key string) ([]byte, bool) {
				b, ok := fake.objects[prefix+"/"+shardedKey(key)]
				return b, ok
			},
			pending: func(id string) bool {
				_, ok := fake.parts[id]
				return ok
			},
		},
	}
}

const contentKey = "aa" + "bbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

func upload(t *testing.T, u MultipartUpload, n int, body string) Part {
	t.Helper()
	etag, err := u.UploadPart(context.Background(), n, int64(len(body)), strings.NewReader(body))
	if err != nil {
		t.Fatalf("upload part %d: %v", n, err)
	}
	if etag == "" {
		t.Fatalf("part %d returned an empty etag — Complete has nothing to reference it by", n)
	}
	return Part{Number: n, ETag: etag}
}

// TestMultipart_AssemblesInPartOrder is the contract every chunked upload
// rests on: the object is the parts concatenated by part number, whatever
// order the caller uploaded or listed them in.
func TestMultipart_AssemblesInPartOrder(t *testing.T) {
	for _, b := range multipartBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			ctx := context.Background()
			u, err := b.svc.BeginMultipart(ctx, "originals", contentKey, ObjectMeta{Filename: "pack.zip"})
			if err != nil {
				t.Fatal(err)
			}
			if u.ID() == "" {
				t.Fatal("empty upload id — the session could never be resumed")
			}

			p3 := upload(t, u, 3, "-three")
			p1 := upload(t, u, 1, "one")
			p2 := upload(t, u, 2, "-two")

			key, err := u.Complete(ctx, []Part{p3, p1, p2})
			if err != nil {
				t.Fatal(err)
			}
			if key != contentKey {
				t.Errorf("key = %q, want the declared content key %q", key, contentKey)
			}
			got, ok := b.object("originals", contentKey)
			if !ok {
				t.Fatal("no object at the content key")
			}
			if string(got) != "one-two-three" {
				t.Errorf("object = %q, want the parts in part-number order", got)
			}
			if b.pending(u.ID()) {
				t.Error("in-progress state survived Complete")
			}
		})
	}
}

// TestMultipart_ResumeContinuesTheSameUpload is the whole point of the
// abstraction: chunks arrive as separate requests, so every chunk after the
// first runs on a handle rebuilt from the stored id.
func TestMultipart_ResumeContinuesTheSameUpload(t *testing.T) {
	for _, b := range multipartBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			ctx := context.Background()
			u, err := b.svc.BeginMultipart(ctx, "originals", contentKey, ObjectMeta{Filename: "pack.zip", LibraryID: 4})
			if err != nil {
				t.Fatal(err)
			}
			p1 := upload(t, u, 1, "first-")

			resumed, err := b.svc.ResumeMultipart(ctx, "originals", contentKey, u.ID())
			if err != nil {
				t.Fatalf("resume: %v", err)
			}
			if resumed.ID() != u.ID() {
				t.Errorf("resumed id = %q, want %q", resumed.ID(), u.ID())
			}
			p2 := upload(t, resumed, 2, "second")

			if _, err := resumed.Complete(ctx, []Part{p1, p2}); err != nil {
				t.Fatal(err)
			}
			got, ok := b.object("originals", contentKey)
			if !ok {
				t.Fatal("no object at the content key")
			}
			if string(got) != "first-second" {
				t.Errorf("object = %q, want both parts — a resumed handle must not start a new upload", got)
			}
		})
	}
}

// TestMultipart_ReuploadReplacesThePart covers the retry path: a chunk that
// times out is re-sent under the same index and must not be appended twice.
func TestMultipart_ReuploadReplacesThePart(t *testing.T) {
	for _, b := range multipartBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			ctx := context.Background()
			u, err := b.svc.BeginMultipart(ctx, "blobs", contentKey, ObjectMeta{})
			if err != nil {
				t.Fatal(err)
			}
			upload(t, u, 1, "GARBAGE")
			p1 := upload(t, u, 1, "good")

			if _, err := u.Complete(ctx, []Part{p1}); err != nil {
				t.Fatal(err)
			}
			got, _ := b.object("blobs", contentKey)
			if string(got) != "good" {
				t.Errorf("object = %q, want only the retried part's bytes", got)
			}
		})
	}
}

// TestMultipart_AbortLeavesNothing pins that an abandoned session costs
// nothing: no object at the target key and no in-progress state to leak.
func TestMultipart_AbortLeavesNothing(t *testing.T) {
	for _, b := range multipartBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			ctx := context.Background()
			u, err := b.svc.BeginMultipart(ctx, "originals", contentKey, ObjectMeta{})
			if err != nil {
				t.Fatal(err)
			}
			upload(t, u, 1, "partial")

			if err := u.Abort(ctx); err != nil {
				t.Fatal(err)
			}
			if _, ok := b.object("originals", contentKey); ok {
				t.Error("an aborted upload must not publish its target key")
			}
			if b.pending(u.ID()) {
				t.Error("in-progress state survived Abort")
			}
		})
	}
}

// TestMultipart_RejectsOutOfRangePart guards the numbering the caller derives
// from chunk index; S3 stops at 10,000 parts and LocalFS agrees so the dev
// backend fails the same way.
func TestMultipart_RejectsOutOfRangePart(t *testing.T) {
	for _, b := range multipartBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			ctx := context.Background()
			u, err := b.svc.BeginMultipart(ctx, "blobs", contentKey, ObjectMeta{})
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range []int{0, -1, 10001} {
				if _, err := u.UploadPart(ctx, n, 1, strings.NewReader("x")); err == nil {
					t.Errorf("part number %d was accepted", n)
				}
			}
		})
	}
}

// TestLocalMultipart_RejectsShortPart is LocalFS-only because S3 enforces it
// server-side via Content-Length. Without the check a truncated chunk would
// be stored silently and only surface as a hash mismatch at complete.
func TestLocalMultipart_RejectsShortPart(t *testing.T) {
	lfs, err := NewLocalFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	u, err := lfs.BeginMultipart(ctx, "originals", contentKey, ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := u.UploadPart(ctx, 1, 100, strings.NewReader("only ten!!")); err == nil {
		t.Fatal("a part shorter than its declared size was accepted")
	}
	if _, err := u.Complete(ctx, []Part{{Number: 1}}); err == nil {
		t.Error("the rejected part must not remain available to Complete")
	}
}

// TestLocalMultipart_ResumeKeepsMetadata guards the sidecar: Complete runs on
// a resumed handle in every real flow, so metadata set at Begin has to survive
// the round-trip through the upload id.
func TestLocalMultipart_ResumeKeepsMetadata(t *testing.T) {
	root := t.TempDir()
	lfs, err := NewLocalFS(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	meta := ObjectMeta{Filename: "Big Pack.unitypackage", LibraryID: 2, AssetID: 9, Kind: "unitypackage"}

	u, err := lfs.BeginMultipart(ctx, "originals", contentKey, meta)
	if err != nil {
		t.Fatal(err)
	}
	p1 := upload(t, u, 1, "payload")

	resumed, err := lfs.ResumeMultipart(ctx, "originals", contentKey, u.ID())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resumed.Complete(ctx, []Part{p1}); err != nil {
		t.Fatal(err)
	}

	got, err := lfs.StatMeta(ctx, "originals", contentKey)
	if err != nil {
		t.Fatal(err)
	}
	if got != meta {
		t.Errorf("meta = %+v, want %+v", got, meta)
	}
	if entries, err := os.ReadDir(filepath.Join(root, uploadsDir)); err != nil || len(entries) != 0 {
		t.Errorf("upload scratch = %v (err %v), want it cleared", entries, err)
	}
}

// TestLocalMultipart_ResumeRejectsUnknownUpload keeps a stale or forged id
// from silently opening a fresh upload that would complete with missing parts.
func TestLocalMultipart_ResumeRejectsUnknownUpload(t *testing.T) {
	lfs, err := NewLocalFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "../../etc", strings.Repeat("f", 32)} {
		if _, err := lfs.ResumeMultipart(context.Background(), "originals", contentKey, id); err == nil {
			t.Errorf("upload id %q was accepted", id)
		}
	}
}

// TestMultipart_DedupesOntoAnExistingObject: content-addressing means two
// clients can upload identical bytes concurrently. The loser must land on the
// same key without corrupting the winner's object.
func TestMultipart_DedupesOntoAnExistingObject(t *testing.T) {
	root := t.TempDir()
	lfs, err := NewLocalFS(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	first, err := lfs.BeginMultipart(ctx, "originals", contentKey, ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	p := upload(t, first, 1, "the winning bytes")
	if _, err := first.Complete(ctx, []Part{p}); err != nil {
		t.Fatal(err)
	}

	second, err := lfs.BeginMultipart(ctx, "originals", contentKey, ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	p2 := upload(t, second, 1, "the winning bytes")
	if _, err := second.Complete(ctx, []Part{p2}); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(filepath.Join(root, "originals", contentKey[:2], contentKey))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, []byte("the winning bytes")) {
		t.Errorf("object = %q, want the winner's bytes intact", body)
	}
	if second.(*localMultipart).dirExists() {
		t.Error("the deduped upload left its scratch behind")
	}
}

func (m *localMultipart) dirExists() bool {
	_, err := os.Stat(m.dir())
	return err == nil
}
