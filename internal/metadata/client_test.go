package metadata

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIPAllowed(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", // loopback
		"10.0.0.5", "172.16.4.4", "192.168.1.1", // private
		"169.254.1.1", // link-local
		"0.0.0.0",     // unspecified
		"224.0.0.1",   // multicast
		"100.64.0.1",  // CGNAT
		"fc00::1",     // ULA
		"fe80::1",     // link-local v6
	}
	for _, s := range blocked {
		if ipAllowed(net.ParseIP(s)) {
			t.Errorf("expected %s to be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"}
	for _, s := range allowed {
		if !ipAllowed(net.ParseIP(s)) {
			t.Errorf("expected %s to be allowed", s)
		}
	}
	if ipAllowed(nil) {
		t.Error("nil IP must be blocked")
	}
}

func TestClientGet_BlocksLoopback(t *testing.T) {
	// A real local server the guard must refuse to reach — proves the SSRF
	// dialer blocks even a live, reachable internal address.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secret"))
	}))
	defer srv.Close()

	c := newHTTPClient(ClientOptions{})
	_, err := c.Get(context.Background(), srv.URL, nil)
	if err == nil {
		t.Fatal("expected loopback fetch to be blocked")
	}
	if !errors.Is(err, errBlockedAddress) && !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected blocked-address error, got %v", err)
	}
}

func TestClientDo_RejectsNonHTTPScheme(t *testing.T) {
	c := newHTTPClient(ClientOptions{})
	_, err := c.Do(context.Background(), http.MethodGet, "ftp://example.com/x", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported scheme") {
		t.Errorf("expected unsupported-scheme error, got %v", err)
	}
}

func TestHostLimiter_SpacesRequests(t *testing.T) {
	l := newHostLimiter(50 * time.Millisecond)
	start := time.Now()
	for i := 0; i < 3; i++ {
		l.wait(context.Background(), "example.com")
	}
	// Three requests at a 50ms gap: the 2nd and 3rd each wait ~50ms, so total
	// elapsed must be at least ~100ms. A no-op limiter would return instantly.
	if elapsed := time.Since(start); elapsed < 90*time.Millisecond {
		t.Errorf("limiter did not space requests: elapsed %v", elapsed)
	}
}

func TestHostLimiter_IndependentHosts(t *testing.T) {
	l := newHostLimiter(200 * time.Millisecond)
	start := time.Now()
	l.wait(context.Background(), "a.com")
	l.wait(context.Background(), "b.com")
	// Different hosts don't gate each other — both should pass promptly.
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("distinct hosts should not block each other: elapsed %v", elapsed)
	}
}
