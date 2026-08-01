package main

import (
	"flag"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/thomas-elliott/damson/internal/axi"
)

// Server-side pagination bounds, mirrored here so --limit is rejected with a
// usage error rather than being silently clamped by the API.
const (
	defaultLimit = 50
	maxLimit     = 200
)

// pageFlags are the pagination controls every list command carries.
type pageFlags struct {
	limit  int
	offset int
}

func (p *pageFlags) register(fs *flag.FlagSet) {
	fs.IntVar(&p.limit, "limit", defaultLimit, "maximum rows to return")
	fs.IntVar(&p.offset, "offset", 0, "rows to skip")
}

// validate bounds the pagination flags. Split from apply so commands that
// paginate client-side (whole-collection endpoints) still reject bad input.
func (p *pageFlags) validate() error {
	if p.limit < 1 || p.limit > maxLimit {
		return axi.Usagef(
			[]string{fmt.Sprintf("--limit must be between 1 and %d", maxLimit)},
			"--limit %d is out of range", p.limit)
	}
	if p.offset < 0 {
		return axi.Usagef(nil, "--offset cannot be negative")
	}
	return nil
}

func (p *pageFlags) apply(q url.Values) error {
	if err := p.validate(); err != nil {
		return err
	}
	q.Set("limit", strconv.Itoa(p.limit))
	if p.offset > 0 {
		q.Set("offset", strconv.Itoa(p.offset))
	}
	return nil
}

// pageHelp offers the next page whenever one exists, so a truncated list is
// never mistaken for the whole answer.
func (p *pageFlags) pageHelp(command string, shown int, total int64) []string {
	if total <= int64(p.offset+shown) {
		return nil
	}
	return []string{fmt.Sprintf("%s %s --offset %d --limit %d",
		binaryName, command, p.offset+shown, p.limit)}
}

// setIf adds a query parameter only when the value is non-blank, so an unset
// flag is absent rather than an empty filter the server has to interpret.
func setIf(q url.Values, key, value string) {
	if v := strings.TrimSpace(value); v != "" {
		q.Set(key, v)
	}
}

func setBoolIf(q url.Values, key string, value bool) {
	if value {
		q.Set(key, "true")
	}
}

// describeFilters renders the active filters for an empty-state note, so the
// reader can see exactly which constraints produced nothing.
func describeFilters(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		if k != "limit" && k != "offset" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+q.Get(k))
	}
	return strings.Join(parts, " ")
}

// emptyNote states why a list is empty: which filters were applied, and how
// much the library holds in total, so "none" is never ambiguous.
func emptyNote(noun, filters string, libraryTotal int64) string {
	scope := ""
	if libraryTotal >= 0 {
		scope = fmt.Sprintf(" (%s in the library)", axi.Count(int(libraryTotal), noun))
	}
	if filters == "" {
		return fmt.Sprintf("no %ss%s", noun, scope)
	}
	return fmt.Sprintf("no %ss match %s%s", noun, filters, scope)
}
