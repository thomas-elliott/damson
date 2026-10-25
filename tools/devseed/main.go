// Command devseed fills a dev library with a few sample packs and about 300
// reference assets, so a fresh `just dev` has something to browse at scale. Run
// it as `just seed` while the backend is up. It generates the packs into
// tmp/seed, signs in as the dev admin and uploads them with dmsn, then writes
// the reference assets straight into the dev Postgres. Packs and references
// the library already holds are skipped, so it is safe to run again.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/thomas-elliott/damson/internal/db"
)

const (
	outDir = "tmp/seed"
	server = "http://localhost:8080"
)

func main() {
	log.SetFlags(0)
	email := os.Getenv("DAMSON_LOCAL_ADMIN_EMAIL")
	password := os.Getenv("DAMSON_LOCAL_ADMIN_PASSWORD")

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	packs := []string{
		writeZip("Sample Terrain Textures.zip", terrainTextures()),
		writeZip("Sample UI Icons.zip", uiIcons()),
		writeUnityPackage("Sample Props.unitypackage", props()),
	}

	token, err := login(server, email, password)
	if err != nil {
		log.Fatalf("sign in to %s as %s: %v (is `just dev` running?)", server, email, err)
	}

	args := append([]string{"run", "./cmd/dmsn", "upload", "--server", server, "--token", token, "--skip-existing", "--wait"}, packs...)
	cmd := exec.Command("go", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}

	libraryID, err := currentLibrary(server, token)
	if err != nil {
		log.Fatalf("find %s's library: %v", email, err)
	}
	dsn, err := postgresDSN()
	if err != nil {
		log.Fatal(err)
	}
	gdb, err := db.Open(dsn)
	if err != nil {
		log.Fatalf("open the dev database: %v", err)
	}
	created, skipped, err := writeReferences(gdb, libraryID)
	if err != nil {
		log.Fatalf("write reference assets: %v", err)
	}
	log.Printf("reference assets: %d written, %d already there", created, skipped)
}

// currentLibrary is the signed-in user's current library, as /api/me reports it.
func currentLibrary(server, token string) (uint, error) {
	req, err := http.NewRequest(http.MethodGet, server+"/api/me", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("%s: %s", resp.Status, msg)
	}
	var me struct {
		CurrentLibrary *struct {
			ID uint `json:"id"`
		} `json:"current_library"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return 0, err
	}
	if me.CurrentLibrary == nil {
		return 0, fmt.Errorf("no current library")
	}
	return me.CurrentLibrary.ID, nil
}

// postgresDSN reads the database the server uses, as internal/config does:
// DAMSON_POSTGRES_DSN, or the file DAMSON_POSTGRES_DSN_FILE names.
func postgresDSN() (string, error) {
	direct, file := os.Getenv("DAMSON_POSTGRES_DSN"), os.Getenv("DAMSON_POSTGRES_DSN_FILE")
	switch {
	case direct != "" && file != "":
		return "", fmt.Errorf("set either DAMSON_POSTGRES_DSN or DAMSON_POSTGRES_DSN_FILE, not both")
	case file != "":
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read DAMSON_POSTGRES_DSN_FILE: %w", err)
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	case direct != "":
		return direct, nil
	}
	return "", fmt.Errorf("DAMSON_POSTGRES_DSN (or DAMSON_POSTGRES_DSN_FILE) is not set")
}

func login(server, email, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := http.Post(server+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%s: %s", resp.Status, msg)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Token, nil
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
