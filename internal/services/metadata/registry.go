package metadata

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
)

// Fetcher retrieves and normalises metadata for a single listing URL.
type Fetcher interface {
	Fetch(ctx context.Context, client *Client, rawURL string) (*Metadata, error)
}

// Searcher finds candidate listings by name. Adapters that can't search (or
// the generic scraper) simply don't implement it.
type Searcher interface {
	Search(ctx context.Context, client *Client, query string) ([]Candidate, error)
	Store() string
}

// Registry maps URL hosts to store adapters and falls back to the generic
// scraper for unknown hosts. Adapters register a host suffix (e.g.
// "assetstore.unity.com"); matching is exact or dot-suffixed.
type Registry struct {
	client    *Client
	generic   Fetcher
	byHost    map[string]Fetcher
	searchers []Searcher
}

// NewRegistry builds a registry backed by the given client. With no adapters
// registered it resolves every URL to the generic scraper.
func NewRegistry(client *Client) *Registry {
	return &Registry{
		client:  client,
		generic: GenericFetcher{},
		byHost:  map[string]Fetcher{},
	}
}

// Client returns the shared SSRF-guarded client so callers (and tests) can
// reuse it.
func (r *Registry) Client() *Client { return r.client }

// RegisterHost associates a host suffix with a fetcher adapter.
func (r *Registry) RegisterHost(hostSuffix string, f Fetcher) {
	r.byHost[strings.ToLower(strings.TrimPrefix(hostSuffix, "."))] = f
}

// RegisterSearcher adds a searcher to the name-search fan-out.
func (r *Registry) RegisterSearcher(s Searcher) {
	r.searchers = append(r.searchers, s)
}

// Searchers returns the registered searchers for the fan-out endpoint.
func (r *Registry) Searchers() []Searcher { return r.searchers }

// FetcherFor resolves the adapter for a URL's host, or the generic scraper.
func (r *Registry) FetcherFor(rawURL string) Fetcher {
	u, err := url.Parse(rawURL)
	if err != nil {
		return r.generic
	}
	host := strings.ToLower(u.Hostname())
	for suffix, f := range r.byHost {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return f
		}
	}
	return r.generic
}

// Fetch resolves the adapter for rawURL and fetches. On adapter failure it
// degrades to the generic scraper, per the layered-fetcher design; if that
// also fails the error is returned.
func (r *Registry) Fetch(ctx context.Context, rawURL string) (*Metadata, error) {
	f := r.FetcherFor(rawURL)
	m, err := f.Fetch(ctx, r.client, rawURL)
	if err == nil {
		return m, nil
	}
	if _, isGeneric := f.(GenericFetcher); isGeneric {
		return nil, err
	}
	slog.Warn("metadata: adapter failed, falling back to generic scraper", "url", rawURL, "err", err)
	return r.generic.Fetch(ctx, r.client, rawURL)
}
