package axi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/toon"
)

func sampleTable() Table {
	return Table{
		Key:         "assets",
		Columns:     []string{"id", "name", "publisher", "needs_review"},
		Allowed:     []string{"id", "name", "publisher", "needs_review", "description", "store"},
		Truncatable: []string{"description"},
		Rows: []map[string]any{
			{"id": 511, "name": "RPG Builder", "publisher": "Blink", "needs_review": false,
				"description": strings.Repeat("x", 600), "store": "unity"},
			{"id": 512, "name": "DOTween", "publisher": "Demigiant", "needs_review": true,
				"description": "short", "store": "unity"},
		},
		Total: 412,
	}
}

func render(t *testing.T, tbl Table, help []string, o Options) string {
	t.Helper()
	var sb strings.Builder
	if err := RenderTable(&sb, tbl, help, o); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

func TestRenderTable_DefaultProjectionAndAggregates(t *testing.T) {
	got := render(t, sampleTable(), []string{"dmsn asset <id>"}, Options{})

	want := "assets[2]{id,name,publisher,needs_review}:\n" +
		"  511,RPG Builder,Blink,false\n" +
		"  512,DOTween,Demigiant,true\n" +
		"total: 412\n" +
		"shown: 2\n" +
		"help[1]:\n" +
		"  dmsn asset <id>\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	// The wide fields stayed out: a default projection that leaked description
	// would defeat the point of having one.
	if strings.Contains(got, "xxxx") || strings.Contains(got, "unity") {
		t.Fatalf("non-default fields leaked into the default projection:\n%s", got)
	}
}

func TestRenderTable_FieldsOverride(t *testing.T) {
	got := render(t, sampleTable(), nil, Options{Fields: []string{"name", "store"}})

	if !strings.HasPrefix(got, "assets[2]{name,store}:\n") {
		t.Fatalf("projection not applied:\n%s", got)
	}
	if strings.Contains(got, "Blink") {
		t.Fatalf("unrequested field present:\n%s", got)
	}
}

func TestRenderTable_UnknownFieldIsAUsageError(t *testing.T) {
	var sb strings.Builder
	err := RenderTable(&sb, sampleTable(), nil, Options{Fields: []string{"name", "bogus"}})
	if err == nil {
		t.Fatal("expected an error for an unknown field")
	}
	e := AsError(err)
	if e.Code != CodeUsage {
		t.Fatalf("code: got %q want %q", e.Code, CodeUsage)
	}
	if e.ExitCode() != ExitUsage {
		t.Fatalf("exit: got %d want %d", e.ExitCode(), ExitUsage)
	}
	if !strings.Contains(e.Message, "bogus") {
		t.Fatalf("message should name the offending field: %q", e.Message)
	}
	// The help block has to make the next attempt succeed.
	joined := strings.Join(e.Help, " ")
	for _, f := range []string{"id", "name", "publisher", "needs_review", "description", "store"} {
		if !strings.Contains(joined, f) {
			t.Fatalf("help omits the valid field %q: %q", f, joined)
		}
	}
	if sb.Len() != 0 {
		t.Fatalf("nothing should have been written on a usage error, got %q", sb.String())
	}
}

func TestRenderTable_DedupesRequestedFields(t *testing.T) {
	got := render(t, sampleTable(), nil, Options{Fields: []string{"id", "id", "name"}})
	if !strings.HasPrefix(got, "assets[2]{id,name}:\n") {
		t.Fatalf("duplicate column not collapsed:\n%s", got)
	}
}

func TestRenderTable_Truncation(t *testing.T) {
	tbl := sampleTable()

	t.Run("clipped with a total-length note", func(t *testing.T) {
		got := render(t, tbl, nil, Options{Fields: []string{"description"}})
		if !strings.Contains(got, "truncated, 600 chars total — use --full") {
			t.Fatalf("expected the truncation note:\n%s", got)
		}
		// Clipped to the budget, not merely annotated.
		if strings.Contains(got, strings.Repeat("x", TruncateLimit+1)) {
			t.Fatalf("content exceeded the limit:\n%s", got)
		}
	})

	t.Run("--full leaves it whole", func(t *testing.T) {
		got := render(t, tbl, nil, Options{Fields: []string{"description"}, Full: true})
		if strings.Contains(got, "truncated") {
			t.Fatalf("--full should not truncate:\n%s", got)
		}
		if !strings.Contains(got, strings.Repeat("x", 600)) {
			t.Fatalf("--full lost content")
		}
	})

	t.Run("short values are untouched", func(t *testing.T) {
		got := render(t, tbl, nil, Options{Fields: []string{"description"}})
		if !strings.Contains(got, "short") {
			t.Fatalf("a value under the limit should pass through:\n%s", got)
		}
	})

	t.Run("non-truncatable fields are never clipped", func(t *testing.T) {
		wide := sampleTable()
		wide.Truncatable = nil
		got := render(t, wide, nil, Options{Fields: []string{"description"}})
		if strings.Contains(got, "truncated") {
			t.Fatalf("a field not declared truncatable was clipped:\n%s", got)
		}
	})
}

func TestRenderTable_EmptyIsNeverSilent(t *testing.T) {
	t.Run("with a note", func(t *testing.T) {
		tbl := sampleTable()
		tbl.Rows = nil
		tbl.EmptyNote = "no assets match tag_path=Characters (412 assets in library)"
		got := render(t, tbl, nil, Options{})
		if !strings.HasPrefix(got, "assets[]: no assets match tag_path=Characters (412 assets in library)\n") {
			t.Fatalf("got:\n%s", got)
		}
	})

	t.Run("without a note it still states something", func(t *testing.T) {
		tbl := sampleTable()
		tbl.Rows = nil
		got := render(t, tbl, nil, Options{})
		first := strings.SplitN(got, "\n", 2)[0]
		if first == "assets[]:" {
			t.Fatalf("empty result rendered silently: %q", first)
		}
		if !strings.HasPrefix(first, "assets[]: ") {
			t.Fatalf("got %q", first)
		}
	})
}

func TestRenderTable_QuietAndNoHelp(t *testing.T) {
	help := []string{"dmsn asset <id>"}

	noHelp := render(t, sampleTable(), help, Options{NoHelp: true})
	if strings.Contains(noHelp, "help[") {
		t.Fatalf("--no-help left a help block:\n%s", noHelp)
	}
	if !strings.Contains(noHelp, "total: 412") {
		t.Fatalf("--no-help should keep aggregates:\n%s", noHelp)
	}

	quiet := render(t, sampleTable(), help, Options{Quiet: true})
	for _, unwanted := range []string{"help[", "total:", "shown:"} {
		if strings.Contains(quiet, unwanted) {
			t.Fatalf("--quiet left %q:\n%s", unwanted, quiet)
		}
	}
	if !strings.Contains(quiet, "RPG Builder") {
		t.Fatalf("--quiet dropped the data:\n%s", quiet)
	}
}

func TestRenderTable_JSON(t *testing.T) {
	out := render(t, sampleTable(), []string{"dmsn asset <id>"}, Options{Format: FormatJSON})

	var got struct {
		Assets []map[string]any `json:"assets"`
		Total  int              `json:"total"`
		Shown  int              `json:"shown"`
		Help   []string         `json:"help"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if got.Total != 412 || got.Shown != 2 || len(got.Assets) != 2 {
		t.Fatalf("aggregates wrong: %+v", got)
	}
	if len(got.Help) != 1 {
		t.Fatalf("help missing from JSON: %+v", got.Help)
	}
	// The same projection applies — JSON is an encoding switch, not a bypass.
	if _, leaked := got.Assets[0]["store"]; leaked {
		t.Fatalf("JSON ignored the default projection: %+v", got.Assets[0])
	}

	t.Run("quiet drops aggregates here too", func(t *testing.T) {
		q := render(t, sampleTable(), []string{"x"}, Options{Format: FormatJSON, Quiet: true})
		var m map[string]any
		if err := json.Unmarshal([]byte(q), &m); err != nil {
			t.Fatalf("not valid JSON: %v", err)
		}
		for _, k := range []string{"total", "shown", "help"} {
			if _, present := m[k]; present {
				t.Fatalf("--quiet left %q in the JSON payload: %v", k, m)
			}
		}
	})
}

func TestRenderRecord(t *testing.T) {
	rec := Record{
		Key: "asset",
		Fields: []toon.KV{
			{Key: "id", Value: 511},
			{Key: "name", Value: "RPG Builder"},
			{Key: "description", Value: strings.Repeat("y", 500)},
		},
		Allowed:     []string{"id", "name", "description", "licence"},
		Truncatable: []string{"description"},
	}

	t.Run("renders an object block", func(t *testing.T) {
		var sb strings.Builder
		if err := RenderRecord(&sb, rec, nil, Options{}); err != nil {
			t.Fatalf("render: %v", err)
		}
		got := sb.String()
		if !strings.HasPrefix(got, "asset:\n  id: 511\n  name: RPG Builder\n") {
			t.Fatalf("got:\n%s", got)
		}
		if !strings.Contains(got, "truncated, 500 chars total") {
			t.Fatalf("record fields should truncate too:\n%s", got)
		}
	})

	t.Run("projection filters", func(t *testing.T) {
		var sb strings.Builder
		if err := RenderRecord(&sb, rec, nil, Options{Fields: []string{"name"}}); err != nil {
			t.Fatalf("render: %v", err)
		}
		if got := sb.String(); got != "asset:\n  name: RPG Builder\n" {
			t.Fatalf("got %q", got)
		}
	})

	// A field the vocabulary declares but this record omitted must not be
	// reported as an unknown name — that sends the caller chasing a typo.
	t.Run("a declared-but-absent field is accepted", func(t *testing.T) {
		var sb strings.Builder
		if err := RenderRecord(&sb, rec, nil, Options{Fields: []string{"licence"}}); err != nil {
			t.Fatalf("declared field rejected: %v", err)
		}
		if got := sb.String(); got != "asset:\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("a genuinely unknown field is a usage error", func(t *testing.T) {
		var sb strings.Builder
		err := RenderRecord(&sb, rec, nil, Options{Fields: []string{"nope"}})
		if err == nil || AsError(err).Code != CodeUsage {
			t.Fatalf("got %v, want a usage error", err)
		}
	})
}

func TestRenderError(t *testing.T) {
	t.Run("usage errors exit 2 and carry both templates", func(t *testing.T) {
		e := Usagef([]string{
			`dmsn upload "~/Packs/RPG Builder" --as asset`,
			`dmsn upload "~/Packs/RPG Builder" --as assets`,
		}, "%s is a directory; --as is required", "~/Packs/RPG Builder")

		var sb strings.Builder
		code := RenderError(&sb, e, Options{})
		if code != ExitUsage {
			t.Fatalf("exit: got %d want %d", code, ExitUsage)
		}
		want := "error:\n" +
			"  code: usage\n" +
			"  message: \"~/Packs/RPG Builder is a directory; --as is required\"\n" +
			"help[2]:\n" +
			"  dmsn upload \"~/Packs/RPG Builder\" --as asset\n" +
			"  dmsn upload \"~/Packs/RPG Builder\" --as assets\n"
		if got := sb.String(); got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("non-usage errors exit 1", func(t *testing.T) {
		var sb strings.Builder
		if code := RenderError(&sb, Errorf(CodeNotFound, "asset 9 not found"), Options{}); code != ExitError {
			t.Fatalf("exit: got %d want %d", code, ExitError)
		}
		if !strings.Contains(sb.String(), "code: not_found") {
			t.Fatalf("got:\n%s", sb.String())
		}
	})

	t.Run("a plain error is still enveloped, never bare", func(t *testing.T) {
		var sb strings.Builder
		code := RenderError(&sb, errors.New("dial tcp: connection refused"), Options{})
		if code != ExitError {
			t.Fatalf("exit: got %d", code)
		}
		got := sb.String()
		if !strings.HasPrefix(got, "error:\n  code: ") {
			t.Fatalf("unenveloped output:\n%s", got)
		}
		if !strings.Contains(got, "connection refused") {
			t.Fatalf("cause lost:\n%s", got)
		}
	})

	t.Run("json envelope", func(t *testing.T) {
		var sb strings.Builder
		RenderError(&sb, Usagef([]string{"dmsn assets"}, "bad flag"), Options{Format: FormatJSON})
		var got struct {
			Error struct {
				Code    string   `json:"code"`
				Message string   `json:"message"`
				Help    []string `json:"help"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(sb.String()), &got); err != nil {
			t.Fatalf("not valid JSON: %v\n%s", err, sb.String())
		}
		if got.Error.Code != "usage" || got.Error.Message != "bad flag" || len(got.Error.Help) != 1 {
			t.Fatalf("got %+v", got.Error)
		}
	})
}

func TestFromHTTP(t *testing.T) {
	for _, tc := range []struct {
		status   int
		body     string
		wantCode Code
		wantExit int
	}{
		{http.StatusBadRequest, "store_version already exists", CodeUsage, ExitUsage},
		{http.StatusUnauthorized, "unauthorized", CodeAuth, ExitError},
		{http.StatusForbidden, "token is read-only", CodeForbidden, ExitError},
		{http.StatusNotFound, "asset not found", CodeNotFound, ExitError},
		{http.StatusConflict, "duplicate", CodeConflict, ExitError},
		{http.StatusRequestEntityTooLarge, "", CodeTooLarge, ExitError},
		{http.StatusTooManyRequests, "", CodeRateLimited, ExitError},
		{http.StatusInternalServerError, "internal error", CodeServer, ExitError},
		{http.StatusBadGateway, "", CodeServer, ExitError},
	} {
		e := FromHTTP(tc.status, tc.body)
		if e.Code != tc.wantCode {
			t.Fatalf("status %d: code %q want %q", tc.status, e.Code, tc.wantCode)
		}
		if e.ExitCode() != tc.wantExit {
			t.Fatalf("status %d: exit %d want %d", tc.status, e.ExitCode(), tc.wantExit)
		}
		if tc.body != "" && e.Message != tc.body {
			t.Fatalf("status %d: the server's message must survive, got %q", tc.status, e.Message)
		}
		if tc.body == "" && e.Message == "" {
			t.Fatalf("status %d: an empty body must still yield a message", tc.status)
		}
	}

	// A 401 has to tell the caller how to recover.
	if len(FromHTTP(http.StatusUnauthorized, "unauthorized").Help) == 0 {
		t.Fatal("401 should carry recovery help")
	}
}

func TestTruncate(t *testing.T) {
	if got, cut := Truncate("short", 400); cut || got != "short" {
		t.Fatalf("got %q cut=%v", got, cut)
	}
	// Counted in runes, not bytes: a multi-byte string under the rune limit
	// must survive whole.
	multi := strings.Repeat("é", 300)
	if got, cut := Truncate(multi, 400); cut || got != multi {
		t.Fatalf("multi-byte string truncated at %d runes", len([]rune(multi)))
	}
	got, cut := Truncate(strings.Repeat("é", 500), 400)
	if !cut {
		t.Fatal("expected truncation")
	}
	if !strings.Contains(got, "500 chars total") {
		t.Fatalf("note should report the rune length: %q", got)
	}
	if got, cut := Truncate("anything", 0); cut || got != "anything" {
		t.Fatalf("a zero limit must disable truncation, got %q", got)
	}
}

func TestParseFields(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"id", []string{"id"}},
		{"id,name", []string{"id", "name"}},
		{" id , name ", []string{"id", "name"}},
		{"id,,name,", []string{"id", "name"}},
	} {
		got := ParseFields(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("ParseFields(%q) = %v want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("ParseFields(%q) = %v want %v", tc.in, got, tc.want)
			}
		}
	}
}

func TestCount(t *testing.T) {
	if got := Count(1, "asset"); got != "1 asset" {
		t.Fatalf("got %q", got)
	}
	if got := Count(2, "asset"); got != "2 assets" {
		t.Fatalf("got %q", got)
	}
	if got := Count(0, "asset"); got != "0 assets" {
		t.Fatalf("got %q", got)
	}
}
