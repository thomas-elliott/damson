package api

import (
	"io"
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/models"
)

// object_key is SHA-256 hex (64 chars). Anything else is a 400 — don't let a
// path element with a slash or `..` leak into the storage layer.
var objectKeyRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// getFile streams a blob by object key. Gated on membership + the key being
// referenced by the current library's asset_files — without that check a
// user with a valid key could grab bytes from any library.
func (h *handler) getFile(c *gin.Context) {
	lib := getLibrary(c)
	key := c.Param("objectKey")
	if !objectKeyRE.MatchString(key) {
		respond400(c, asError("invalid object key"))
		return
	}
	// Access control: the key must be referenced by either asset_files or
	// asset_versions.original_package_object_key within the current library.
	var count int64
	err := h.db.WithContext(c.Request.Context()).
		Table("asset_files AS f").
		Joins("JOIN asset_versions v ON v.id = f.asset_version_id").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Where("a.library_id = ? AND f.object_key = ?", lib.ID, key).
		Count(&count).Error
	if err != nil {
		respond500(c)
		return
	}
	if count == 0 {
		// Also check original_package_object_key.
		err = h.db.WithContext(c.Request.Context()).
			Table("asset_versions AS v").
			Joins("JOIN assets a ON a.id = v.asset_id").
			Where("a.library_id = ? AND v.original_package_object_key = ?", lib.ID, key).
			Count(&count).Error
		if err != nil {
			respond500(c)
			return
		}
	}
	if count == 0 {
		respond404(c, "not found")
		return
	}
	r, err := h.storage.GetBlob(c.Request.Context(), key)
	if err != nil {
		respond404(c, "not found")
		return
	}
	defer r.Close()
	c.Header("Cache-Control", "private, max-age=3600")
	if _, err := io.Copy(c.Writer, r); err != nil {
		// Can't reset the status once bytes have gone out; just log.
		return
	}
}

// getThumb streams a thumbnail blob by object key.
func (h *handler) getThumb(c *gin.Context) {
	lib := getLibrary(c)
	key := c.Param("objectKey")
	if !objectKeyRE.MatchString(key) {
		respond400(c, asError("invalid object key"))
		return
	}
	// Thumbnails are referenced by assets.thumbnail_object_key.
	var asset models.Asset
	if err := h.db.WithContext(c.Request.Context()).
		Where("library_id = ? AND thumbnail_object_key = ?", lib.ID, key).
		First(&asset).Error; err != nil {
		if isNotFound(err) {
			respond404(c, "not found")
			return
		}
		respond500(c)
		return
	}
	r, err := h.storage.GetThumb(c.Request.Context(), key)
	if err != nil {
		respond404(c, "not found")
		return
	}
	defer r.Close()
	if asset.ThumbnailContentType != "" {
		c.Header("Content-Type", asset.ThumbnailContentType)
	}
	c.Header("Cache-Control", "private, max-age=86400, immutable")
	if _, err := io.Copy(c.Writer, r); err != nil {
		return
	}
}
