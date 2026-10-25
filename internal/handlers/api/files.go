package api

import (
	"io"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/storage"
)

// object_key is SHA-256 hex (64 chars). Anything else is a 400 — don't let a
// path element with a slash or `..` leak into the storage layer.
var objectKeyRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// allowedThumbContentTypes is the set we'll forward as Content-Type on a
// thumbnail response. The DB column is filled from server-generated
// thumbnails, so the values are server-controlled in practice — but the
// allowlist is defense-in-depth in case a future code path lets user input
// reach it.
var allowedThumbContentTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

// getFile streams a blob by object key. Gated on membership + the key being
// referenced by the current library's asset_files — without that check a
// user with a valid key could grab bytes from any library.
//
// Security headers:
//   - Content-Type is forced to application/octet-stream (caller is
//     responsible for sniffing if they want, but the browser must not).
//   - Content-Disposition: attachment ensures the browser downloads rather
//     than rendering — neutralises stored XSS via uploaded HTML/SVG.
//   - X-Content-Type-Options: nosniff blocks MIME sniffing fallbacks.
//
// Served blobs are hostile content because the SPA keeps its bearer in
// localStorage, where any script on this origin can read it.
func (h *Handler) getFile(c *gin.Context) {
	lib := getLibrary(c)
	key := c.Param("objectKey")
	if !objectKeyRE.MatchString(key) {
		respond400(c, asError("invalid object key"))
		return
	}
	// Access control: the key must be referenced by either asset_files or
	// asset_versions.original_package_object_key within the current library.
	// Pull a candidate filename from asset_files for the Content-Disposition
	// header in the same query.
	type fileRef struct {
		RelativePath string
	}
	var ref fileRef
	err := h.db.WithContext(c.Request.Context()).
		Table("asset_files AS f").
		Select("f.relative_path AS relative_path").
		Joins("JOIN asset_versions v ON v.id = f.asset_version_id").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Where("a.library_id = ? AND f.object_key = ?", lib.ID, key).
		Limit(1).
		Scan(&ref).Error
	if err != nil {
		respond500(c, err)
		return
	}
	fromOriginal := false
	if ref.RelativePath == "" {
		// Also check original_package_object_key — archive originals don't have
		// asset_files rows and live under the originals/ prefix.
		var original struct {
			Name        string
			PackageKind string
		}
		err := h.db.WithContext(c.Request.Context()).
			Table("assets AS a").
			Select("a.name, v.package_kind").
			Joins("JOIN asset_versions v ON v.asset_id = a.id").
			Where("a.library_id = ? AND v.original_package_object_key = ?", lib.ID, key).
			Where(models.DownloadableOriginalSQL("v")).
			Limit(1).
			Scan(&original).Error
		if err != nil {
			respond500(c, err)
			return
		}
		if original.Name == "" {
			respond404(c, "not found")
			return
		}
		ref.RelativePath = original.Name + "." + original.PackageKind
		fromOriginal = true
	}

	prefix := storage.Blobs
	if fromOriginal {
		prefix = storage.Originals
	}
	info, exists, herr := h.storage.Stat(c.Request.Context(), prefix, key)
	if herr != nil || !exists {
		respond404(c, "not found")
		return
	}
	r, err := h.storage.Get(c.Request.Context(), prefix, key)
	if err != nil {
		respond404(c, "not found")
		return
	}
	defer r.Close()
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", `attachment; filename="`+sanitiseFilename(ref.RelativePath)+`"`)
	c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	c.Header("Cache-Control", "private, max-age=3600")
	if _, err := io.Copy(c.Writer, r); err != nil {
		// Can't reset the status once bytes have gone out; just log.
		return
	}
}

// getThumb streams a thumbnail blob by object key. Thumbnails are
// server-generated images so we can confidently set their Content-Type — but
// we still forbid sniffing past it.
// authoriseThumbKey resolves the content type for a thumb object key, matching
// it against an asset cover thumbnail or a stored gallery image within the
// library. It writes the error response and returns ok=false on miss/error.
func (h *Handler) authoriseThumbKey(c *gin.Context, libID uint, key string) (string, bool) {
	var asset models.Asset
	err := h.db.WithContext(c.Request.Context()).
		Where("library_id = ? AND thumbnail_object_key = ?", libID, key).
		First(&asset).Error
	switch {
	case err == nil:
		return asset.ThumbnailContentType, true
	case !isNotFound(err):
		respond500(c, err)
		return "", false
	}
	var img models.AssetImage
	ierr := h.db.WithContext(c.Request.Context()).
		Joins("JOIN assets ON assets.id = asset_images.asset_id").
		Where("assets.library_id = ? AND asset_images.object_key = ?", libID, key).
		First(&img).Error
	switch {
	case ierr == nil:
		return img.ContentType, true
	case !isNotFound(ierr):
		respond500(c, ierr)
		return "", false
	}
	respond404(c, "not found")
	return "", false
}

func (h *Handler) getThumb(c *gin.Context) {
	lib := getLibrary(c)
	key := c.Param("objectKey")
	if !objectKeyRE.MatchString(key) {
		respond400(c, asError("invalid object key"))
		return
	}
	// Authorised keys are an asset's cover thumbnail or a stored gallery image,
	// both scoped to the current library.
	ct, ok := h.authoriseThumbKey(c, lib.ID, key)
	if !ok {
		return
	}
	r, err := h.storage.Get(c.Request.Context(), storage.Thumbs, key)
	if err != nil {
		respond404(c, "not found")
		return
	}
	defer r.Close()
	if !allowedThumbContentTypes[ct] {
		ct = "application/octet-stream"
	}
	c.Header("Content-Type", ct)
	c.Header("Cache-Control", "private, max-age=86400, immutable")
	if _, err := io.Copy(c.Writer, r); err != nil {
		return
	}
}

// getPreview streams a per-file preview derivative by object key. Gated on the
// key being referenced by asset_files.preview_object_key within the current
// library. Derivatives are server-generated images, so Content-Type is set from
// the same allowlist as thumbnails; sniffing past it is still forbidden.
func (h *Handler) getPreview(c *gin.Context) {
	lib := getLibrary(c)
	key := c.Param("objectKey")
	if !objectKeyRE.MatchString(key) {
		respond400(c, asError("invalid object key"))
		return
	}
	var ref struct {
		PreviewObjectKey   string
		PreviewContentType string
	}
	err := h.db.WithContext(c.Request.Context()).
		Table("asset_files AS f").
		Select("f.preview_object_key AS preview_object_key, f.preview_content_type AS preview_content_type").
		Joins("JOIN asset_versions v ON v.id = f.asset_version_id").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Where("a.library_id = ? AND f.preview_object_key = ?", lib.ID, key).
		Limit(1).
		Scan(&ref).Error
	if err != nil {
		respond500(c, err)
		return
	}
	if ref.PreviewObjectKey == "" {
		respond404(c, "not found")
		return
	}
	r, err := h.storage.Get(c.Request.Context(), storage.Thumbs, key)
	if err != nil {
		respond404(c, "not found")
		return
	}
	defer r.Close()
	ct := ref.PreviewContentType
	if !allowedThumbContentTypes[ct] {
		ct = "application/octet-stream"
	}
	c.Header("Content-Type", ct)
	c.Header("Cache-Control", "private, max-age=86400, immutable")
	if _, err := io.Copy(c.Writer, r); err != nil {
		return
	}
}

// sanitiseFilename strips path separators and control characters from a
// filename used in a Content-Disposition header. Quotes become underscores so
// they can't escape the surrounding quoted string.
func sanitiseFilename(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		b := name[i]
		switch {
		case b == '/' || b == '\\':
			// Take the basename — drop everything before the last separator.
			out = out[:0]
		case b == '"' || b < 0x20 || b == 0x7f:
			out = append(out, '_')
		default:
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		return "download"
	}
	return string(out)
}
