package metadata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout     = 15 * time.Second
	defaultMaxBody     = 8 << 20 // 8 MiB
	defaultMaxRedirect = 5
	defaultUserAgent   = "damson-metadata/1.0 (+https://github.com/thomas-elliott/damson)"
	defaultHostMinGap  = 500 * time.Millisecond

	// A store that answers 429 or 503 is left alone for its Retry-After, or
	// defaultHostPause when it gives none, and never longer than maxHostPause.
	defaultHostPause = 60 * time.Second
	maxHostPause     = 10 * time.Minute
)

// errBlockedAddress is returned when a request resolves to an IP the SSRF
// guard refuses to dial (loopback, private, link-local, etc.).
var errBlockedAddress = errors.New("metadata: destination address blocked")

// errBodyTooLarge is returned when a response body exceeds the size cap.
var errBodyTooLarge = errors.New("metadata: response body too large")

// RateLimitedError is returned when a host has asked for requests to stop, with
// a 429 or 503, until Until. Every request to that host fails this way, without
// being sent, until then.
type RateLimitedError struct {
	Host  string
	Until time.Time
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("metadata: %s asked for a pause until %s", e.Host, e.Until.UTC().Format(time.RFC3339))
}

// httpClient is the shared outbound HTTP client for all fetchers. It screens
// destination IPs before dialling (SSRF), caps body size, redirects, and
// time, and rate-limits per host. Construct with newHTTPClient; the zero value
// is not usable.
type httpClient struct {
	http      *http.Client
	limiter   *hostLimiter
	maxBody   int64
	userAgent string
}

// ClientOptions tunes the HTTP client. Zero values fall back to package defaults.
type ClientOptions struct {
	Timeout     time.Duration
	MaxBody     int64
	MaxRedirect int
	UserAgent   string
	HostMinGap  time.Duration
	// StoreLookupInterval is the least time between two Registry.Fetch calls
	// to one store, however many requests each makes.
	StoreLookupInterval time.Duration
}

// newHTTPClient builds an SSRF-guarded HTTP client. The custom dialer resolves
// each destination host and refuses non-public IPs, so redirects to internal
// addresses are blocked at connection time, not just on the first hop.
func newHTTPClient(opts ClientOptions) *httpClient {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxBody := opts.MaxBody
	if maxBody <= 0 {
		maxBody = defaultMaxBody
	}
	maxRedirect := opts.MaxRedirect
	if maxRedirect <= 0 {
		maxRedirect = defaultMaxRedirect
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	gap := opts.HostMinGap
	if gap <= 0 {
		gap = defaultHostMinGap
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext:           guardedDialContext(dialer),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	hc := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirect {
				return fmt.Errorf("metadata: stopped after %d redirects", maxRedirect)
			}
			if !isFetchableScheme(req.URL.Scheme) {
				return fmt.Errorf("metadata: redirect to unsupported scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}
	return &httpClient{
		http:      hc,
		limiter:   newHostLimiter(gap),
		maxBody:   maxBody,
		userAgent: ua,
	}
}

// response is a fetched HTTP response with its body already read (up to the
// size cap) and the connection closed.
type response struct {
	StatusCode  int
	ContentType string
	FinalURL    string
	Body        []byte
}

// Get performs a rate-limited, SSRF-guarded GET. headers is optional. The
// body is read up to the client's size cap; exceeding it returns
// errBodyTooLarge.
func (c *httpClient) Get(ctx context.Context, rawURL string, headers map[string]string) (*response, error) {
	return c.Do(ctx, http.MethodGet, rawURL, nil, headers)
}

// Do performs a rate-limited, SSRF-guarded request with an arbitrary method
// and body. Used by adapters that POST (e.g. Unity's Coveo search).
func (c *httpClient) Do(ctx context.Context, method, rawURL string, body io.Reader, headers map[string]string) (*response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	if !isFetchableScheme(req.URL.Scheme) {
		return nil, fmt.Errorf("metadata: unsupported scheme %q", req.URL.Scheme)
	}
	if req.URL.Host == "" {
		return nil, errors.New("metadata: URL missing host")
	}
	host := req.URL.Hostname()
	c.limiter.wait(ctx, host)
	if until, paused := c.limiter.pausedUntil(host); paused {
		return nil, &RateLimitedError{Host: host, Until: until}
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		until := time.Now().Add(retryAfter(resp.Header.Get("Retry-After")))
		c.limiter.pause(host, until)
		slog.Warn("metadata: host asked for a pause", "host", host, "status", resp.StatusCode, "until", until)
		return nil, &RateLimitedError{Host: host, Until: until}
	}

	// One byte past the cap distinguishes an exactly-at-cap body from an over-cap one.
	limited := io.LimitReader(resp.Body, c.maxBody+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > c.maxBody {
		return nil, errBodyTooLarge
	}
	return &response{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		FinalURL:    resp.Request.URL.String(),
		Body:        data,
	}, nil
}

func isFetchableScheme(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "http", "https":
		return true
	}
	return false
}

// guardedDialContext returns a DialContext that resolves the host, refuses to
// dial any non-public IP, and connects directly to a validated IP so a second
// resolution can't swap in an internal address (DNS-rebinding TOCTOU). TLS
// still verifies against the URL hostname — only the dial address is rewritten.
func guardedDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !ipAllowed(ip.IP) {
				return nil, fmt.Errorf("%w: %s", errBlockedAddress, ip.IP)
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("%w: no addresses for %s", errBlockedAddress, host)
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

// ipAllowed reports whether an IP is a public, routable unicast address safe
// to dial. It rejects loopback, private (RFC1918 + fc00::/7), link-local,
// multicast, unspecified, CGNAT (100.64.0.0/10), and anything not global
// unicast.
func ipAllowed(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if !ip.IsGlobalUnicast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
	}
	return true
}

// retryAfter reads a Retry-After header, in seconds or as an HTTP date, and
// bounds it: absent or unreadable is defaultHostPause, and no pause is longer
// than maxHostPause.
func retryAfter(header string) time.Duration {
	d := defaultHostPause
	header = strings.TrimSpace(header)
	if secs, err := strconv.Atoi(header); err == nil && secs > 0 {
		d = time.Duration(secs) * time.Second
	} else if at, err := http.ParseTime(header); err == nil && time.Until(at) > 0 {
		d = time.Until(at)
	}
	return min(d, maxHostPause)
}

// hostLimiter enforces a minimum interval between requests to the same host,
// shared by the manual endpoints and the import pipeline stage, and holds the
// pauses hosts have asked for.
type hostLimiter struct {
	minGap time.Duration
	mu     sync.Mutex
	last   map[string]time.Time
	paused map[string]time.Time
}

func newHostLimiter(minGap time.Duration) *hostLimiter {
	return &hostLimiter{minGap: minGap, last: make(map[string]time.Time), paused: make(map[string]time.Time)}
}

// pause stops requests to host until until. A shorter pause never cuts an
// earlier, longer one short.
func (l *hostLimiter) pause(host string, until time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if until.After(l.paused[host]) {
		l.paused[host] = until
	}
}

// pausedUntil reports whether host is paused now, and until when.
func (l *hostLimiter) pausedUntil(host string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	until, ok := l.paused[host]
	if !ok {
		return time.Time{}, false
	}
	if !time.Now().Before(until) {
		delete(l.paused, host)
		return time.Time{}, false
	}
	return until, true
}

// wait blocks until this host is allowed another request, or ctx expires. It
// reserves the slot before sleeping so concurrent callers to the same host
// queue rather than stampede.
func (l *hostLimiter) wait(ctx context.Context, host string) {
	l.mu.Lock()
	now := time.Now()
	next := now
	if last, ok := l.last[host]; ok {
		if gapEnd := last.Add(l.minGap); gapEnd.After(now) {
			next = gapEnd
		}
	}
	l.last[host] = next
	l.mu.Unlock()

	delay := time.Until(next)
	if delay <= 0 {
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}
