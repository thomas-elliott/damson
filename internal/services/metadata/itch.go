package metadata

import (
	"bytes"
	"context"
	"fmt"
	"strings"

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
	if label, href := itchLicence(resp.Body); label != "" {
		if spdx := firstNonEmpty(spdxFromLicenceURL(href), spdxFromLabel(label)); spdx != "" {
			m.LicenceHint = spdx
		} else if m.LicenceHint == "" {
			m.LicenceHint = label
		}
	}
	m.Tags = dedupeTags(append(m.Tags, itchTags(resp.Body)...))
	return m, nil
}

// itchTags reads the "Tags" and "Genre" rows of the info table, collecting the
// anchor texts itch renders one link per tag.
func itchTags(body []byte) []string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var tags []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "tr" || n.Data == "div") {
			cells := childCells(n)
			if len(cells) >= 2 {
				key := strings.ToLower(strings.TrimSpace(textOf(cells[0])))
				if strings.Contains(key, "tags") || strings.Contains(key, "genre") {
					tags = append(tags, anchorTexts(cells[1])...)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return tags
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
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", ""
	}
	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found {
			return
		}
		if n.Type == html.ElementNode && (n.Data == "tr" || n.Data == "div") {
			cells := childCells(n)
			if len(cells) >= 2 {
				key := strings.ToLower(strings.TrimSpace(textOf(cells[0])))
				if strings.Contains(key, "license") || strings.Contains(key, "licence") {
					label = strings.TrimSpace(textOf(cells[1]))
					href = firstHref(cells[1])
					found = true
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return label, href
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
