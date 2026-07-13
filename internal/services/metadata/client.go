package metadata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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
)

// ErrBlockedAddress is returned when a request resolves to an IP the SSRF
// guard refuses to dial (loopback, private, link-local, etc.).
var ErrBlockedAddress = errors.New("metadata: destination address blocked")

// ErrBodyTooLarge is returned when a response body exceeds the size cap.
var ErrBodyTooLarge = errors.New("metadata: response body too large")

// Client is the shared outbound HTTP client for all fetchers. It screens
// destination IPs before dialling (SSRF), caps body size, redirects, and
// time, and rate-limits per host. Construct with NewClient; the zero value
// is not usable.
type Client struct {
	http      *http.Client
	limiter   *hostLimiter
	maxBody   int64
	userAgent string
}

// ClientOptions tunes a Client. Zero values fall back to package defaults.
type ClientOptions struct {
	Timeout     time.Duration
	MaxBody     int64
	MaxRedirect int
	UserAgent   string
	HostMinGap  time.Duration
}

// NewClient builds an SSRF-guarded HTTP client. The custom dialer resolves
// each destination host and refuses non-public IPs, so redirects to internal
// addresses are blocked at connection time, not just on the first hop.
func NewClient(opts ClientOptions) *Client {
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
	return &Client{
		http:      hc,
		limiter:   newHostLimiter(gap),
		maxBody:   maxBody,
		userAgent: ua,
	}
}

// Response is a fetched HTTP response with its body already read (up to the
// size cap) and the connection closed.
type Response struct {
	StatusCode  int
	ContentType string
	FinalURL    string
	Body        []byte
}

// Get performs a rate-limited, SSRF-guarded GET. headers is optional. The
// body is read up to the client's size cap; exceeding it returns
// ErrBodyTooLarge.
func (c *Client) Get(ctx context.Context, rawURL string, headers map[string]string) (*Response, error) {
	return c.Do(ctx, http.MethodGet, rawURL, nil, headers)
}

// Do performs a rate-limited, SSRF-guarded request with an arbitrary method
// and body. Used by adapters that POST (e.g. Unity's Coveo search).
func (c *Client) Do(ctx context.Context, method, rawURL string, body io.Reader, headers map[string]string) (*Response, error) {
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
	c.limiter.wait(ctx, req.URL.Hostname())

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

	// One byte past the cap distinguishes an exactly-at-cap body from an over-cap one.
	limited := io.LimitReader(resp.Body, c.maxBody+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > c.maxBody {
		return nil, ErrBodyTooLarge
	}
	return &Response{
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
				return nil, fmt.Errorf("%w: %s", ErrBlockedAddress, ip.IP)
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("%w: no addresses for %s", ErrBlockedAddress, host)
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

// hostLimiter enforces a minimum interval between requests to the same host,
// shared by the manual endpoints and the import pipeline stage.
type hostLimiter struct {
	minGap time.Duration
	mu     sync.Mutex
	last   map[string]time.Time
}

func newHostLimiter(minGap time.Duration) *hostLimiter {
	return &hostLimiter{minGap: minGap, last: make(map[string]time.Time)}
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
