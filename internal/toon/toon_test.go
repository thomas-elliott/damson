package toon

import (
	"strings"
	"testing"
	"time"
)

func TestQuote(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"plain word", "RPG", "RPG"},
		{"inner space", "RPG Builder", "RPG Builder"},
		{"empty", "", `""`},
		{"literal true", "true", `"true"`},
		{"literal false", "false", `"false"`},
		{"literal null", "null", `"null"`},
		{"integer-like", "42", `"42"`},
		{"float-like", "1.5", `"1.5"`},
		{"exponent-like", "1e5", `"1e5"`},
		{"infinity-like", "Inf", `"Inf"`},
		{"leading dash", "-flag", `"-flag"`},
		{"leading hash", "#tag", `"#tag"`},
		{"leading space", " pad", `" pad"`},
		{"trailing space", "pad ", `"pad "`},
		{"contains delimiter", "a,b", `"a,b"`},
		{"contains colon", "key:value", `"key:value"`},
		{"contains brackets", "a[0]", `"a[0]"`},
		{"contains braces", "{x}", `"{x}"`},
		{"contains quote", `say "hi"`, `"say \"hi\""`},
		{"contains backslash", `C:\dir`, `"C:\\dir"`},
		{"contains newline", "a\nb", `"a\nb"`},
		{"contains tab", "a\tb", `"a\tb"`},
		{"unicode is bare", "Café", "Café"},
		{"unicode leading rune not mistaken for dash", "—dash", "—dash"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Quote(tc.in); got != tc.want {
				t.Fatalf("Quote(%q) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	var nilTime *time.Time
	stamp := time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)

	for _, tc := range []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, "null"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int", 42, "42"},
		{"negative int", -7, "-7"},
		{"uint", uint(9), "9"},
		{"int64", int64(1 << 40), "1099511627776"},
		{"float trims zeroes", 1.50, "1.5"},
		{"string", "plain", "plain"},
		{"bytes", []byte("plain"), "plain"},
		{"time is RFC3339 and quoted", stamp, `"2026-08-01T09:30:00Z"`},
		{"nil time pointer", nilTime, "null"},
		{"time pointer", &stamp, `"2026-08-01T09:30:00Z"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Format(tc.in); got != tc.want {
				t.Fatalf("Format(%#v) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestDoc_Table(t *testing.T) {
	doc := (&Doc{}).Table("assets",
		[]string{"id", "name", "publisher", "needs_review"},
		[]map[string]any{
			{"id": 511, "name": "RPG Builder", "publisher": "Blink", "needs_review": false},
			{"id": 512, "name": "DOTween, Pro", "publisher": nil, "needs_review": true},
		})

	want := "assets[2]{id,name,publisher,needs_review}:\n" +
		"  511,RPG Builder,Blink,false\n" +
		"  512,\"DOTween, Pro\",null,true\n"
	if got := doc.String(); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A row missing a declared column renders null rather than shifting the
// remaining values into the wrong columns.
func TestDoc_TableMissingKeyIsNullNotAShift(t *testing.T) {
	doc := (&Doc{}).Table("rows", []string{"a", "b", "c"},
		[]map[string]any{{"a": 1, "c": 3}})

	want := "rows[1]{a,b,c}:\n  1,null,3\n"
	if got := doc.String(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDoc_EmptyForms(t *testing.T) {
	t.Run("table with no rows", func(t *testing.T) {
		got := (&Doc{}).Table("assets", []string{"id"}, nil).String()
		if got != "assets[]:\n" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("explicit empty note", func(t *testing.T) {
		got := (&Doc{}).EmptyTable("assets", "no assets match tag=Characters (412 in library)").String()
		if got != "assets[]: no assets match tag=Characters (412 in library)\n" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("list with no items", func(t *testing.T) {
		if got := (&Doc{}).List("tags", nil).String(); got != "tags[]:\n" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("lines with no items", func(t *testing.T) {
		if got := (&Doc{}).Lines("help", nil).String(); got != "help[]:\n" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestDoc_ListAndObjectAndScalar(t *testing.T) {
	doc := (&Doc{}).
		Scalar("total", 412).
		List("tags", []any{"Characters", "3D Models", "a,b"}).
		Object("asset", []KV{
			{Key: "id", Value: 511},
			{Key: "name", Value: "RPG Builder"},
			{Key: "publisher", Value: nil},
		})

	want := "total: 412\n" +
		"tags[3]: Characters,3D Models,\"a,b\"\n" +
		"asset:\n" +
		"  id: 511\n" +
		"  name: RPG Builder\n" +
		"  publisher: null\n"
	if got := doc.String(); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A help entry carrying an embedded newline must not become two lines: the
// declared count would then disagree with the content.
func TestDoc_LinesNeverExceedTheirDeclaredCount(t *testing.T) {
	got := (&Doc{}).Lines("help", []string{
		"dmsn upload \"~/Packs/Odd\nName\" --as asset",
		"dmsn assets",
	}).String()

	if strings.Count(got, "\n") != 3 {
		t.Fatalf("expected header + 2 entries, got:\n%s", got)
	}
	if !strings.HasPrefix(got, "help[2]:\n") {
		t.Fatalf("declared count wrong:\n%s", got)
	}
	if strings.Contains(got, "Odd\nName") {
		t.Fatalf("newline survived into the block:\n%s", got)
	}
}

func TestDoc_EmptyDocumentRendersNothing(t *testing.T) {
	if got := (&Doc{}).String(); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

// Text is prose: it is quoted even when the bare form would be legal, so a
// message field has a stable boundary regardless of its content.
func TestText_AlwaysQuoted(t *testing.T) {
	if got := Format(Text("plain words")); got != `"plain words"` {
		t.Fatalf("got %s", got)
	}
	if got := Format(Text("")); got != `""` {
		t.Fatalf("got %s", got)
	}
	if got := Format(Text(`has "quotes"`)); got != `"has \"quotes\""` {
		t.Fatalf("got %s", got)
	}
	// The plain string form is still allowed to stay bare.
	if got := Format("plain words"); got != "plain words" {
		t.Fatalf("got %s", got)
	}
}
