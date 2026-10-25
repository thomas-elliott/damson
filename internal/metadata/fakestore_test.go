package metadata

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// roundTripperFunc answers requests in-process, so a test can stand in for a
// store without the SSRF guard refusing the loopback address a real
// httptest.Server listens on.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeStoreClient builds a Client whose every request, whatever its host, is
// served by store. Pacing options pass through, so the limiter is the real one.
func fakeStoreClient(t *testing.T, opts ClientOptions, store http.HandlerFunc) *httpClient {
	t.Helper()
	c := newHTTPClient(opts)
	c.http.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		store(rec, r)
		res := rec.Result()
		res.Request = r
		return res, nil
	})
	return c
}

// fakeRegistry is Default's registry over a fake store.
func fakeRegistry(t *testing.T, opts ClientOptions, store http.HandlerFunc) *Registry {
	t.Helper()
	r := Default(opts)
	r.client = fakeStoreClient(t, opts, store)
	return r
}

// unityStore answers Unity's token and search endpoints with coveo (a JSON
// body) and every listing page with page.
func unityStore(t *testing.T, coveo, page []byte) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/coveo/search-token":
			_, _ = w.Write([]byte(`"token"`))
		case r.URL.Path == "/rest/search/v2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(coveo)
		case r.URL.Hostname() == unityHost:
			_, _ = w.Write(page)
		default:
			http.NotFound(w, r)
		}
	}
}
