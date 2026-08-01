package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
)

// Payloads at or above this size go through the chunked transport.
const chunkedUploadThreshold = 256 << 20

const (
	maxChunkAttempts = 5
	retryBaseDelay   = 500 * time.Millisecond
)

type uploadSession struct {
	UploadID       uint   `json:"upload_id"`
	Status         string `json:"status"`
	ChunkSize      int64  `json:"chunk_size"`
	SizeBytes      int64  `json:"size_bytes"`
	ReceivedBytes  int64  `json:"received_bytes"`
	NextChunk      int    `json:"next_chunk"`
	TotalChunks    int    `json:"total_chunks"`
	Deduped        bool   `json:"deduped"`
	AssetID        uint   `json:"asset_id"`
	ImportID       uint   `json:"import_id"`
	SuggestedMatch *struct {
		AssetID uint   `json:"asset_id"`
		Name    string `json:"name"`
	} `json:"suggested_match"`
}

func (s uploadSession) response() *importEnqueueResponse {
	return &importEnqueueResponse{
		AssetID:        s.AssetID,
		ImportID:       s.ImportID,
		SuggestedMatch: s.SuggestedMatch,
	}
}

// hashPayload reads r to the end and rewinds it, returning the content hash.
func hashPayload(r io.ReadSeeker) (string, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", axi.Wrap(axi.CodeIO, err, "cannot rewind the payload: %v", err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", axi.Wrap(axi.CodeIO, err, "cannot read the payload: %v", err)
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", axi.Wrap(axi.CodeIO, err, "cannot rewind the payload: %v", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// seekableReaderAt is what the chunked transport needs: one full pass to hash,
// then ranged reads for the chunks.
type seekableReaderAt interface {
	io.ReadSeeker
	io.ReaderAt
}

// UploadChunked hashes the payload, opens (or resumes) an upload session, and
// sends it a chunk at a time.
func (c *Client) UploadChunked(
	ctx context.Context,
	filename string,
	r seekableReaderAt,
	size int64,
	opts importOptions,
	resumeKey string,
	onProgress func(sent int64),
) (*importEnqueueResponse, error) {
	digest, err := hashPayload(r)
	if err != nil {
		return nil, err
	}

	session, err := c.resumeSession(ctx, resumeKey, size)
	if err != nil {
		return nil, err
	}
	if session == nil {
		session, err = c.openSession(ctx, filename, size, digest, opts)
		if err != nil {
			return nil, err
		}
		if session.Deduped {
			if onProgress != nil {
				onProgress(size)
			}
			return session.response(), nil
		}
		rememberUploadSession(resumeKey, session.UploadID)
	}

	if onProgress != nil {
		onProgress(session.ReceivedBytes)
	}
	for index := session.NextChunk; int64(index)*session.ChunkSize < size; {
		offset := int64(index) * session.ChunkSize
		length := min(session.ChunkSize, size-offset)
		state, err := c.sendChunk(ctx, session.UploadID, index, io.NewSectionReader(r, offset, length), length)
		if err != nil {
			return nil, err
		}
		if onProgress != nil {
			onProgress(state.ReceivedBytes)
		}
		if state.NextChunk <= index {
			return nil, axi.Errorf(axi.CodeServer, "server did not advance past chunk %d", index)
		}
		index = state.NextChunk
	}

	var done uploadSession
	path := "/api/imports/uploads/" + strconv.FormatUint(uint64(session.UploadID), 10) + "/complete"
	if err := c.PostJSON(ctx, path, struct{}{}, &done); err != nil {
		return nil, err
	}
	forgetUploadSession(resumeKey)
	return done.response(), nil
}

func (c *Client) openSession(ctx context.Context, filename string, size int64, digest string, opts importOptions) (*uploadSession, error) {
	req := map[string]any{
		"filename":   filename,
		"size_bytes": size,
		"sha256":     digest,
	}
	for k, v := range opts.fields() {
		if k == "asset_id" {
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, axi.Wrap(axi.CodeUsage, err, "invalid --asset-id %q", v)
			}
			req[k] = n
			continue
		}
		req[k] = v
	}
	var out uploadSession
	if err := c.PostJSON(ctx, "/api/imports/uploads", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// resumeSession returns a still-open session recorded for this payload, or nil
// when there is none to resume.
func (c *Client) resumeSession(ctx context.Context, resumeKey string, size int64) (*uploadSession, error) {
	uploadID := storedUploadSession(resumeKey)
	if uploadID == 0 {
		return nil, nil
	}
	var out uploadSession
	path := "/api/imports/uploads/" + strconv.FormatUint(uint64(uploadID), 10)
	if err := c.GetJSON(ctx, path, nil, &out); err != nil {
		forgetUploadSession(resumeKey)
		return nil, nil
	}
	if out.Status != "open" || out.SizeBytes != size {
		forgetUploadSession(resumeKey)
		return nil, nil
	}
	return &out, nil
}

func (c *Client) sendChunk(ctx context.Context, uploadID uint, index int, body io.ReadSeeker, length int64) (*uploadSession, error) {
	url := fmt.Sprintf("%s/api/imports/uploads/%d/chunks/%d", c.BaseURL, uploadID, index)
	var lastErr error
	for attempt := 1; attempt <= maxChunkAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := body.Seek(0, io.SeekStart); err != nil {
			return nil, axi.Wrap(axi.CodeIO, err, "cannot rewind chunk %d: %v", index, err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
		if err != nil {
			return nil, axi.Wrap(axi.CodeUsage, err, "build chunk request")
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		req.ContentLength = length
		c.authorise(req)

		resp, err := c.uploadHTTP().Do(req)
		if err != nil {
			lastErr = networkError(err, c.BaseURL)
		} else {
			var out uploadSession
			ferr := finish(resp, &out)
			resp.Body.Close()
			if ferr == nil {
				return &out, nil
			}
			if !retryableStatus(resp.StatusCode) {
				return nil, ferr
			}
			lastErr = ferr
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryBaseDelay << (attempt - 1)):
		}
	}
	return nil, lastErr
}

func retryableStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

// Resumable session state, keyed by payload identity, kept beside the config.

type resumeState struct {
	Sessions map[string]uint `json:"sessions"`
}

var resumeStateMu sync.Mutex

func resumeStatePath() (string, error) {
	cfg, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfg), "uploads.json"), nil
}

func uploadResumeKey(path string, size int64, modTime time.Time) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return fmt.Sprintf("%s:%d:%d", abs, size, modTime.UnixNano())
}

func readResumeState() resumeState {
	state := resumeState{Sessions: map[string]uint{}}
	path, err := resumeStatePath()
	if err != nil {
		return state
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil || state.Sessions == nil {
		return resumeState{Sessions: map[string]uint{}}
	}
	return state
}

func writeResumeState(state resumeState) {
	path, err := resumeStatePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func storedUploadSession(key string) uint {
	if key == "" {
		return 0
	}
	resumeStateMu.Lock()
	defer resumeStateMu.Unlock()
	return readResumeState().Sessions[key]
}

func rememberUploadSession(key string, uploadID uint) {
	if key == "" {
		return
	}
	resumeStateMu.Lock()
	defer resumeStateMu.Unlock()
	state := readResumeState()
	state.Sessions[key] = uploadID
	writeResumeState(state)
}

func forgetUploadSession(key string) {
	if key == "" {
		return
	}
	resumeStateMu.Lock()
	defer resumeStateMu.Unlock()
	state := readResumeState()
	if _, ok := state.Sessions[key]; !ok {
		return
	}
	delete(state.Sessions, key)
	writeResumeState(state)
}
