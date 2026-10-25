package metadata

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

// storeLog records when each request reached a fake store, by path.
type storeLog struct {
	mu   sync.Mutex
	hits []storeHit
}

type storeHit struct {
	at   time.Time
	path string
}

func (l *storeLog) add(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hits = append(l.hits, storeHit{at: time.Now(), path: path})
}

func (l *storeLog) snapshot() []storeHit {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]storeHit(nil), l.hits...)
}

func TestRetryAfter(t *testing.T) {
	cases := map[string]time.Duration{
		"":        defaultHostPause,
		"soon":    defaultHostPause,
		"0":       defaultHostPause,
		"5":       5 * time.Second,
		"3600":    maxHostPause,
		"9999999": maxHostPause,
	}
	for header, want := range cases {
		if got := retryAfter(header); got != want {
			t.Errorf("retryAfter(%q) = %v, want %v", header, got, want)
		}
	}
	date := time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)
	if got := retryAfter(date); got < 110*time.Second || got > 2*time.Minute {
		t.Errorf("retryAfter(date two minutes on) = %v", got)
	}
}

// A 429 pauses the host for every caller: nothing reaches it until the pause
// is over, while other hosts are unaffected.
func TestClient_429PausesTheHostForEveryCaller(t *testing.T) {
	var log storeLog
	first := true
	c := fakeStoreClient(t, ClientOptions{HostMinGap: time.Millisecond}, func(w http.ResponseWriter, r *http.Request) {
		log.add(r.URL.Host)
		if r.URL.Host == "store.example.com" && first {
			first = false
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	ctx := context.Background()

	_, err := c.Get(ctx, "https://store.example.com/a", nil)
	var limited *RateLimitedError
	if !errors.As(err, &limited) {
		t.Fatalf("a 429 must come back as a RateLimitedError, got %v", err)
	}
	if _, err := c.Get(ctx, "https://store.example.com/images/b.png", nil); !errors.As(err, &limited) {
		t.Fatalf("a request during the pause must be refused, got %v", err)
	}
	if _, err := c.Get(ctx, "https://other.example.com/", nil); err != nil {
		t.Fatalf("another host must not be paused: %v", err)
	}
	if n := len(log.snapshot()); n != 2 {
		t.Fatalf("the store saw %d requests, want 2: the paused request must not be sent", n)
	}

	time.Sleep(time.Until(limited.Until))
	if _, err := c.Get(ctx, "https://store.example.com/a", nil); err != nil {
		t.Fatalf("after Retry-After the host must be reachable again: %v", err)
	}
}

// Whole lookups to one store are spaced by the interval, whatever the gap
// between single requests, while another store is not held up.
func TestRegistryFetch_SpacesLookupsToOneStore(t *testing.T) {
	const interval = 150 * time.Millisecond
	var log storeLog
	coveo := readFixture(t, "unity_coveo.json")
	page := readFixture(t, "unity_listing_page.html")
	unity := unityStore(t, coveo, page)
	r := fakeRegistry(t, ClientOptions{HostMinGap: time.Millisecond, StoreLookupInterval: interval},
		func(w http.ResponseWriter, req *http.Request) {
			log.add(req.URL.Path)
			unity(w, req)
		})
	ctx := context.Background()

	for range 3 {
		if _, err := r.Fetch(ctx, "https://assetstore.unity.com/packages/package/162153"); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
	}
	var starts []time.Time
	for _, h := range log.snapshot() {
		if h.path == "/api/coveo/search-token" {
			starts = append(starts, h.at)
		}
	}
	if len(starts) != 3 {
		t.Fatalf("saw %d lookups, want 3", len(starts))
	}
	for i := 1; i < len(starts); i++ {
		if gap := starts[i].Sub(starts[i-1]); gap < interval-5*time.Millisecond {
			t.Fatalf("lookup %d began %v after the one before, want at least %v", i+1, gap, interval)
		}
	}

	// The fake store has nothing at this host, so the lookup fails; only how
	// long it waited to start matters.
	began := time.Now()
	_, _ = r.Fetch(ctx, "https://other.example.org/pack")
	if waited := time.Since(began); waited > interval/2 {
		t.Fatalf("a lookup to another store waited %v", waited)
	}
}

// The job that hit a 429 waits out Retry-After and then finishes, and nothing
// reaches the store in the meantime, from it or from any other caller.
func TestRegistryFetchWaiting_WaitsOutA429ThenFinishes(t *testing.T) {
	var log storeLog
	var mu sync.Mutex
	limited := false
	coveo := readFixture(t, "unity_coveo.json")
	page := readFixture(t, "unity_listing_page.html")
	unity := unityStore(t, coveo, page)
	r := fakeRegistry(t, ClientOptions{HostMinGap: time.Millisecond, StoreLookupInterval: time.Millisecond},
		func(w http.ResponseWriter, req *http.Request) {
			log.add(req.URL.Path)
			mu.Lock()
			first := !limited && req.URL.Path == "/api/coveo/search-token"
			limited = limited || first
			mu.Unlock()
			if first {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			unity(w, req)
		})
	ctx := context.Background()
	link := "https://assetstore.unity.com/packages/package/162153"

	began := time.Now()
	done := make(chan error, 1)
	var md *Metadata
	go func() {
		var err error
		md, err = r.FetchWaiting(ctx, link)
		done <- err
	}()

	// Another caller during the pause is turned away without a request.
	time.Sleep(200 * time.Millisecond)
	var rl *RateLimitedError
	if _, err := r.Fetch(ctx, link); !errors.As(err, &rl) {
		t.Fatalf("a lookup during the pause must be refused, got %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("FetchWaiting: %v", err)
	}
	if md.Store != StoreUnity || md.StoreLookupFailed {
		t.Fatalf("got store %q failed=%v, want the Unity answer after the pause", md.Store, md.StoreLookupFailed)
	}
	if waited := time.Since(began); waited < time.Second {
		t.Fatalf("finished after %v, before Retry-After was up", waited)
	}
	hits := log.snapshot()
	pauseEnd := hits[0].at.Add(time.Second)
	for _, h := range hits[1:] {
		if h.at.Before(pauseEnd) {
			t.Fatalf("%s reached the store %v into a 1 s pause", h.path, h.at.Sub(hits[0].at))
		}
	}
}
