// Package toon emits Token-Optimized Object Notation, the output encoding the
// dmsn CLI uses by default. It is emit-only — nothing in damson reads TOON —
// and does no I/O, so the whole encoder is table-testable.
package toon

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Indent is the per-level indentation TOON output uses.
const Indent = "  "

// Delimiter separates values inside tabular rows and primitive arrays.
const Delimiter = ","

// Text is a string that is always quoted, whatever its content. Use it for
// free-form prose — an error message, a note — where a stable, unambiguous
// value boundary matters more than saving two characters.
type Text string

// KV is one key/value pair inside an object block.
type KV struct {
	Key   string
	Value any
}

// Doc is an ordered sequence of top-level blocks. The zero value is ready to
// use; every method appends and returns the receiver so calls chain.
type Doc struct {
	blocks []block
}

type block interface {
	write(sb *strings.Builder, depth int)
}

func (d *Doc) add(b block) *Doc {
	d.blocks = append(d.blocks, b)
	return d
}

// Scalar appends `key: value`.
func (d *Doc) Scalar(key string, value any) *Doc {
	return d.add(scalarBlock{key: key, value: value})
}

// Object appends `key:` followed by one indented line per field. An object
// with no fields is written as `key:` alone.
func (d *Doc) Object(key string, fields []KV) *Doc {
	return d.add(objectBlock{key: key, fields: fields})
}

// Table appends a tabular array: `key[N]{col,…}:` then one indented,
// delimiter-joined line per row. Values are looked up by column name; a
// missing key renders as null.
func (d *Doc) Table(key string, columns []string, rows []map[string]any) *Doc {
	return d.add(tableBlock{key: key, columns: columns, rows: rows})
}

// EmptyTable appends `key[]: note` — the definitive empty state. An empty
// note degrades to a bare `key[]:`.
func (d *Doc) EmptyTable(key, note string) *Doc {
	return d.add(emptyBlock{key: key, note: note})
}

// List appends a primitive array: `key[N]: a,b,c`, or `key[]:` when empty.
func (d *Doc) List(key string, items []any) *Doc {
	return d.add(listBlock{key: key, items: items})
}

// Lines appends `key[N]:` followed by one indented, unencoded line per item.
// Used for help blocks, whose entries are literal commands rather than data.
func (d *Doc) Lines(key string, items []string) *Doc {
	return d.add(linesBlock{key: key, items: items})
}

// String renders the document. Output always ends in a newline unless empty.
func (d *Doc) String() string {
	var sb strings.Builder
	for _, b := range d.blocks {
		b.write(&sb, 0)
	}
	return sb.String()
}

type scalarBlock struct {
	key   string
	value any
}

func (b scalarBlock) write(sb *strings.Builder, depth int) {
	writeIndent(sb, depth)
	sb.WriteString(b.key)
	sb.WriteString(": ")
	sb.WriteString(Format(b.value))
	sb.WriteByte('\n')
}

type objectBlock struct {
	key    string
	fields []KV
}

func (b objectBlock) write(sb *strings.Builder, depth int) {
	writeIndent(sb, depth)
	sb.WriteString(b.key)
	sb.WriteString(":\n")
	for _, f := range b.fields {
		scalarBlock{key: f.Key, value: f.Value}.write(sb, depth+1)
	}
}

type tableBlock struct {
	key     string
	columns []string
	rows    []map[string]any
}

func (b tableBlock) write(sb *strings.Builder, depth int) {
	if len(b.rows) == 0 {
		emptyBlock{key: b.key}.write(sb, depth)
		return
	}
	writeIndent(sb, depth)
	sb.WriteString(b.key)
	sb.WriteByte('[')
	sb.WriteString(strconv.Itoa(len(b.rows)))
	sb.WriteString("]{")
	sb.WriteString(strings.Join(b.columns, Delimiter))
	sb.WriteString("}:\n")
	for _, row := range b.rows {
		writeIndent(sb, depth+1)
		for i, col := range b.columns {
			if i > 0 {
				sb.WriteString(Delimiter)
			}
			sb.WriteString(Format(row[col]))
		}
		sb.WriteByte('\n')
	}
}

type emptyBlock struct {
	key  string
	note string
}

func (b emptyBlock) write(sb *strings.Builder, depth int) {
	writeIndent(sb, depth)
	sb.WriteString(b.key)
	sb.WriteString("[]:")
	if b.note != "" {
		sb.WriteByte(' ')
		sb.WriteString(b.note)
	}
	sb.WriteByte('\n')
}

type listBlock struct {
	key   string
	items []any
}

func (b listBlock) write(sb *strings.Builder, depth int) {
	if len(b.items) == 0 {
		emptyBlock{key: b.key}.write(sb, depth)
		return
	}
	writeIndent(sb, depth)
	sb.WriteString(b.key)
	sb.WriteByte('[')
	sb.WriteString(strconv.Itoa(len(b.items)))
	sb.WriteString("]: ")
	for i, it := range b.items {
		if i > 0 {
			sb.WriteString(Delimiter)
		}
		sb.WriteString(Format(it))
	}
	sb.WriteByte('\n')
}

type linesBlock struct {
	key   string
	items []string
}

func (b linesBlock) write(sb *strings.Builder, depth int) {
	if len(b.items) == 0 {
		emptyBlock{key: b.key}.write(sb, depth)
		return
	}
	writeIndent(sb, depth)
	sb.WriteString(b.key)
	sb.WriteByte('[')
	sb.WriteString(strconv.Itoa(len(b.items)))
	sb.WriteString("]:\n")
	for _, it := range b.items {
		writeIndent(sb, depth+1)
		sb.WriteString(flatten(it))
		sb.WriteByte('\n')
	}
}

// flatten collapses embedded line breaks so one item can never become two
// lines and silently corrupt the declared count.
func flatten(s string) string {
	if !strings.ContainsAny(s, "\n\r") {
		return s
	}
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r'
	}), " ")
}

func writeIndent(sb *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		sb.WriteString(Indent)
	}
}

// Format renders one value as a TOON scalar, quoting when the value would
// otherwise be ambiguous.
func Format(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(t)
	case Text:
		return quoted(string(t))
	case string:
		return Quote(t)
	case []byte:
		return Quote(string(t))
	case int:
		return strconv.Itoa(t)
	case int8:
		return strconv.FormatInt(int64(t), 10)
	case int16:
		return strconv.FormatInt(int64(t), 10)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	case uint8:
		return strconv.FormatUint(uint64(t), 10)
	case uint16:
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case time.Time:
		return Quote(t.UTC().Format(time.RFC3339))
	case *time.Time:
		if t == nil {
			return "null"
		}
		return Quote(t.UTC().Format(time.RFC3339))
	case fmt.Stringer:
		return Quote(t.String())
	default:
		return Quote(fmt.Sprint(t))
	}
}

// Quote returns s as a TOON scalar, wrapping it in double quotes and escaping
// the contents when a bare form would be ambiguous.
func Quote(s string) string {
	if !NeedsQuote(s) {
		return s
	}
	return quoted(s)
}

func quoted(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

// NeedsQuote reports whether s would be ambiguous as a bare TOON scalar.
func NeedsQuote(s string) bool {
	if s == "" {
		return true
	}
	switch s {
	case "true", "false", "null":
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	if s[0] == '-' || s[0] == '#' {
		return true
	}
	if strings.TrimSpace(s) != s {
		return true
	}
	return strings.ContainsAny(s, ":\"\\[]{}\n\r\t"+Delimiter)
}
