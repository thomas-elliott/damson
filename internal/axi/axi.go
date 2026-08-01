// Package axi holds the output contract the dmsn CLI is built to: the
// encoding switch, field projection, content truncation, contextual help and
// the structured error envelope with its exit-code mapping.
//
// Pure Go with no I/O beyond the io.Writer it is handed, so every rule here is
// table-testable independently of any command.
package axi

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/thomas-elliott/damson/internal/toon"
)

// TruncateLimit is the character budget for free-text fields before they are
// clipped. Long descriptions and licence bodies are the only realistic way a
// list response blows an agent's context.
const TruncateLimit = 400

// Format selects the output encoding.
type Format string

const (
	FormatTOON Format = "toon"
	FormatJSON Format = "json"
)

// Exit codes. Anything the user could fix by retyping the command is 2; a
// well-formed command that failed is 1.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// Options carry the global flags that shape every command's output.
type Options struct {
	Format Format
	// Fields overrides a command's default projection. Empty means default.
	Fields []string
	// Full disables truncation of free-text fields.
	Full bool
	// NoHelp suppresses the trailing help block.
	NoHelp bool
	// Quiet suppresses help and aggregate lines, leaving only the data.
	Quiet bool
}

func (o Options) format() Format {
	if o.Format == FormatJSON {
		return FormatJSON
	}
	return FormatTOON
}

func (o Options) helpSuppressed() bool { return o.NoHelp || o.Quiet }

// Table is a list result plus the aggregates AXI requires alongside it.
type Table struct {
	// Key names the array, e.g. "assets".
	Key string
	// Columns is the default projection, in output order.
	Columns []string
	// Allowed is every field --fields may name. Empty means Columns only.
	Allowed []string
	// Truncatable names the free-text fields subject to the character budget.
	Truncatable []string
	Rows        []map[string]any
	// Total is the server-reported count before pagination. Negative means
	// unknown, and the total line is omitted.
	Total int
	// EmptyNote is rendered as `key[]: note` when there are no rows. It must
	// say why the result is empty — never leave an empty result silent.
	EmptyNote string
}

// emptyNote guarantees principle 5: an empty result always says something.
// A command that supplies no note still gets a definitive statement rather
// than a bare `key[]:` the reader has to interpret.
func (t Table) emptyNote() string {
	if t.EmptyNote != "" {
		return t.EmptyNote
	}
	return "no " + t.Key + " found"
}

// Record is a single-entity result rendered as an object block.
type Record struct {
	Key    string
	Fields []toon.KV
	// Allowed is every field --fields may name. Empty falls back to the keys
	// present on this record, which under-reports whenever a nullable field
	// was omitted — so commands with optional fields should set it.
	Allowed     []string
	Truncatable []string
}

// RenderTable writes t in the requested encoding, applying field projection
// and truncation. A projection naming an unknown field is a usage error.
func RenderTable(w io.Writer, t Table, help []string, o Options) error {
	columns, err := Project(t, o.Fields)
	if err != nil {
		return err
	}

	rows := projectRows(t, columns, o)

	if o.format() == FormatJSON {
		payload := map[string]any{t.Key: rows}
		if !o.Quiet {
			payload["shown"] = len(rows)
			if t.Total >= 0 {
				payload["total"] = t.Total
			}
		}
		if len(help) > 0 && !o.helpSuppressed() {
			payload["help"] = help
		}
		return writeJSON(w, payload)
	}

	doc := &toon.Doc{}
	if len(rows) == 0 {
		doc.EmptyTable(t.Key, t.emptyNote())
	} else {
		doc.Table(t.Key, columns, rows)
	}
	if !o.Quiet {
		if t.Total >= 0 {
			doc.Scalar("total", t.Total)
		}
		doc.Scalar("shown", len(rows))
	}
	appendHelp(doc, help, o)
	_, err = io.WriteString(w, doc.String())
	return err
}

// RenderRecord writes a single entity as an object block.
func RenderRecord(w io.Writer, r Record, help []string, o Options) error {
	if len(o.Fields) > 0 {
		allowed := r.Allowed
		if len(allowed) == 0 {
			allowed = fieldKeys(r.Fields)
		}
		if err := validateFields(o.Fields, allowed); err != nil {
			return err
		}
	}

	truncatable := set(r.Truncatable)
	keep := set(o.Fields)
	fields := make([]toon.KV, 0, len(r.Fields))
	for _, f := range r.Fields {
		if len(keep) > 0 && !keep[f.Key] {
			continue
		}
		fields = append(fields, toon.KV{
			Key:   f.Key,
			Value: maybeTruncate(f.Value, truncatable[f.Key], o.Full),
		})
	}

	if o.format() == FormatJSON {
		payload := make(map[string]any, len(fields))
		for _, f := range fields {
			payload[f.Key] = f.Value
		}
		out := map[string]any{r.Key: payload}
		if len(help) > 0 && !o.helpSuppressed() {
			out["help"] = help
		}
		return writeJSON(w, out)
	}

	doc := (&toon.Doc{}).Object(r.Key, fields)
	appendHelp(doc, help, o)
	_, err := io.WriteString(w, doc.String())
	return err
}

// Block is one named object inside a multi-block result.
type Block struct {
	Key    string
	Fields []toon.KV
}

// RenderBlocks writes several named objects as one result.
func RenderBlocks(w io.Writer, blocks []Block, help []string, o Options) error {
	return RenderComposite(w, blocks, nil, help, o)
}

// RenderComposite writes object blocks followed by tables as ONE result:
// consecutive blocks in TOON, a single flat object in JSON. Rendering the
// parts with separate calls would emit concatenated JSON documents, which do
// not parse — so a multi-part result has to go through here.
//
// Field projection is deliberately not applied: --fields names a column of
// one table, and a composite has several. Commands wanting projection should
// render a single table instead.
func RenderComposite(w io.Writer, blocks []Block, tables []Table, help []string, o Options) error {
	if o.format() == FormatJSON {
		payload := make(map[string]any, len(blocks)+len(tables)+1)
		for _, b := range blocks {
			fields := make(map[string]any, len(b.Fields))
			for _, f := range b.Fields {
				fields[f.Key] = f.Value
			}
			payload[b.Key] = fields
		}
		for _, t := range tables {
			payload[t.Key] = projectRows(t, t.Columns, o)
		}
		if len(help) > 0 && !o.helpSuppressed() {
			payload["help"] = help
		}
		return writeJSON(w, payload)
	}

	doc := &toon.Doc{}
	for _, b := range blocks {
		doc.Object(b.Key, b.Fields)
	}
	for _, t := range tables {
		rows := projectRows(t, t.Columns, o)
		if len(rows) == 0 {
			doc.EmptyTable(t.Key, t.emptyNote())
		} else {
			doc.Table(t.Key, t.Columns, rows)
		}
	}
	appendHelp(doc, help, o)
	_, err := io.WriteString(w, doc.String())
	return err
}

// projectRows narrows each row to columns, truncating the declared free-text
// fields unless --full is set.
func projectRows(t Table, columns []string, o Options) []map[string]any {
	truncatable := set(t.Truncatable)
	rows := make([]map[string]any, len(t.Rows))
	for i, src := range t.Rows {
		row := make(map[string]any, len(columns))
		for _, c := range columns {
			row[c] = maybeTruncate(src[c], truncatable[c], o.Full)
		}
		rows[i] = row
	}
	return rows
}

// RenderError writes the error envelope and returns the process exit code.
// Errors go to stdout, not stderr: an agent reads one stream, and an error is
// as much a result as a table.
func RenderError(w io.Writer, err error, o Options) int {
	e := AsError(err)
	if o.format() == FormatJSON {
		body := map[string]any{"code": string(e.Code), "message": e.Message}
		if len(e.Help) > 0 && !o.NoHelp {
			body["help"] = e.Help
		}
		_ = writeJSON(w, map[string]any{"error": body})
		return e.ExitCode()
	}
	doc := (&toon.Doc{}).Object("error", []toon.KV{
		{Key: "code", Value: string(e.Code)},
		{Key: "message", Value: toon.Text(e.Message)},
	})
	if len(e.Help) > 0 && !o.NoHelp {
		doc.Lines("help", e.Help)
	}
	_, _ = io.WriteString(w, doc.String())
	return e.ExitCode()
}

// Project resolves the effective column list for t under the requested
// override, validating every name against the table's allowed set.
func Project(t Table, fields []string) ([]string, error) {
	allowed := t.Allowed
	if len(allowed) == 0 {
		allowed = t.Columns
	}
	if len(fields) == 0 {
		return t.Columns, nil
	}
	if err := validateFields(fields, allowed); err != nil {
		return nil, err
	}
	return dedupe(fields), nil
}

func dedupe(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, i := range items {
		if seen[i] {
			continue
		}
		seen[i] = true
		out = append(out, i)
	}
	return out
}

func validateFields(requested, allowed []string) error {
	known := set(allowed)
	var unknown []string
	for _, f := range requested {
		if !known[f] {
			unknown = append(unknown, f)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sorted := append([]string(nil), allowed...)
	sort.Strings(sorted)
	return &Error{
		Code: CodeUsage,
		Message: fmt.Sprintf("unknown %s: %s",
			plural("field", len(unknown)), strings.Join(unknown, ", ")),
		Help: []string{"available fields: " + strings.Join(sorted, ",")},
	}
}

// Truncate clips s to limit characters, appending a note that states the full
// length so the caller knows what they are missing and how to get it.
func Truncate(s string, limit int) (string, bool) {
	runes := []rune(s)
	if limit <= 0 || len(runes) <= limit {
		return s, false
	}
	return string(runes[:limit]) + fmt.Sprintf("… (truncated, %d chars total — use --full)", len(runes)), true
}

func maybeTruncate(v any, truncatable, full bool) any {
	if !truncatable || full {
		return v
	}
	s, ok := v.(string)
	if !ok {
		return v
	}
	out, _ := Truncate(s, TruncateLimit)
	return out
}

func appendHelp(doc *toon.Doc, help []string, o Options) {
	if len(help) == 0 || o.helpSuppressed() {
		return
	}
	doc.Lines("help", help)
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func set(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	m := make(map[string]bool, len(items))
	for _, i := range items {
		m[i] = true
	}
	return m
}

func fieldKeys(fields []toon.KV) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = f.Key
	}
	return out
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// ParseFields splits a comma-separated --fields value, dropping blanks.
func ParseFields(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Count renders an aggregate line's value, e.g. `2 imported, 1 failed`.
func Count(n int, noun string) string {
	return strconv.Itoa(n) + " " + plural(noun, n)
}
