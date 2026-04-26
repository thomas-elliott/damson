//go:build ignore

// Regenerate web/e2e/fixtures/tiny.unitypackage. Run from this directory:
//   go run gen-tiny-unitypackage.go
//
// The fixture is a minimal-but-valid Unity .unitypackage with one asset
// (`Assets/E2EFixture.txt`). The format mirrors the test packages built
// in internal/services/extract/unitypackage_test.go: each entry lives
// under a 32-hex-char GUID directory, with `pathname` carrying the
// in-Unity virtual path and `asset` carrying the bytes.
package main

import (
	"archive/tar"
	"compress/gzip"
	"log"
	"os"
)

func main() {
	const guid = "11111111111111111111111111111111"
	const out = "tiny.unitypackage"

	f, err := os.Create(out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	write := func(name string, data []byte) {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Size: int64(len(data)), Mode: 0o644, Typeflag: tar.TypeReg,
		}); err != nil {
			log.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			log.Fatal(err)
		}
	}
	write(guid+"/pathname", []byte("Assets/E2EFixture.txt"))
	write(guid+"/asset", []byte("damson e2e fixture asset payload\n"))

	if err := tw.Close(); err != nil {
		log.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		log.Fatal(err)
	}
}
