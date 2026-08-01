package unitycache

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSplitCategory(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"3D ModelsCharactersHumanoids", []string{"3D Models", "Characters", "Humanoids"}},
		{"Editor ExtensionsGame ToolkitsRPG Toolkits", []string{"Editor Extensions", "Game Toolkits", "RPG Toolkits"}},
		{"Textures MaterialsGUI Skins", []string{"Textures Materials", "GUI Skins"}},
		{"AudioSound FX", []string{"Audio", "Sound FX"}},
		{"ScriptingAnimation", []string{"Scripting", "Animation"}},
		{"3D Models", []string{"3D Models"}},
		{"Templates", []string{"Templates"}},
		{"", nil},
		{"   ", nil},
	}
	for _, c := range cases {
		if got := SplitCategory(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitCategory(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestParseCachePath(t *testing.T) {
	cases := []struct {
		rel           string
		wantPublisher string
		wantCategory  []string
		wantName      string
	}{
		{
			"A2 Games/3D ModelsCharactersHumanoids/Satomi Anime-Style Character For Games And VRChat.unitypackage",
			"A2 Games",
			[]string{"3D Models", "Characters", "Humanoids"},
			"Satomi Anime-Style Character For Games And VRChat",
		},
		{
			"Demigiant/ScriptingAnimation/DOTween HOTween v2.unitypackage",
			"Demigiant",
			[]string{"Scripting", "Animation"},
			"DOTween HOTween v2",
		},
		{
			"Blink/Editor ExtensionsGame ToolkitsRPG Toolkits/RPG Builder.unitypackage",
			"Blink",
			[]string{"Editor Extensions", "Game Toolkits", "RPG Toolkits"},
			"RPG Builder",
		},
		{
			"Publisher/Pack.unitypackage",
			"Publisher",
			nil,
			"Pack",
		},
		{
			`Publisher\AudioSound FX\Pack.unitypackage`,
			"Publisher",
			[]string{"Audio", "Sound FX"},
			"Pack",
		},
		{
			"Publisher/AudioSound FX/Deeper Nesting/Pack.UNITYPACKAGE",
			"Publisher",
			[]string{"Audio", "Sound FX", "Deeper Nesting"},
			"Pack",
		},
	}
	for _, c := range cases {
		got, err := ParseCachePath(c.rel)
		if err != nil {
			t.Errorf("ParseCachePath(%q) errored: %v", c.rel, err)
			continue
		}
		if got.Publisher != c.wantPublisher {
			t.Errorf("ParseCachePath(%q).Publisher = %q, want %q", c.rel, got.Publisher, c.wantPublisher)
		}
		if !reflect.DeepEqual(got.Category, c.wantCategory) {
			t.Errorf("ParseCachePath(%q).Category = %#v, want %#v", c.rel, got.Category, c.wantCategory)
		}
		if got.Name != c.wantName {
			t.Errorf("ParseCachePath(%q).Name = %q, want %q", c.rel, got.Name, c.wantName)
		}
	}
}

func TestParseCachePathRejects(t *testing.T) {
	cases := []struct {
		name string
		rel  string
	}{
		{"empty", ""},
		{"blank", "   "},
		{"absolute", "/Publisher/Pack.unitypackage"},
		{"windows drive", `C:\Publisher\Pack.unitypackage`},
		{"traversal", "Publisher/../../etc/Pack.unitypackage"},
		{"leading traversal", "../Publisher/Pack.unitypackage"},
		{"newline", "Publisher/Au\ndio/Pack.unitypackage"},
		{"nul byte", "Publisher/Audio/Pack\x00.unitypackage"},
		{"too shallow", "Pack.unitypackage"},
		{"wrong extension", "Publisher/Audio/Pack.zip"},
		{"no extension", "Publisher/Audio/Pack"},
		{"empty name", "Publisher/Audio/.unitypackage"},
		{"blank publisher", "   /Audio/Pack.unitypackage"},
		{"over long", strings.Repeat("a", MaxRelativePathLength) + "/Pack.unitypackage"},
	}
	for _, c := range cases {
		if _, err := ParseCachePath(c.rel); !errors.Is(err, ErrInvalidRelativePath) {
			t.Errorf("ParseCachePath(%s) = %v, want ErrInvalidRelativePath", c.name, err)
		}
	}
}

func TestCategoryPath(t *testing.T) {
	p, err := ParseCachePath("Blink/Editor ExtensionsGame Toolkits/RPG Builder.unitypackage")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, want := p.CategoryPath(), "Editor Extensions/Game Toolkits"; got != want {
		t.Errorf("CategoryPath() = %q, want %q", got, want)
	}
	bare, err := ParseCachePath("Publisher/Pack.unitypackage")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := bare.CategoryPath(); got != "" {
		t.Errorf("CategoryPath() with no category = %q, want empty", got)
	}
}
