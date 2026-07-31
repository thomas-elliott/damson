package metadata

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ItchAdapter fetches metadata from itch.io listing pages: JSON-LD Product +
// og/twitter meta (itch omits og:title) plus the "More information" table's
// "Asset license" row when the seller set one.
type ItchAdapter struct{}

func (ItchAdapter) Store() string { return StoreItch }

// Fetch retrieves the listing HTML, runs the generic scraper, then enriches
// with the itch-specific licence row.
func (ItchAdapter) Fetch(ctx context.Context, client *Client, rawURL string) (*Metadata, error) {
	resp, err := client.Get(ctx, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("itch: fetch %s: status %d", rawURL, resp.StatusCode)
	}
	m := ParseHTML(resp.Body, resp.FinalURL)
	m.Store = StoreItch
	if m.ListingURL == "" {
		m.ListingURL = resp.FinalURL
	}
	rows := itchInfoRows(resp.Body)
	if label, href := itchLicenceFromRows(rows); label != "" {
		if spdx := firstNonEmpty(spdxFromLicenceURL(href), spdxFromLabel(label)); spdx != "" {
			m.LicenceHint = spdx
		} else if m.LicenceHint == "" {
			m.LicenceHint = label
		}
	}
	m.Tags = dedupeTags(append(m.Tags, itchTagsFromRows(rows)...))
	itchApplyRows(m, rows, resp.FinalURL)
	return m, nil
}

// itchRow is one label/value pair of the "More information" table. Date rows
// render as relative text ("14 days ago") with the real timestamp in the
// abbr title, so both are captured.
type itchRow struct {
	Label     string
	Value     string
	Href      string
	AbbrTitle string
	Anchors   []string
}

// itchInfoRowLabels maps a row label to the typed field it fills. Everything
// else becomes a spec-sheet attribute.
var itchTypedRowLabels = map[string]bool{
	"author": true, "published": true, "release date": true, "updated": true,
	"tags": true, "genre": true, "rating": true,
}

// itchInfoRows walks the listing once and returns every label/value row of the
// info table, in page order. It replaces the per-field walkers that each parsed
// the whole document again.
func itchInfoRows(body []byte) []itchRow {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	// Scope to the info panel. A whole-document walk also harvests the
	// "More information" disclosure wrapper — whose two child divs make it look
	// like a row with the entire table as its value — and the community
	// discussion list, whose post entries use the same two-div shape.
	root := itchInfoPanel(doc)
	if root == nil {
		root = doc
	}
	var rows []itchRow
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "tr" || n.Data == "div") {
			cells := childCells(n)
			if len(cells) >= 2 {
				label := strings.ToLower(strings.TrimSpace(textOf(cells[0])))
				if label != "" && len(label) <= itchMaxLabelLen {
					rows = append(rows, itchRow{
						Label:     label,
						Value:     strings.TrimSpace(textOf(cells[1])),
						Href:      firstHref(cells[1]),
						AbbrTitle: firstAbbrTitle(cells[1]),
						Anchors:   anchorTexts(cells[1]),
					})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return rows
}

// itchInfoPanelClass marks the "More information" table itch renders on every
// listing page.
const itchInfoPanelClass = "game_info_panel_widget"

// itchInfoPanel finds the info-table container, or nil when the page has none.
func itchInfoPanel(n *html.Node) *html.Node {
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "class" {
				for _, cls := range strings.Fields(a.Val) {
					if cls == itchInfoPanelClass {
						return n
					}
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := itchInfoPanel(c); found != nil {
			return found
		}
	}
	return nil
}

// itchMaxLabelLen keeps prose blocks that happen to sit in a two-div layout from
// being mistaken for info rows — real labels are short ("Made with", "Platforms").
const itchMaxLabelLen = 24

// itchApplyRows maps the harvested rows onto typed fields where they fit, and
// records the rest as spec-sheet attributes in page order.
func itchApplyRows(m *Metadata, rows []itchRow, pageURL string) {
	for _, r := range rows {
		switch {
		case r.Label == "author" || r.Label == "publisher":
			if m.Publisher == "" || m.Publisher == "itch.io" {
				m.Publisher = r.Value
			}
			if u := resolveURL(pageURL, r.Href); u != "" {
				m.PublisherURL = u
			}
		case r.Label == "published" || r.Label == "release date":
			if t := itchRowTime(r); t != nil {
				m.ReleasedAt = t
			}
		case r.Label == "updated":
			if t := itchRowTime(r); t != nil {
				m.StoreUpdatedAt = t
			}
		case itchTypedRowLabels[r.Label],
			strings.Contains(r.Label, "license"), strings.Contains(r.Label, "licence"),
			strings.Contains(r.Label, "tags"), strings.Contains(r.Label, "genre"):
			// Handled elsewhere: tags and genre feed Tags, the licence row feeds
			// LicenceHint, and rating comes from the page's JSON-LD
			// aggregateRating.
		default:
			m.AddAttribute(itchAttributeKey(r.Label), itchAttributeLabel(r.Label), r.Value)
		}
	}
}

// itchRowTime prefers the abbr title's absolute timestamp over the row's
// relative text ("14 days ago"), which carries no usable date.
func itchRowTime(r itchRow) *time.Time {
	if t := parseTimestamp(strings.TrimSuffix(strings.TrimSpace(r.AbbrTitle), " UTC")); t != nil {
		return t
	}
	return parseTimestamp(r.Value)
}

// itchAttributeKey turns a row label into a stable snake_case attribute key.
func itchAttributeKey(label string) string {
	var sb strings.Builder
	prevUnderscore := false
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore && sb.Len() > 0 {
				sb.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	return strings.Trim(sb.String(), "_")
}

// itchAttributeLabel restores the row label's display casing, which itchInfoRows
// lowercased for matching.
func itchAttributeLabel(label string) string {
	if label == "" {
		return ""
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

func itchLicenceFromRows(rows []itchRow) (label, href string) {
	for _, r := range rows {
		if strings.Contains(r.Label, "license") || strings.Contains(r.Label, "licence") {
			return r.Value, r.Href
		}
	}
	return "", ""
}

func itchTagsFromRows(rows []itchRow) []string {
	var tags []string
	for _, r := range rows {
		if strings.Contains(r.Label, "tags") || strings.Contains(r.Label, "genre") {
			tags = append(tags, r.Anchors...)
		}
	}
	return tags
}

// itchTags reads the "Tags" and "Genre" rows of the info table, collecting the
// anchor texts itch renders one link per tag.
func itchTags(body []byte) []string {
	return itchTagsFromRows(itchInfoRows(body))
}

// firstAbbrTitle returns the title attribute of the first abbr descendant —
// where itch keeps the absolute timestamp behind its relative date text.
func firstAbbrTitle(n *html.Node) string {
	var title string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if title != "" {
			return
		}
		if node.Type == html.ElementNode && node.Data == "abbr" {
			for _, a := range node.Attr {
				if a.Key == "title" {
					title = a.Val
					return
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return title
}

// anchorTexts returns the trimmed text of every anchor descendant of n.
func anchorTexts(n *html.Node) []string {
	var out []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			if t := strings.TrimSpace(textOf(node)); t != "" {
				out = append(out, t)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

// itchLicence scans the info table for the "Asset license" (or "License") row
// and returns its label text and any anchor href.
func itchLicence(body []byte) (label, href string) {
	return itchLicenceFromRows(itchInfoRows(body))
}

// childCells returns the direct td/div children of a row-like node — the
// label/value pair itch renders per info row.
func childCells(n *html.Node) []*html.Node {
	var cells []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "div") {
			cells = append(cells, c)
		}
	}
	return cells
}

func textOf(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(sb.String()), " ")
}

func firstHref(n *html.Node) string {
	var href string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if href != "" {
			return
		}
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, a := range node.Attr {
				if a.Key == "href" {
					href = a.Val
					return
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return href
}

// spdxFromLabel maps a human licence label ("Creative Commons Attribution
// v4.0", "CC0", "MIT License") to an SPDX id where confident.
func spdxFromLabel(label string) string {
	l := strings.ToLower(label)
	switch {
	case strings.Contains(l, "cc0") || strings.Contains(l, "public domain"):
		return "CC0-1.0"
	case strings.Contains(l, "attribution") && strings.Contains(l, "noncommercial") && strings.Contains(l, "sharealike"):
		return "CC-BY-NC-SA-4.0"
	case strings.Contains(l, "attribution") && strings.Contains(l, "noncommercial"):
		return "CC-BY-NC-4.0"
	case strings.Contains(l, "attribution") && strings.Contains(l, "sharealike"):
		return "CC-BY-SA-4.0"
	case strings.Contains(l, "attribution"):
		return "CC-BY-4.0"
	case strings.Contains(l, "mit"):
		return "MIT"
	case strings.Contains(l, "apache"):
		return "Apache-2.0"
	}
	return ""
}
