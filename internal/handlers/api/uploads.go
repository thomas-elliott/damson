package api

import (
	"context"
	"crypto/sha256"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/ingest"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/storage"
)

const (
	uploadChunkBytes = 32 << 20
	uploadSessionTTL = 24 * time.Hour
)

var errHashNotResumable = errors.New("sha256 digest cannot be marshalled")

type uploadOpenRequest struct {
	Filename           string `json:"filename"`
	SizeBytes          int64  `json:"size_bytes"`
	Sha256             string `json:"sha256"`
	ContentType        string `json:"content_type"`
	AssetID            uint   `json:"asset_id"`
	StoreVersion       string `json:"store_version"`
	SourceKind         string `json:"source_kind"`
	SourceRelativePath string `json:"source_relative_path"`
}

type uploadSessionResponse struct {
	UploadID       uint             `json:"upload_id"`
	Status         string           `json:"status"`
	ChunkSize      int64            `json:"chunk_size"`
	SizeBytes      int64            `json:"size_bytes"`
	ReceivedBytes  int64            `json:"received_bytes"`
	NextChunk      int              `json:"next_chunk"`
	TotalChunks    int              `json:"total_chunks"`
	Deduped        bool             `json:"deduped"`
	ExpiresAt      time.Time        `json:"expires_at"`
	AssetID        uint             `json:"asset_id,omitempty"`
	ImportID       uint             `json:"import_id,omitempty"`
	SuggestedMatch *matchSuggestion `json:"suggested_match,omitempty"`
}

func totalChunks(size, chunkSize int64) int {
	if size <= 0 || chunkSize <= 0 {
		return 0
	}
	return int((size + chunkSize - 1) / chunkSize)
}

func sessionResponse(s *models.UploadSession) uploadSessionResponse {
	r := uploadSessionResponse{
		UploadID:      s.ID,
		Status:        s.Status,
		ChunkSize:     s.ChunkSize,
		SizeBytes:     s.SizeBytes,
		ReceivedBytes: s.ReceivedBytes,
		NextChunk:     s.NextChunk,
		TotalChunks:   totalChunks(s.SizeBytes, s.ChunkSize),
		ExpiresAt:     s.ExpiresAt,
	}
	if s.AssetID != nil {
		r.AssetID = *s.AssetID
	}
	if s.ImportID != nil {
		r.ImportID = *s.ImportID
	}
	return r
}

// postUploadSession opens a resumable upload.
func (h *Handler) postUploadSession(c *gin.Context) {
	lib, u, ok := h.uploadContext(c)
	if !ok {
		return
	}

	var req uploadOpenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	digest := strings.ToLower(strings.TrimSpace(req.Sha256))
	if !isSHA256Hex(digest) {
		respond400(c, errors.New("sha256 must be a 64-character hex digest"))
		return
	}
	if req.SizeBytes <= 0 {
		respond400(c, errors.New("size_bytes must be positive"))
		return
	}
	settings, err := loadOrCreateServerSettings(h.db.WithContext(c.Request.Context()))
	if err != nil {
		respond500(c, err)
		return
	}
	if settings.MaxUploadBytes > 0 && req.SizeBytes > settings.MaxUploadBytes {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": msgUploadTooLarge})
		return
	}

	spec := newImportSpec(lib.ID, u.ID, req.Filename, req.ContentType)
	spec.targetAssetID = req.AssetID
	spec.reqStoreVersion = strings.TrimLeft(strings.TrimSpace(req.StoreVersion), "vV")
	spec.sourceKind = strings.TrimSpace(req.SourceKind)
	spec.sourceRelativePath, err = ingest.ValidateSource(spec.sourceKind, req.SourceRelativePath)
	if err != nil {
		slog.Warn("upload session: invalid source hint",
			"err", err, "library_id", lib.ID, "source_kind", spec.sourceKind)
		respond400(c, err)
		return
	}

	ctx := c.Request.Context()
	if err := h.checkImportConflict(ctx, spec); err != nil {
		respondGORM(c, err, "target asset not found")
		return
	}

	// Touched as well as checked: a reused object must look new to the orphan
	// sweep until the import's rows reference it.
	present, err := h.storage.Touch(ctx, spec.storagePrefix(), digest)
	if err != nil {
		respond500(c, fmt.Errorf("head %s (library %d): %w", digest, lib.ID, err))
		return
	}
	if present {
		h.completeDedupedUpload(c, spec, digest, req.SizeBytes)
		return
	}

	up, err := h.storage.BeginMultipart(ctx, spec.storagePrefix(), digest, spec.objectMeta(0, 0))
	if err != nil {
		respond500(c, fmt.Errorf("begin multipart (library %d): %w", lib.ID, err))
		return
	}

	session := models.UploadSession{
		LibraryID:          lib.ID,
		UserID:             u.ID,
		Status:             models.UploadOpen,
		Filename:           spec.filename,
		ContentType:        spec.contentType,
		SizeBytes:          req.SizeBytes,
		Sha256:             digest,
		ChunkSize:          h.chunkSize,
		ObjectPrefix:       string(spec.storagePrefix()),
		BackendUploadID:    up.ID(),
		PartsJSON:          json.RawMessage("[]"),
		StoreVersion:       spec.reqStoreVersion,
		SourceKind:         spec.sourceKind,
		SourceRelativePath: spec.sourceRelativePath,
		ExpiresAt:          time.Now().UTC().Add(uploadSessionTTL),
	}
	if spec.targetAssetID != 0 {
		session.TargetAssetID = &spec.targetAssetID
	}
	if err := h.db.WithContext(ctx).Create(&session).Error; err != nil {
		if aerr := up.Abort(context.WithoutCancel(ctx)); aerr != nil {
			slog.Error("upload session: abort after failed create", "err", aerr, "upload_id", up.ID())
		}
		respond500(c, err)
		return
	}
	slog.Info("upload session opened",
		"upload_id", session.ID, "library_id", lib.ID, "user_id", u.ID,
		"filename", session.Filename, "size_bytes", session.SizeBytes,
		"chunk_size", session.ChunkSize)

	c.JSON(http.StatusCreated, sessionResponse(&session))
}

// completeDedupedUpload runs the import against an object storage already
// holds. No session row is created — there is nothing left to transfer.
func (h *Handler) completeDedupedUpload(c *gin.Context, spec importSpec, objectKey string, size int64) {
	ctx := c.Request.Context()
	rows, err := h.createImportRows(ctx, spec)
	if err != nil {
		respondGORM(c, err, "target asset not found")
		return
	}
	if err := h.finishImport(ctx, spec, rows, objectKey, size); err != nil {
		h.abandonImport(context.WithoutCancel(ctx), spec, rows)
		respond500(c, err)
		return
	}
	slog.Info("upload deduped against stored content",
		"library_id", spec.libraryID, "asset_id", rows.assetID,
		"import_id", rows.importID, "object_key", objectKey, "size_bytes", size)

	chunks := totalChunks(size, h.chunkSize)
	c.JSON(http.StatusAccepted, uploadSessionResponse{
		Status:         models.UploadCompleted,
		ChunkSize:      h.chunkSize,
		SizeBytes:      size,
		ReceivedBytes:  size,
		TotalChunks:    chunks,
		NextChunk:      chunks,
		Deduped:        true,
		AssetID:        rows.assetID,
		ImportID:       rows.importID,
		SuggestedMatch: h.suggestMatch(ctx, spec, rows.assetID),
	})
}

func (h *Handler) getUploadSession(c *gin.Context) {
	session, ok := h.loadUploadSession(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, sessionResponse(session))
}

// putUploadChunk streams one chunk straight into the backing multipart upload
// and advances the running hash. Chunks are strictly sequential: the hash
// state travels through the database between requests.
func (h *Handler) putUploadChunk(c *gin.Context) {
	session, ok := h.loadUploadSession(c)
	if !ok {
		return
	}
	if !h.requireOpenSession(c, session) {
		return
	}

	index, err := strconv.Atoi(c.Param("n"))
	if err != nil || index < 0 {
		respond400Msg(c, fmt.Errorf("invalid chunk index %q: %w", c.Param("n"), err), "invalid chunk index")
		return
	}
	if index < session.NextChunk {
		c.JSON(http.StatusOK, sessionResponse(session))
		return
	}
	if index > session.NextChunk {
		respond409(c, fmt.Sprintf("chunk %d is out of order; send chunk %d next", index, session.NextChunk))
		return
	}

	remaining := session.SizeBytes - session.ReceivedBytes
	if remaining <= 0 {
		respond409(c, "the declared size has already been received; complete the upload")
		return
	}
	expected := min(session.ChunkSize, remaining)
	if c.Request.ContentLength >= 0 && c.Request.ContentLength != expected {
		respond400(c, fmt.Errorf("chunk %d must be %d bytes, got %d", index, expected, c.Request.ContentLength))
		return
	}

	running, err := resumeUploadHash(session.HashState)
	if err != nil {
		respond500(c, err)
		return
	}

	ctx := c.Request.Context()
	up, err := h.storage.ResumeMultipart(ctx, storage.Prefix(session.ObjectPrefix), session.Sha256, session.BackendUploadID)
	if err != nil {
		respond500(c, fmt.Errorf("resume multipart (session %d): %w", session.ID, err))
		return
	}

	body := http.MaxBytesReader(c.Writer, c.Request.Body, expected)
	etag, err := up.UploadPart(ctx, index+1, expected, io.TeeReader(body, running))
	if err != nil {
		slog.Warn("upload chunk failed",
			"err", err, "upload_id", session.ID, "library_id", session.LibraryID,
			"chunk", index, "expected_bytes", expected)
		respondUploadOrServerError(c, err, fmt.Errorf("upload part (session %d, chunk %d): %w", session.ID, index, err),
			"upload_id", session.ID, "library_id", session.LibraryID, "chunk", index)
		return
	}

	state, err := marshalUploadHash(running)
	if err != nil {
		respond500(c, err)
		return
	}
	parts, err := appendPart(session.PartsJSON, storage.Part{Number: index + 1, ETag: etag})
	if err != nil {
		respond500(c, err)
		return
	}

	// Conditional on next_chunk: two requests racing the same index must not
	// both advance the running hash. The loser gets 409 and re-reads the state.
	res := h.db.WithContext(ctx).Model(&models.UploadSession{}).
		Where("id = ? AND status = ? AND next_chunk = ?", session.ID, models.UploadOpen, index).
		Updates(map[string]any{
			"received_bytes": session.ReceivedBytes + expected,
			"next_chunk":     index + 1,
			"hash_state":     state,
			"parts_json":     parts,
			"updated_at":     time.Now().UTC(),
		})
	if res.Error != nil {
		respond500(c, res.Error)
		return
	}
	if res.RowsAffected == 0 {
		respond409(c, fmt.Sprintf("chunk %d was already accepted by another request", index))
		return
	}

	session.ReceivedBytes += expected
	session.NextChunk = index + 1
	c.JSON(http.StatusOK, sessionResponse(session))
}

// postUploadComplete verifies the received bytes against the declared hash,
// then runs the same import the one-shot endpoint does.
func (h *Handler) postUploadComplete(c *gin.Context) {
	session, ok := h.loadUploadSession(c)
	if !ok {
		return
	}
	if session.Status == models.UploadCompleted {
		c.JSON(http.StatusAccepted, sessionResponse(session))
		return
	}
	if !h.requireOpenSession(c, session) {
		return
	}
	if session.ReceivedBytes != session.SizeBytes {
		respond409(c, fmt.Sprintf("received %d of %d bytes; send chunk %d next",
			session.ReceivedBytes, session.SizeBytes, session.NextChunk))
		return
	}

	ctx := c.Request.Context()
	running, err := resumeUploadHash(session.HashState)
	if err != nil {
		respond500(c, err)
		return
	}
	got := hex.EncodeToString(running.Sum(nil))
	up, err := h.storage.ResumeMultipart(ctx, storage.Prefix(session.ObjectPrefix), session.Sha256, session.BackendUploadID)
	if err != nil {
		respond500(c, fmt.Errorf("resume multipart (session %d): %w", session.ID, err))
		return
	}
	if got != session.Sha256 {
		slog.Warn("upload hash mismatch",
			"upload_id", session.ID, "library_id", session.LibraryID,
			"declared", session.Sha256, "received", got)
		h.discardSession(context.WithoutCancel(ctx), session, up, models.UploadAborted)
		respond400(c, fmt.Errorf("uploaded content hashes to %s, not the declared %s", got, session.Sha256))
		return
	}

	// Rows first, then the multipart: a conflict here must not leave a
	// completed object that nothing references.
	spec := h.specForSession(session)
	rows, err := h.createImportRows(ctx, spec)
	if err != nil {
		respondGORM(c, err, "target asset not found")
		return
	}

	objectKey, err := up.Complete(ctx, sessionParts(session))
	if err != nil {
		h.abandonImport(context.WithoutCancel(ctx), spec, rows)
		respond500(c, fmt.Errorf("complete multipart (session %d): %w", session.ID, err))
		return
	}
	if err := h.finishImport(ctx, spec, rows, objectKey, session.SizeBytes); err != nil {
		h.abandonImport(context.WithoutCancel(ctx), spec, rows)
		respond500(c, err)
		return
	}

	if err := h.db.WithContext(ctx).Model(&models.UploadSession{}).
		Where("id = ?", session.ID).
		Updates(map[string]any{
			"status":     models.UploadCompleted,
			"asset_id":   rows.assetID,
			"import_id":  rows.importID,
			"updated_at": time.Now().UTC(),
		}).Error; err != nil {
		slog.Error("upload session: could not mark completed",
			"err", err, "upload_id", session.ID, "import_id", rows.importID)
	}
	slog.Info("upload session completed",
		"upload_id", session.ID, "library_id", session.LibraryID,
		"asset_id", rows.assetID, "import_id", rows.importID,
		"size_bytes", session.SizeBytes, "chunks", session.NextChunk)

	session.Status = models.UploadCompleted
	resp := sessionResponse(session)
	resp.AssetID = rows.assetID
	resp.ImportID = rows.importID
	resp.SuggestedMatch = h.suggestMatch(ctx, spec, rows.assetID)
	c.JSON(http.StatusAccepted, resp)
}

func (h *Handler) deleteUploadSession(c *gin.Context) {
	session, ok := h.loadUploadSession(c)
	if !ok {
		return
	}
	if session.Status != models.UploadOpen {
		c.Status(http.StatusNoContent)
		return
	}
	ctx := c.Request.Context()
	up, err := h.storage.ResumeMultipart(ctx, storage.Prefix(session.ObjectPrefix), session.Sha256, session.BackendUploadID)
	if err != nil {
		slog.Warn("upload session: could not reattach to abort",
			"err", err, "upload_id", session.ID)
		up = nil
	}
	h.discardSession(ctx, session, up, models.UploadAborted)
	c.Status(http.StatusNoContent)
}

// discardSession aborts the backing multipart and records the terminal
// status. Both halves are best-effort.
func (h *Handler) discardSession(ctx context.Context, s *models.UploadSession, up storage.MultipartUpload, status string) {
	if up != nil {
		if err := up.Abort(ctx); err != nil {
			slog.Error("upload session: multipart left behind",
				"err", err, "upload_id", s.ID, "backend_upload_id", s.BackendUploadID)
		}
	}
	if err := h.db.WithContext(ctx).Model(&models.UploadSession{}).
		Where("id = ?", s.ID).
		Updates(map[string]any{"status": status, "updated_at": time.Now().UTC()}).Error; err != nil {
		slog.Error("upload session: could not record terminal status",
			"err", err, "upload_id", s.ID, "status", status)
	}
}

func (h *Handler) specForSession(s *models.UploadSession) importSpec {
	spec := newImportSpec(s.LibraryID, s.UserID, s.Filename, s.ContentType)
	if s.TargetAssetID != nil {
		spec.targetAssetID = *s.TargetAssetID
	}
	spec.reqStoreVersion = s.StoreVersion
	spec.sourceKind = s.SourceKind
	spec.sourceRelativePath = s.SourceRelativePath
	return spec
}

// uploadContext resolves the dependencies and request context every session
// endpoint needs.
func (h *Handler) uploadContext(c *gin.Context) (*models.Library, *models.User, bool) {
	if h.storage == nil {
		respond500(c, errStorageNotConfigured)
		return nil, nil, false
	}
	if h.importPool == nil {
		respond500(c, errImportPoolNotConfigured)
		return nil, nil, false
	}
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		respond500(c, errRequestContextIncomplete)
		return nil, nil, false
	}
	return lib, u, true
}

func (h *Handler) loadUploadSession(c *gin.Context) (*models.UploadSession, bool) {
	lib, _, ok := h.uploadContext(c)
	if !ok {
		return nil, false
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return nil, false
	}
	var s models.UploadSession
	err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respond404(c, "upload session not found")
		} else {
			respond500(c, err)
		}
		return nil, false
	}
	return &s, true
}

func (h *Handler) requireOpenSession(c *gin.Context, s *models.UploadSession) bool {
	if s.Status != models.UploadOpen {
		respond409(c, "upload session is "+s.Status)
		return false
	}
	if time.Now().UTC().After(s.ExpiresAt) {
		c.AbortWithStatusJSON(http.StatusGone, gin.H{"error": "upload session has expired"})
		return false
	}
	return true
}

func sessionParts(s *models.UploadSession) []storage.Part {
	var parts []storage.Part
	if err := json.Unmarshal(s.PartsJSON, &parts); err != nil {
		slog.Error("upload session: unreadable part list", "err", err, "upload_id", s.ID)
	}
	return parts
}

func appendPart(raw json.RawMessage, p storage.Part) (json.RawMessage, error) {
	var parts []storage.Part
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &parts); err != nil {
			return nil, err
		}
	}
	parts = append(parts, p)
	return json.Marshal(parts)
}

// resumeUploadHash rebuilds the running SHA-256 from the state stored after
// the previous chunk. Empty state means the upload has not started.
func resumeUploadHash(state []byte) (hash.Hash, error) {
	h := sha256.New()
	if len(state) == 0 {
		return h, nil
	}
	u, ok := h.(encoding.BinaryUnmarshaler)
	if !ok {
		return nil, errHashNotResumable
	}
	if err := u.UnmarshalBinary(state); err != nil {
		return nil, fmt.Errorf("restore upload hash: %w", err)
	}
	return h, nil
}

func marshalUploadHash(h hash.Hash) ([]byte, error) {
	m, ok := h.(encoding.BinaryMarshaler)
	if !ok {
		return nil, errHashNotResumable
	}
	state, err := m.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("save upload hash: %w", err)
	}
	return state, nil
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
