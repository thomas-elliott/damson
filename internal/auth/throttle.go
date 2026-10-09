package auth

import (
	"sync"
	"time"
)

// LoginThrottle implements an in-memory token bucket keyed on a string (an IP
// address or an email) for rate-limiting unauthenticated credential
// endpoints — primarily POST /api/auth/login and POST /api/auth/bootstrap.
//
// A single binary, single process design means an in-memory bucket is
// sufficient. Real distributed deployments would put a real rate-limiter at
// the proxy. The cap on tracked keys keeps memory bounded under a hostile
// caller spraying random IPs; once it is reached, new keys are refused.
type LoginThrottle struct {
	burst    int           // tokens at full
	refill   time.Duration // wall time between token refills
	maxKeys  int           // hard cap on tracked keys; new keys refused beyond it
	mu       sync.Mutex
	buckets  map[string]*throttleBucket
	now      func() time.Time
	cleanAt  time.Time
	cleanGap time.Duration
}

type throttleBucket struct {
	tokens   float64
	lastSeen time.Time
}

// NewLoginThrottle constructs a throttle with the supplied burst capacity and
// refill interval. Defaults: 5 burst, one token per minute, 10k keys tracked.
// The combination lets a legitimate user mistype a password ~5 times before
// being told to slow down, and lets an attacker test only ~60 passwords/hour
// per IP and per email — bcrypt at default cost handles the rest.
func NewLoginThrottle(burst int, refill time.Duration) *LoginThrottle {
	if burst <= 0 {
		burst = 5
	}
	if refill <= 0 {
		refill = time.Minute
	}
	return &LoginThrottle{
		burst:    burst,
		refill:   refill,
		maxKeys:  10_000,
		buckets:  make(map[string]*throttleBucket),
		now:      time.Now,
		cleanGap: 10 * time.Minute,
	}
}

// Allow consumes one token from the bucket associated with key. Returns
// false if the bucket is empty (caller should respond 429). An empty key is
// always allowed — the throttle is best-effort and shouldn't block requests
// where we couldn't derive a key.
func (t *LoginThrottle) Allow(key string) bool {
	if key == "" {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	t.maybeCleanLocked(now)
	b, ok := t.buckets[key]
	if !ok {
		// Refuse rather than evict: evicting would let a caller spraying
		// fresh keys clear someone else's lockout. Idle buckets still age out
		// through maybeCleanLocked.
		if len(t.buckets) >= t.maxKeys {
			return false
		}
		b = &throttleBucket{tokens: float64(t.burst), lastSeen: now}
		t.buckets[key] = b
	} else {
		elapsed := now.Sub(b.lastSeen)
		if elapsed > 0 {
			refill := elapsed.Seconds() / t.refill.Seconds()
			b.tokens += refill
			if b.tokens > float64(t.burst) {
				b.tokens = float64(t.burst)
			}
		}
		b.lastSeen = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// maybeCleanLocked drops bucket entries that haven't been touched in a full
// burst-refill window. Called on every Allow but only does work
// `cleanGap` apart so the hot path stays cheap.
func (t *LoginThrottle) maybeCleanLocked(now time.Time) {
	if now.Sub(t.cleanAt) < t.cleanGap {
		return
	}
	t.cleanAt = now
	cutoff := now.Add(-time.Duration(t.burst) * t.refill)
	for k, b := range t.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(t.buckets, k)
		}
	}
}
