package main

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/licence"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/tags"
)

// reference is one generated asset recorded by its location: a path on the
// NFS store, a store link, or both.
type reference struct {
	Name, Publisher, Category string
	Path, Link                string
	Store                     string
	ReleasedAt                time.Time
	LatestStoreVersion        string
	Licence                   string // detected type: an SPDX id, unity-eula, custom or unknown
	NeedsReview               bool

	// Only a row with a link carries what a store listing gives.
	StoreProductID, PublisherURL string
	StoreUpdatedAt               *time.Time
	Engine, MinEngineVersion     string
	Price, PriceCurrency         string
	Rating                       *float64
	RatingCount                  *int
	StoreDeprecated              bool
}

// nearDuplicateStem names the pack seeded in two versions, so search can be
// tried on a pair that differs only in its version.
const nearDuplicateStem = "Stylized Forest Pack"

// publisher is a made-up publisher and the store it sells on.
type publisher struct {
	name, store, creator string
}

var publishers = []publisher{
	{"Polyforge", "unity", ""},
	{"Lowpoly Lab", "unity", ""},
	{"Northwind Audio", "unity", ""},
	{"Pixel Orchard", "itch", "pixelorchard"},
	{"Kenji Sato", "itch", "kenjisato"},
	{"Moss and Stone", "sketchfab", ""},
	{"Atlas Scans", "sketchfab", ""},
	{"Brightwater Studio", "generic", ""},
}

var categories = []string{
	"Environments", "Characters", "Props", "Audio", "VFX", "UI", "Textures", "Animations",
}

var adjectives = []string{
	"Stylized", "Modular", "Low Poly", "Realistic", "Fantasy", "Sci-Fi", "Medieval", "Desert",
	"Arctic", "Haunted", "Tropical", "Industrial", "Retro", "Cozy", "Ancient", "Neon",
}

var nouns = []string{
	"Forest", "Village", "Dungeon", "Spaceship", "Weapons", "Creatures", "Footsteps", "Explosions",
	"Interface", "Rocks", "Furniture", "Vehicles", "Ruins", "Market", "Cave", "Harbor",
}

var suffixes = []string{"Pack", "Kit", "Collection", "Bundle", "Set"}

// Licence types in rough proportion, covering each licence status.
var licenceTypes = []string{"MIT", "CC0-1.0", "CC-BY-4.0", "unity-eula", "unity-eula", "custom", "unknown"}

// references generates the reference library: about 300 rows, the same on
// every run, a third each path-only, link-only and both.
func references() []reference {
	rng := rand.New(rand.NewPCG(41, 2026))
	paths, links := map[string]bool{}, map[string]bool{}
	var refs []reference

	add := func(name string, pub publisher, kind int) {
		r := reference{
			Name:               name,
			Publisher:          pub.name,
			Category:           categories[rng.IntN(len(categories))],
			Store:              pub.store,
			ReleasedAt:         time.Date(2015+rng.IntN(11), time.Month(1+rng.IntN(12)), 1+rng.IntN(28), 0, 0, 0, 0, time.UTC),
			LatestStoreVersion: fmt.Sprintf("%d.%d.%d", 1+rng.IntN(3), rng.IntN(10), rng.IntN(5)),
			Licence:            licenceTypes[rng.IntN(len(licenceTypes))],
			NeedsReview:        rng.Float64() < 0.15,
		}
		slug := slugify(name)
		if kind != 1 {
			r.Path = fmt.Sprintf("/mnt/assets/%s/%s/%s", pub.name, r.Category, strings.ReplaceAll(name, " ", "_"))
			for n := 2; paths[r.Path]; n++ {
				r.Path = fmt.Sprintf("/mnt/assets/%s/%s/%s_%d", pub.name, r.Category, strings.ReplaceAll(name, " ", "_"), n)
			}
			paths[r.Path] = true
		}
		if kind != 0 {
			for {
				id := 10000 + rng.IntN(290000)
				r.StoreProductID = fmt.Sprint(id)
				r.Link, r.PublisherURL = storeLink(pub, r.Category, slug, id)
				if !links[r.Link] {
					break
				}
			}
			links[r.Link] = true
			updated := r.ReleasedAt.AddDate(0, rng.IntN(36), 0)
			rating := float64(300+rng.IntN(201)) / 100
			count := rng.IntN(2000)
			r.StoreUpdatedAt, r.Rating, r.RatingCount = &updated, &rating, &count
			r.Engine, r.MinEngineVersion = "Unity", []string{"2020.3", "2021.3", "2022.3", "6000.0"}[rng.IntN(4)]
			r.Price, r.PriceCurrency = []string{"0", "4.99", "14.99", "29.99", "59.99"}[rng.IntN(5)], "USD"
			r.StoreDeprecated = rng.Float64() < 0.05
		}
		refs = append(refs, r)
	}

	for i := range 288 {
		name := fmt.Sprintf("%s %s %s",
			adjectives[rng.IntN(len(adjectives))], nouns[rng.IntN(len(nouns))], suffixes[rng.IntN(len(suffixes))])
		add(name, publishers[rng.IntN(len(publishers))], i%3)
	}
	// Names a real library repeats: the same pack from two sellers, a pack
	// in two versions, and a render-pipeline twin.
	add("Modular Dungeon Kit", publishers[0], 2)
	add("Modular Dungeon Kit", publishers[5], 1)
	add(nearDuplicateStem+" v1.2", publishers[1], 0)
	add(nearDuplicateStem+" v1.3", publishers[1], 2)
	add("POLYGON_Fantasy_Village", publishers[0], 0)
	add("POLYGON_Fantasy_Village_URP", publishers[0], 2)
	return refs
}

// storeLink is a listing URL on the publisher's store, with a made-up id, and
// the publisher's page there.
func storeLink(pub publisher, category, slug string, id int) (link, publisherURL string) {
	pubSlug := slugify(pub.name)
	switch pub.store {
	case "unity":
		return fmt.Sprintf("https://assetstore.unity.com/packages/%s/%s-%d", strings.ToLower(category), slug, id),
			fmt.Sprintf("https://assetstore.unity.com/publishers/%d", 1000+len(pub.name))
	case "itch":
		return fmt.Sprintf("https://%s.itch.io/%s-%d", pub.creator, slug, id), fmt.Sprintf("https://%s.itch.io", pub.creator)
	case "sketchfab":
		return fmt.Sprintf("https://sketchfab.com/3d-models/%s-%x", slug, id), "https://sketchfab.com/" + pubSlug
	default:
		return fmt.Sprintf("https://www.fab.com/listings/%s-%d", slug, id), "https://www.fab.com/sellers/" + pubSlug
	}
}

func slugify(s string) string {
	s = strings.ToLower(s)
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, s)
}

// writeReferences records the generated references in the library, in one
// transaction, skipping any whose path or link the library already has. It
// makes no network calls and writes no audit entries.
func writeReferences(db *gorm.DB, libraryID uint) (created, skipped int, err error) {
	err = db.Transaction(func(tx *gorm.DB) error {
		var held []struct {
			LocationPath *string
			SourceURL    string
		}
		if err := tx.Model(&models.Asset{}).Where("library_id = ?", libraryID).
			Select("location_path", "source_url").Find(&held).Error; err != nil {
			return err
		}
		known := map[string]bool{}
		for _, h := range held {
			if h.LocationPath != nil {
				known[*h.LocationPath] = true
			}
			if h.SourceURL != "" {
				known[h.SourceURL] = true
			}
		}
		for _, r := range references() {
			if (r.Path != "" && known[r.Path]) || (r.Link != "" && known[r.Link]) {
				skipped++
				continue
			}
			if err := writeReference(tx, libraryID, r); err != nil {
				return fmt.Errorf("%s: %w", r.Name, err)
			}
			created++
		}
		return nil
	})
	return created, skipped, err
}

func writeReference(tx *gorm.DB, libraryID uint, r reference) error {
	released := r.ReleasedAt
	asset := models.Asset{
		LibraryID:          libraryID,
		Name:               r.Name,
		SourceURL:          r.Link,
		Publisher:          &r.Publisher,
		Store:              &r.Store,
		ReleasedAt:         &released,
		LatestStoreVersion: &r.LatestStoreVersion,
		StoreUpdatedAt:     r.StoreUpdatedAt,
		Rating:             r.Rating,
		RatingCount:        r.RatingCount,
		StoreDeprecated:    r.StoreDeprecated,
		IsReference:        true,
		NeedsReview:        r.NeedsReview,
	}
	if r.Path != "" {
		asset.LocationPath = &r.Path
	}
	for _, f := range []struct {
		dst **string
		v   string
	}{
		{&asset.StoreProductID, r.StoreProductID},
		{&asset.PublisherURL, r.PublisherURL},
		{&asset.Engine, r.Engine},
		{&asset.MinEngineVersion, r.MinEngineVersion},
		{&asset.Price, r.Price},
		{&asset.PriceCurrency, r.PriceCurrency},
	} {
		if f.v != "" {
			v := f.v
			*f.dst = &v
		}
	}
	if err := tx.Create(&asset).Error; err != nil {
		return err
	}
	// GORM leaves a zero-value field with a column default out of the
	// INSERT, and needs_review defaults to true, so false is written here.
	if !r.NeedsReview {
		if err := tx.Model(&asset).Update("needs_review", false).Error; err != nil {
			return err
		}
	}

	attrs := licence.AttributesForType(r.Licence)
	if r.Licence == "custom" {
		no := false
		attrs = licence.Attributes{CommercialUse: &no}
	}
	attrsJSON, err := json.Marshal(attrs)
	if err != nil {
		return err
	}
	lic := models.Licence{
		AssetID:        asset.ID,
		DetectedType:   r.Licence,
		OriginalText:   "Seeded by `just seed`; not a real licence.",
		AttributesJSON: attrsJSON,
	}
	if err := tx.Create(&lic).Error; err != nil {
		return err
	}
	if err := tx.Model(&asset).Update("licence_id", lic.ID).Error; err != nil {
		return err
	}

	// The folder below the publisher is the asset's one tag, as `dmsn scan`
	// gives it. The link is made directly: tags.Attach would audit it.
	tagID, err := tags.CreateOrReusePath(tx, libraryID, r.Category)
	if err != nil {
		return err
	}
	return tx.Create(&models.AssetTag{AssetID: asset.ID, TagID: tagID}).Error
}
