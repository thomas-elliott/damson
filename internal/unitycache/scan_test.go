package unitycache

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeFile(t *testing.T, root, rel string, size int) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(full, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return full
}

func fixtureCache(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	writeFile(t, root, "A2 Games/3D ModelsCharactersHumanoids/Satomi.unitypackage", 11)
	writeFile(t, root, "Demigiant/ScriptingAnimation/DOTween HOTween v2.unitypackage", 22)
	writeFile(t, root, "Blink/Editor ExtensionsGame ToolkitsRPG Toolkits/RPG Builder.unitypackage", 33)

	// An in-progress download: the .tmp payload and its sidecar must both be
	// skipped, as must a dotted cache directory.
	writeFile(t, root, "Demigiant/ScriptingAnimation/.DOTween-content__0001.tmp", 44)
	writeFile(t, root, "Demigiant/ScriptingAnimation/.DOTween-content__0001.tmp.json", 55)
	writeFile(t, root, ".hidden/Publisher/Hidden.unitypackage", 66)

	writeFile(t, root, "Demigiant/ScriptingAnimation/notes.txt", 77)
	writeFile(t, root, "Loose At Root.unitypackage", 88)

	return root
}

func TestScan(t *testing.T) {
	root := fixtureCache(t)

	entries, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Scan returned %d entries, want 3: %#v", len(entries), entries)
	}

	want := []string{
		"A2 Games/3D ModelsCharactersHumanoids/Satomi.unitypackage",
		"Blink/Editor ExtensionsGame ToolkitsRPG Toolkits/RPG Builder.unitypackage",
		"Demigiant/ScriptingAnimation/DOTween HOTween v2.unitypackage",
	}
	for i, w := range want {
		if entries[i].RelativePath != w {
			t.Errorf("entries[%d].RelativePath = %q, want %q", i, entries[i].RelativePath, w)
		}
	}

	for _, e := range entries {
		if e.Publisher == "" || e.Name == "" {
			t.Errorf("entry %q has no parsed publisher/name: %#v", e.RelativePath, e)
		}
		if e.SizeBytes == 0 {
			t.Errorf("entry %q has zero size", e.RelativePath)
		}
		if e.ModTime.IsZero() {
			t.Errorf("entry %q has zero modtime", e.RelativePath)
		}
	}

	first := entries[0]
	if first.SizeBytes != 11 {
		t.Errorf("first entry size = %d, want 11", first.SizeBytes)
	}
	if got, want := first.CategoryPath(), "3D Models/Characters/Humanoids"; got != want {
		t.Errorf("first entry category = %q, want %q", got, want)
	}
}

func TestScanSkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevation on Windows")
	}
	root := t.TempDir()
	writeFile(t, root, "Publisher/Audio/Real.unitypackage", 10)

	outside := t.TempDir()
	target := writeFile(t, outside, "Elsewhere.unitypackage", 20)
	link := filepath.Join(root, "Publisher", "Audio", "Linked.unitypackage")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	entries, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Scan returned %d entries, want 1 (the symlink must be skipped): %#v", len(entries), entries)
	}
	if entries[0].Name != "Real" {
		t.Errorf("entries[0].Name = %q, want Real", entries[0].Name)
	}
}

func TestScanDepthCap(t *testing.T) {
	root := t.TempDir()
	deep := "Publisher"
	for range MaxScanDepth {
		deep += "/nested"
	}
	writeFile(t, root, deep+"/TooDeep.unitypackage", 10)
	writeFile(t, root, "Publisher/Audio/Shallow.unitypackage", 10)

	entries, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Scan returned %d entries, want 1: %#v", len(entries), entries)
	}
	if entries[0].Name != "Shallow" {
		t.Errorf("entries[0].Name = %q, want Shallow", entries[0].Name)
	}
}

func TestScanRejectsNonDirectory(t *testing.T) {
	root := t.TempDir()
	file := writeFile(t, root, "notadir.txt", 1)

	if _, err := Scan(file); err == nil {
		t.Error("Scan of a file returned no error")
	}
	if _, err := Scan(filepath.Join(root, "missing")); err == nil {
		t.Error("Scan of a missing root returned no error")
	}
}

func TestDefaultRoots(t *testing.T) {
	t.Setenv(CacheRootEnv, "/explicit/root")
	roots := DefaultRoots()
	if len(roots) != 1 || roots[0] != "/explicit/root" {
		t.Fatalf("DefaultRoots with %s set = %#v, want [/explicit/root]", CacheRootEnv, roots)
	}

	t.Setenv(CacheRootEnv, "")
	roots = DefaultRoots()
	if len(roots) == 0 {
		t.Fatal("DefaultRoots with no override returned nothing")
	}
	for _, r := range roots {
		if filepath.Base(r) != CacheDirName {
			t.Errorf("DefaultRoots returned %q, which does not end in %q", r, CacheDirName)
		}
	}
}
