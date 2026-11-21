// Command devseed fills a dev library with a few sample packs and about 300
// reference assets, so a fresh `just dev` has something to browse at scale. Run
// it as `just seed` while the backend is up. The seed goes into the sysadmin's
// current library: the dev admin in local mode, or in OIDC mode the first
// person to sign in, so sign in once first. It generates the packs into tmp/seed, mints a short-lived personal
// access token for that user straight in the dev Postgres, uploads the packs
// with dmsn, writes the reference assets into the database, then deletes the
// token. Packs and references the library already holds are skipped, so it is
// safe to run again.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"cmp"
	"compress/gzip"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/db"
	"github.com/thomas-elliott/damson/internal/models"
)

const outDir = "tmp/seed"

func main() {
	log.SetFlags(0)
	server := "http://127.0.0.1:" + cmp.Or(os.Getenv("PORT"), "8080")

	dsn := os.Getenv("DAMSON_POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("DAMSON_POSTGRES_DSN is not set; run this as `just seed`")
	}
	gdb, err := db.Open(dsn)
	if err != nil {
		log.Fatalf("open the dev database: %v", err)
	}
	admin, err := sysadmin(gdb)
	if err != nil {
		log.Fatal(err)
	}
	libraryID := *admin.CurrentLibraryID

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	packs := []string{
		writeZip("Sample Terrain Textures.zip", terrainTextures()),
		writeZip("Sample UI Icons.zip", uiIcons()),
		writeUnityPackage("Sample Props.unitypackage", props()),
	}

	token, cleanup, err := mintToken(gdb, admin.ID, libraryID)
	if err != nil {
		log.Fatal(err)
	}
	args := append([]string{"run", "./cmd/dmsn", "upload", "--server", server, "--token", token, "--skip-existing", "--wait"}, packs...)
	cmd := exec.Command("go", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	uploadErr := cmd.Run()
	cleanup()
	if uploadErr != nil {
		log.Fatalf("upload the sample packs to %s: %v (is `just dev` running?)", server, uploadErr)
	}

	created, skipped, err := writeReferences(gdb, libraryID)
	if err != nil {
		log.Fatalf("write reference assets: %v", err)
	}
	log.Printf("reference assets: %d written, %d already there", created, skipped)
}

// sysadmin is the dev database's first sysadmin, who must have a current
// library. In OIDC mode the first person to sign in becomes both.
func sysadmin(gdb *gorm.DB) (models.User, error) {
	var u models.User
	err := gdb.Where("role = ?", "sysadmin").Order("id").First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return u, errors.New("the dev database has no users yet: start `just dev` (with OIDC, also sign in once in a browser), then run `just seed` again")
	}
	if err != nil {
		return u, fmt.Errorf("find the sysadmin: %w", err)
	}
	if u.CurrentLibraryID == nil {
		return u, fmt.Errorf("%s has no current library", u.Email)
	}
	return u, nil
}

// mintToken writes a write-scoped personal access token for userID, pinned to
// libraryID, that expires in an hour. Only its hash is stored, as the API
// stores one. cleanup deletes the row; the expiry covers a seed that is killed
// before it runs.
func mintToken(gdb *gorm.DB, userID, libraryID uint) (secret string, cleanup func(), err error) {
	secret, hash, prefix, err := auth.GenerateToken()
	if err != nil {
		return "", nil, err
	}
	expires := time.Now().Add(time.Hour)
	row := models.APIToken{
		UserID: userID, LibraryID: libraryID, Label: "just seed",
		TokenHash: hash, Prefix: prefix, Scope: auth.ScopeWrite, ExpiresAt: &expires,
	}
	if err := gdb.Create(&row).Error; err != nil {
		return "", nil, fmt.Errorf("mint a seed token: %w", err)
	}
	cleanup = func() {
		if err := gdb.Delete(&row).Error; err != nil {
			log.Printf("delete the seed token: %v", err)
		}
	}
	return secret, cleanup, nil
}

// file is one entry in a generated pack.
type file struct {
	path string
	data []byte
}

func terrainTextures() []file {
	tints := []struct {
		name string
		c    color.RGBA
	}{
		{"Grass", color.RGBA{86, 140, 60, 255}},
		{"Dirt", color.RGBA{120, 85, 55, 255}},
		{"Rock", color.RGBA{128, 128, 132, 255}},
		{"Sand", color.RGBA{214, 190, 130, 255}},
		{"Snow", color.RGBA{235, 240, 248, 255}},
		{"Water", color.RGBA{50, 110, 170, 255}},
	}
	var files []file
	for i, t := range tints {
		files = append(files, file{"Textures/" + t.name + ".png", noiseTexture(256, t.c, uint64(i))})
	}
	return files
}

func uiIcons() []file {
	shapes := []struct {
		name   string
		inside func(x, y float64) bool
	}{
		{"Circle", func(x, y float64) bool { return x*x+y*y < 0.8 }},
		{"Square", func(x, y float64) bool { return math.Abs(x) < 0.75 && math.Abs(y) < 0.75 }},
		{"Diamond", func(x, y float64) bool { return math.Abs(x)+math.Abs(y) < 0.95 }},
		{"Ring", func(x, y float64) bool { r := x*x + y*y; return r < 0.8 && r > 0.4 }},
		{"Plus", func(x, y float64) bool {
			return (math.Abs(x) < 0.25 || math.Abs(y) < 0.25) && math.Abs(x) < 0.85 && math.Abs(y) < 0.85
		}},
	}
	var files []file
	for _, s := range shapes {
		files = append(files, file{"Icons/" + s.name + ".png", icon(128, s.inside)})
	}
	return files
}

func props() []file {
	return []file{
		{"Assets/SampleProps/Textures/Crate.png", noiseTexture(256, color.RGBA{150, 105, 60, 255}, 10)},
		{"Assets/SampleProps/Textures/Barrel.png", noiseTexture(256, color.RGBA{110, 70, 40, 255}, 11)},
		{"Assets/SampleProps/Textures/Metal.png", noiseTexture(256, color.RGBA{160, 165, 170, 255}, 12)},
		{"Assets/SampleProps/README.txt", []byte("Sample props generated by `just seed`.\n")},
	}
}

// noiseTexture is a square of per-pixel noise around one colour.
func noiseTexture(size int, base color.RGBA, seed uint64) []byte {
	rng := rand.New(rand.NewPCG(seed, seed+1))
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			v := 0.85 + 0.3*rng.Float64()
			img.Set(x, y, color.RGBA{scale(base.R, v), scale(base.G, v), scale(base.B, v), 255})
		}
	}
	return encodePNG(img)
}

func icon(size int, inside func(x, y float64) bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fg := color.RGBA{60, 60, 70, 255}
	for y := range size {
		for x := range size {
			nx := 2*float64(x)/float64(size-1) - 1
			ny := 2*float64(y)/float64(size-1) - 1
			if inside(nx, ny) {
				img.Set(x, y, fg)
			}
		}
	}
	return encodePNG(img)
}

func scale(c uint8, v float64) uint8 {
	return uint8(math.Min(255, float64(c)*v))
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

func writeZip(name string, files []file) string {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f.path)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := w.Write(f.data); err != nil {
			log.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		log.Fatal(err)
	}
	return writeOut(name, buf.Bytes())
}

// writeUnityPackage writes the .unitypackage layout: a gzipped tar with one
// 32-hex-character GUID directory per asset, holding `pathname` (the path
// inside the Unity project) and `asset` (the bytes).
func writeUnityPackage(name string, files []file) string {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for i, f := range files {
		guid := fmt.Sprintf("%032x", i+1)
		writeTar(tw, guid+"/pathname", []byte(f.path))
		writeTar(tw, guid+"/asset", f.data)
	}
	if err := tw.Close(); err != nil {
		log.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		log.Fatal(err)
	}
	return writeOut(name, buf.Bytes())
}

func writeTar(tw *tar.Writer, name string, data []byte) {
	if err := tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(data)), Mode: 0o644, Typeflag: tar.TypeReg}); err != nil {
		log.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		log.Fatal(err)
	}
}

func writeOut(name string, data []byte) string {
	path := filepath.Join(outDir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatal(err)
	}
	return path
}
