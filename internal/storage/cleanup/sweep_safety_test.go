package cleanup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/storage"
	"github.com/thomas-elliott/damson/internal/testdb"
)

const minAge = 24 * time.Hour

// age backdates a LocalFS object past minAge, as if it were written long ago.
func age(t *testing.T, root, prefix, key string) {
	t.Helper()
	old := time.Now().Add(-2 * minAge)
	if err := os.Chtimes(filepath.Join(root, prefix, key[:2], key), old, old); err != nil {
		t.Fatalf("backdate %s/%s: %v", prefix, key, err)
	}
}

func newAgedEnv(t *testing.T) (string, *storage.LocalFS) {
	t.Helper()
	root := t.TempDir()
	store, err := storage.NewLocalFS(root)
	if err != nil {
		t.Fatalf("local storage: %v", err)
	}
	return root, store
}

// Gallery images live under thumbs/ and are referenced only by asset_images.
func TestSweep_NeverDeletesAGalleryImage(t *testing.T) {
	root, store := newAgedEnv(t)
	db := testdb.NewPostgres(t)
	ctx := context.Background()
	_, assetID, _ := seedParents(t, db)

	key := putThumb(t, store, ctx, "gallery-image")
	if err := db.Create(&models.AssetImage{AssetID: assetID, SourceURL: "https://store.example/1.png", ObjectKey: key}).Error; err != nil {
		t.Fatalf("seed gallery row: %v", err)
	}
	age(t, root, "thumbs", key)

	cands, err := FindOrphans(ctx, store, db, minAge, time.Now())
	if err != nil {
		t.Fatalf("find orphans: %v", err)
	}
	if candidateSet(cands)["thumbs|"+key] {
		t.Fatal("a gallery image was offered as an orphan")
	}
	res, err := DeleteOrphans(ctx, store, db, []Key{{Prefix: "thumbs", Key: key}}, minAge, time.Now(), nil)
	if err != nil {
		t.Fatalf("delete orphans: %v", err)
	}
	if len(res.Deleted) != 0 {
		t.Fatal("a gallery image was deleted")
	}
}

// An import that dedups onto an old orphan reuses the object before its row
// commits. The sweep must keep the object, whether the reuse lands between
// scan and delete or while the delete itself runs.
func TestSweep_KeepsAnObjectAnImportDedupsOnto(t *testing.T) {
	cases := []struct {
		name string
		// reuseDuringDelete runs the dedup after DeleteOrphans has read the
		// reference set, as it checks the key, rather than before it starts.
		reuseDuringDelete bool
	}{
		{"between scan and delete", false},
		{"while the delete runs", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, local := newAgedEnv(t)
			db := testdb.NewPostgres(t)
			ctx := context.Background()

			payload := []byte("shared texture")
			key, _, err := local.Put(ctx, storage.Blobs, bytes.NewReader(payload), storage.ObjectMeta{})
			if err != nil {
				t.Fatalf("put: %v", err)
			}
			age(t, root, "blobs", key)

			cands, err := FindOrphans(ctx, local, db, minAge, time.Now())
			if err != nil || !candidateSet(cands)["blobs|"+key] {
				t.Fatalf("setup: the old blob should be an orphan candidate (err %v)", err)
			}

			dedup := func() {
				if got, _, err := local.Put(ctx, storage.Blobs, bytes.NewReader(payload), storage.ObjectMeta{}); err != nil || got != key {
					t.Fatalf("dedup put: key %s, err %v", got, err)
				}
			}
			var store storage.Service = local
			if tc.reuseDuringDelete {
				store = &beforeStat{Service: local, hook: dedup}
			} else {
				dedup()
			}

			if _, err := DeleteOrphans(ctx, store, db, []Key{{Prefix: "blobs", Key: key}}, minAge, time.Now(), nil); err != nil {
				t.Fatalf("delete orphans: %v", err)
			}
			if _, exists, _ := local.Stat(ctx, storage.Blobs, key); !exists {
				t.Fatal("the sweep deleted an object an import had just deduped onto")
			}
		})
	}
}

// beforeStat runs hook once, when the sweep first stats a key.
type beforeStat struct {
	storage.Service
	hook func()
}

func (b *beforeStat) Stat(ctx context.Context, prefix storage.Prefix, key string) (storage.ObjectInfo, bool, error) {
	if b.hook != nil {
		b.hook()
		b.hook = nil
	}
	return b.Service.Stat(ctx, prefix, key)
}
