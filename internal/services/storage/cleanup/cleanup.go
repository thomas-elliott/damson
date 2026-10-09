package cleanup

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

var sweptPrefixes = []string{"blobs", "thumbs", "originals"}

func isSweptPrefix(p string) bool {
	for _, s := range sweptPrefixes {
		if s == p {
			return true
		}
	}
	return false
}

// Candidate is one orphaned object surfaced by a scan: present in the store but
// referenced by no live DB row and older than the minimum age.
type Candidate struct {
	Prefix       string
	Key          string
	Size         int64
	LastModified time.Time
	Meta         storage.ObjectMeta
}

// Key identifies a single object for deletion.
type Key struct {
	Prefix string
	Key    string
}

// SkippedObject is a requested deletion that was not performed, with the reason.
type SkippedObject struct {
	Prefix string
	Key    string
	Reason string
}

// DeleteResult reports the outcome of a DeleteOrphans call.
type DeleteResult struct {
	Deleted    []Candidate
	Skipped    []SkippedObject
	FreedBytes int64
}

// FindOrphans lists every object under the swept prefixes and returns those
// referenced by no live DB row and last modified before now-minAge. The age
// filter is the safety guard against deleting in-flight uploads whose DB rows
// have not yet committed.
func FindOrphans(ctx context.Context, store storage.Service, db *gorm.DB, minAge time.Duration, now time.Time) ([]Candidate, error) {
	known, err := knownKeys(ctx, db)
	if err != nil {
		return nil, err
	}
	cutoff := now.Add(-minAge)

	var out []Candidate
	for _, prefix := range sweptPrefixes {
		objs, err := store.List(ctx, prefix)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", prefix, err)
		}
		for _, o := range objs {
			if _, ref := known[prefix][o.Key]; ref {
				continue
			}
			if o.LastModified.After(cutoff) {
				continue
			}
			// Metadata is a display aid; a stat failure (e.g. the object vanished
			// mid-scan) must not abort the sweep.
			meta, err := store.StatMeta(ctx, prefix, o.Key)
			if err != nil {
				meta = storage.ObjectMeta{}
			}
			out = append(out, Candidate{
				Prefix:       prefix,
				Key:          o.Key,
				Size:         o.Size,
				LastModified: o.LastModified,
				Meta:         meta,
			})
		}
	}
	return out, nil
}

// DeleteOrphans deletes the requested keys, re-verifying each against a
// freshly-read known set and a fresh stat taken immediately before its
// deletion: a key that became referenced, dropped below the minimum age, or
// already vanished is skipped with a reason rather than deleted. Each
// successful deletion is audited.
//
// The per-key stat is what protects an object an import is deduping onto: the
// reuse touches the object (storage.Service.Touch) before the import's row
// commits, so the object reads as too new even though no row references it yet.
func DeleteOrphans(ctx context.Context, store storage.Service, db *gorm.DB, keys []Key, minAge time.Duration, now time.Time, actor *uint) (DeleteResult, error) {
	known, err := knownKeys(ctx, db)
	if err != nil {
		return DeleteResult{}, err
	}
	cutoff := now.Add(-minAge)

	var res DeleteResult
	skip := func(k Key, reason string) {
		res.Skipped = append(res.Skipped, SkippedObject{Prefix: k.Prefix, Key: k.Key, Reason: reason})
	}
	for _, k := range keys {
		if !isSweptPrefix(k.Prefix) {
			skip(k, "invalid prefix")
			continue
		}
		if _, ref := known[k.Prefix][k.Key]; ref {
			skip(k, "now referenced")
			continue
		}
		info, exists, err := store.Stat(ctx, k.Prefix, k.Key)
		if err != nil {
			skip(k, "stat failed: "+err.Error())
			continue
		}
		if !exists {
			skip(k, "not found")
			continue
		}
		if info.LastModified.After(cutoff) {
			skip(k, "too new")
			continue
		}
		if err := store.Delete(ctx, k.Prefix, k.Key); err != nil {
			skip(k, "delete failed: "+err.Error())
			continue
		}
		// Best-effort: the bucket delete already happened and can't join a DB
		// transaction, so a failed audit write is logged, not fatal.
		if err := audit.Write(db.WithContext(ctx), audit.Entry{
			UserID:     actor,
			ActionType: "admin.storage.orphan_delete",
			TargetType: "storage_object",
			Diff:       map[string]any{"prefix": k.Prefix, "key": k.Key, "size": info.Size},
		}); err != nil {
			slog.Warn("cleanup: audit orphan delete", "err", err, "prefix", k.Prefix, "key", k.Key)
		}
		res.Deleted = append(res.Deleted, Candidate{Prefix: k.Prefix, Key: k.Key, Size: info.Size, LastModified: info.LastModified})
		res.FreedBytes += info.Size
	}
	return res, nil
}

// knownKeys builds the set of object keys referenced by live DB rows, keyed by
// the prefix each column maps to. Purged originals are excluded so a failed
// purge becomes a legitimate cleanup target; loose uploads' original keys live
// in blobs/ and are already covered by asset_files.object_key.
func knownKeys(ctx context.Context, db *gorm.DB) (map[string]map[string]struct{}, error) {
	known := map[string]map[string]struct{}{
		"blobs":     {},
		"thumbs":    {},
		"originals": {},
	}
	add := func(prefix string, keys []string) {
		for _, k := range keys {
			if k != "" {
				known[prefix][k] = struct{}{}
			}
		}
	}
	db = db.WithContext(ctx)

	var fileKeys []string
	if err := db.Model(&models.AssetFile{}).Distinct().Pluck("object_key", &fileKeys).Error; err != nil {
		return nil, fmt.Errorf("pluck file keys: %w", err)
	}
	add("blobs", fileKeys)

	var assetThumbs []string
	if err := db.Model(&models.Asset{}).Distinct().Pluck("thumbnail_object_key", &assetThumbs).Error; err != nil {
		return nil, fmt.Errorf("pluck asset thumbs: %w", err)
	}
	add("thumbs", assetThumbs)

	var galleryKeys []string
	if err := db.Model(&models.AssetImage{}).Distinct().Pluck("object_key", &galleryKeys).Error; err != nil {
		return nil, fmt.Errorf("pluck gallery keys: %w", err)
	}
	add("thumbs", galleryKeys)

	var previewKeys []string
	if err := db.Model(&models.AssetFile{}).Distinct().Pluck("preview_object_key", &previewKeys).Error; err != nil {
		return nil, fmt.Errorf("pluck preview keys: %w", err)
	}
	add("thumbs", previewKeys)

	var origKeys []string
	if err := db.Model(&models.AssetVersion{}).
		Where("original_package_purged = ? AND package_kind IN ?", false, []string{"unitypackage", "zip"}).
		Distinct().Pluck("original_package_object_key", &origKeys).Error; err != nil {
		return nil, fmt.Errorf("pluck original keys: %w", err)
	}
	add("originals", origKeys)

	return known, nil
}
