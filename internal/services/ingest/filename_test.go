package ingest

import "testing"

func TestParseImportFilename(t *testing.T) {
	cases := []struct {
		in          string
		wantName    string
		wantVersion string
	}{
		{"Goblin_Warrior_AnimSet_v1.1.unitypackage", "Goblin Warrior AnimSet", "1.1"},
		{"Goblin_Warrior_AnimSet_v1.1", "Goblin Warrior AnimSet", "1.1"},
		{"pack_v2.zip", "pack", "2"},
		{"FantasyPack.unitypackage", "FantasyPack", ""},
		{"my-cool-asset.zip", "my cool asset", ""},
		{"Building_7.unitypackage", "Building 7", ""},
		{"pack.v3.unitypackage", "pack", "3"},
		{"Env Kit 2.0.zip", "Env Kit", "2.0"},
		{"AnimSet", "AnimSet", ""},
		{"", "", ""},
	}
	for _, tc := range cases {
		name, version := ParseImportFilename(tc.in)
		if name != tc.wantName || version != tc.wantVersion {
			t.Errorf("ParseImportFilename(%q) = (%q, %q), want (%q, %q)",
				tc.in, name, version, tc.wantName, tc.wantVersion)
		}
	}
}

func TestParseImportFilenamePreservesCamelCase(t *testing.T) {
	name, _ := ParseImportFilename("Goblin_Warrior_AnimSet_v1.1.unitypackage")
	if name != "Goblin Warrior AnimSet" {
		t.Fatalf("camelCase word split: got %q", name)
	}
}
