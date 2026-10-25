package metadata

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// defaultStoreLookupInterval is the least time between two lookups to one
// store when ClientOptions sets none.
const defaultStoreLookupInterval = 2 * time.Second

// maxPauseWaits bounds how many host pauses FetchWaiting sits out for one
// lookup before giving up.
const maxPauseWaits = 3

// fetcher retrieves and normalises metadata for a single listing URL.
type fetcher interface {
	Fetch(ctx context.Context, client *httpClient, rawURL string) (*Metadata, error)
}

// searcher finds candidate listings by name. Adapters that can't search (or
// the generic scraper) simply don't implement it.
type searcher interface {
	Search(ctx context.Context, client *httpClient, query string) ([]Candidate, error)
	Store() string
}

// Registry maps URL hosts to store adapters and falls back to the generic
// scraper for unknown hosts. Adapters register a host suffix (e.g.
// "assetstore.unity.com"); matching is exact or dot-suffixed.
type Registry struct {
	client    *httpClient
	generic   fetcher
	byHost    map[string]fetcher
	searchers []searcher
	// lookups spaces whole lookups to one store, keyed by storeKey.
	lookups *hostLimiter
}

// newRegistry builds a registry backed by the given client, with lookups to
// one store at least lookupInterval apart (the default when non-positive).
// With no adapters registered it resolves every URL to the generic scraper.
func newRegistry(client *httpClient, lookupInterval time.Duration) *Registry {
	if lookupInterval <= 0 {
		lookupInterval = defaultStoreLookupInterval
	}
	return &Registry{
		client:  client,
		generic: genericFetcher{},
		byHost:  map[string]fetcher{},
		lookups: newHostLimiter(lookupInterval),
	}
}

// Default builds a registry with the built-in store adapters registered by
// host. Callers pass zero options for defaults.
func Default(opts ClientOptions) *Registry {
	r := newRegistry(newHTTPClient(opts), opts.StoreLookupInterval)
	r.registerHost(unityHost, unityAdapter{})
	r.registerHost("sketchfab.com", sketchfabAdapter{})
	r.registerHost("itch.io", itchAdapter{})
	r.registerSearcher(unityAdapter{})
	r.registerSearcher(sketchfabAdapter{})
	r.registerSearcher(itchAdapter{})
	return r
}

// registerHost associates a host suffix with a fetcher adapter.
func (r *Registry) registerHost(hostSuffix string, f fetcher) {
	r.byHost[strings.ToLower(strings.TrimPrefix(hostSuffix, "."))] = f
}

// registerSearcher adds a searcher to the name-search fan-out.
func (r *Registry) registerSearcher(s searcher) {
	r.searchers = append(r.searchers, s)
}

// resolve returns the URL's fetcher and the key its lookups are spaced by: the
// adapter's registered host, so every itch.io subdomain is one store, or the
// URL's own host for the generic scraper.
func (r *Registry) resolve(rawURL string) (fetcher, string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return r.generic, ""
	}
	host := strings.ToLower(u.Hostname())
	for suffix, f := range r.byHost {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return f, suffix
		}
	}
	return r.generic, host
}

// Fetch resolves the adapter for rawURL and fetches, at least the store
// lookup interval after the last lookup to the same store. On adapter failure
// it degrades to the generic scraper, per the layered-fetcher design, and
// marks the result StoreLookupFailed so callers report it; if that also fails
// the error is returned. A host's request to pause is returned as a
// *RateLimitedError rather than fallen back from: the page is on the same
// store, and the caller should wait.
func (r *Registry) Fetch(ctx context.Context, rawURL string) (*Metadata, error) {
	f, key := r.resolve(rawURL)
	r.lookups.wait(ctx, key)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m, err := f.Fetch(ctx, r.client, rawURL)
	if err == nil {
		return m, nil
	}
	var limited *RateLimitedError
	if _, isGeneric := f.(genericFetcher); isGeneric || errors.As(err, &limited) {
		return nil, err
	}
	slog.Warn("metadata: adapter failed, falling back to generic scraper", "url", rawURL, "err", err)
	m, gerr := r.generic.Fetch(ctx, r.client, rawURL)
	if gerr != nil {
		return nil, gerr
	}
	m.StoreLookupFailed = true
	return m, nil
}

// FetchWaiting is Fetch for a caller nobody is waiting on, such as a
// background job: when the store asks for a pause it sits the pause out and
// tries again, up to maxPauseWaits times, instead of failing.
func (r *Registry) FetchWaiting(ctx context.Context, rawURL string) (*Metadata, error) {
	for waits := 0; ; waits++ {
		m, err := r.Fetch(ctx, rawURL)
		var limited *RateLimitedError
		if !errors.As(err, &limited) || waits == maxPauseWaits {
			return m, err
		}
		slog.Info("metadata: waiting out a store pause", "url", rawURL, "until", limited.Until)
		timer := time.NewTimer(time.Until(limited.Until))
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}
}
