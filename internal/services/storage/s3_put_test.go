package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeS3 is an in-memory, path-style S3 good enough for the put/copy paths:
// PutObject, HeadObject, DeleteObject, CopyObject, and the multipart trio the
// SDK's Uploader uses for a large body.
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	parts   map[string]map[int][]byte

	// Recorded for assertions.
	puts        []string
	copies      []string
	deletes     []string
	multiparts  []string
	maxPartSeen int
}

func newFakeS3() *fakeS3 {
	return &fakeS3{objects: map[string][]byte{}, parts: map[string]map[int][]byte{}}
}

func (f *fakeS3) key(r *http.Request) string {
	return strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/"), "bucket/")
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := f.key(r)
	q := r.URL.Query()

	switch {
	case r.Method == http.MethodPost && q.Has("uploads"):
		id := "upload-" + key
		f.parts[id] = map[int][]byte{}
		f.multiparts = append(f.multiparts, key)
		writeXML(w, fmt.Sprintf(
			`<InitiateMultipartUploadResult><Bucket>bucket</Bucket><Key>%s</Key><UploadId>%s</UploadId></InitiateMultipartUploadResult>`, key, id))

	case r.Method == http.MethodPut && q.Get("uploadId") != "":
		id := q.Get("uploadId")
		var n int
		fmt.Sscanf(q.Get("partNumber"), "%d", &n)
		body, _ := io.ReadAll(r.Body)
		if f.parts[id] == nil {
			f.parts[id] = map[int][]byte{}
		}
		f.parts[id][n] = body
		if n > f.maxPartSeen {
			f.maxPartSeen = n
		}
		w.Header().Set("ETag", fmt.Sprintf("%q", fmt.Sprintf("part-%d", n)))
		w.WriteHeader(http.StatusOK)

	case r.Method == http.MethodPost && q.Get("uploadId") != "":
		id := q.Get("uploadId")
		_, _ = io.Copy(io.Discard, r.Body)
		var assembled []byte
		for i := 1; i <= len(f.parts[id]); i++ {
			assembled = append(assembled, f.parts[id][i]...)
		}
		f.objects[key] = assembled
		delete(f.parts, id)
		writeXML(w, fmt.Sprintf(
			`<CompleteMultipartUploadResult><Bucket>bucket</Bucket><Key>%s</Key><ETag>"done"</ETag></CompleteMultipartUploadResult>`, key))

	case r.Method == http.MethodPut && r.Header.Get("x-amz-copy-source") != "":
		src := strings.TrimPrefix(r.Header.Get("x-amz-copy-source"), "bucket/")
		f.objects[key] = append([]byte(nil), f.objects[src]...)
		f.copies = append(f.copies, src+" -> "+key)
		writeXML(w, `<CopyObjectResult><ETag>"copied"</ETag></CopyObjectResult>`)

	case r.Method == http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		f.objects[key] = body
		f.puts = append(f.puts, key)
		w.WriteHeader(http.StatusOK)

	case r.Method == http.MethodHead:
		b, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(b)))
		w.WriteHeader(http.StatusOK)

	case r.Method == http.MethodDelete:
		delete(f.objects, key)
		f.deletes = append(f.deletes, key)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func writeXML(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+body)
}

func newTestS3(t *testing.T) (*S3, *fakeS3) {
	t.Helper()
	fake := newFakeS3()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	s, err := NewS3(context.Background(), S3Config{
		Endpoint:       srv.URL,
		Region:         "us-east-1",
		Bucket:         "bucket",
		AccessKey:      "key",
		SecretKey:      "secret",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, fake
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// TestS3PutHashed_SmallBodyGoesStraightToItsKey covers the inline path: the
// content-addressed key is known before anything is written, so there is no
// staging object and no copy.
func TestS3PutHashed_SmallBodyGoesStraightToItsKey(t *testing.T) {
	s, fake := newTestS3(t)
	body := []byte("a small loose file")

	key, n, err := s.PutBlob(context.Background(), bytes.NewReader(body), ObjectMeta{Filename: "x.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if key != sha256Hex(body) {
		t.Errorf("key = %s, want the content hash", key)
	}
	if n != int64(len(body)) {
		t.Errorf("size = %d, want %d", n, len(body))
	}

	wantKey := "blobs/" + shardedKey(key)
	if got, ok := fake.objects[wantKey]; !ok || !bytes.Equal(got, body) {
		t.Errorf("object at %s = %q, want the body", wantKey, got)
	}
	if len(fake.copies) != 0 {
		t.Errorf("a small body must not be copied; got %v", fake.copies)
	}
	for _, k := range fake.puts {
		if strings.HasPrefix(k, stagingPrefix+"/") {
			t.Errorf("a small body must not use a staging object; wrote %s", k)
		}
	}
}

// TestS3PutHashed_LargeBodyStagesThenCopies covers the path that replaced the
// temp-file spool. The bytes must reach the bucket as multipart parts and end
// up at the content-addressed key, with the staging object cleaned up.
func TestS3PutHashed_LargeBodyStagesThenCopies(t *testing.T) {
	s, fake := newTestS3(t)
	// Just over the inline threshold, so the staged path is taken.
	body := bytes.Repeat([]byte("z"), inlinePutBytes+1024)

	key, n, err := s.PutOriginal(context.Background(), bytes.NewReader(body), ObjectMeta{Filename: "big.zip"})
	if err != nil {
		t.Fatal(err)
	}
	if key != sha256Hex(body) {
		t.Errorf("key = %s, want the content hash of the streamed bytes", key)
	}
	if n != int64(len(body)) {
		t.Errorf("size = %d, want %d", n, len(body))
	}

	finalKey := "originals/" + shardedKey(key)
	got, ok := fake.objects[finalKey]
	if !ok {
		t.Fatalf("nothing at the final key %s; objects = %v", finalKey, keysOf(fake.objects))
	}
	if !bytes.Equal(got, body) {
		t.Errorf("stored %d bytes at the final key, want %d identical", len(got), len(body))
	}

	if len(fake.multiparts) == 0 {
		t.Error("a large body must be uploaded as multipart parts, not one PutObject")
	}
	if len(fake.copies) != 1 {
		t.Errorf("want exactly one staging→final copy, got %v", fake.copies)
	}

	// The staging object is transient: it must not survive the call.
	for k := range fake.objects {
		if strings.HasPrefix(k, stagingPrefix+"/") {
			t.Errorf("staging object %s left behind", k)
		}
	}
	if len(fake.deletes) != 1 || !strings.HasPrefix(fake.deletes[0], stagingPrefix+"/") {
		t.Errorf("want the staging object deleted, got deletes = %v", fake.deletes)
	}
}

// TestS3PutHashed_DedupeSkipsTheCopy proves HEAD-before-write survives the
// rewrite: re-uploading content the bucket already holds must not copy 20 GB
// around inside the bucket.
func TestS3PutHashed_DedupeSkipsTheCopy(t *testing.T) {
	s, fake := newTestS3(t)
	body := bytes.Repeat([]byte("y"), inlinePutBytes+1024)

	key, _, err := s.PutOriginal(context.Background(), bytes.NewReader(body), ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	copiesAfterFirst := len(fake.copies)

	key2, _, err := s.PutOriginal(context.Background(), bytes.NewReader(body), ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if key2 != key {
		t.Errorf("second upload key = %s, want the same content hash %s", key2, key)
	}
	if len(fake.copies) != copiesAfterFirst {
		t.Errorf("the second upload must dedupe rather than copy again; copies = %v", fake.copies)
	}
	if len(fake.deletes) != 2 {
		t.Errorf("both staging objects must be dropped, got deletes = %v", fake.deletes)
	}
}

// TestS3PutHashed_HashesWhatActuallyArrived guards the TeeReader wiring: the
// key must be the hash of the bytes that reached the bucket, not of anything
// the caller claimed.
func TestS3PutHashed_HashesWhatActuallyArrived(t *testing.T) {
	s, fake := newTestS3(t)
	body := bytes.Repeat([]byte("q"), inlinePutBytes+7)

	key, _, err := s.PutBlob(context.Background(), iotaReader(body), ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	stored := fake.objects["blobs/"+shardedKey(key)]
	if sha256Hex(stored) != key {
		t.Errorf("stored bytes hash to %s but the key is %s", sha256Hex(stored), key)
	}
	if len(stored) != len(body) {
		t.Errorf("stored %d bytes, want %d", len(stored), len(body))
	}
}

// iotaReader returns b through a reader that yields short reads, which is how
// a network body actually behaves and what a naive single-Read implementation
// would get wrong.
func iotaReader(b []byte) io.Reader {
	return &choppyReader{b: b}
}

type choppyReader struct {
	b   []byte
	off int
}

func (c *choppyReader) Read(p []byte) (int, error) {
	if c.off >= len(c.b) {
		return 0, io.EOF
	}
	n := 7919
	if n > len(p) {
		n = len(p)
	}
	if c.off+n > len(c.b) {
		n = len(c.b) - c.off
	}
	copy(p, c.b[c.off:c.off+n])
	c.off += n
	return n, nil
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
