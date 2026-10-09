package auth

import (
	"testing"
	"time"
)

func TestLoginThrottle_AllowsBurst(t *testing.T) {
	tt := NewLoginThrottle(3, time.Minute)
	tt.now = staticClock(time.Unix(1_000_000, 0))

	for i := 0; i < 3; i++ {
		if !tt.Allow("a") {
			t.Fatalf("attempt %d: expected allow", i)
		}
	}
	if tt.Allow("a") {
		t.Fatal("expected deny once burst is exhausted")
	}
}

func TestLoginThrottle_RefillsOverTime(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	tt := NewLoginThrottle(2, time.Minute)
	tt.now = func() time.Time { return now }

	if !tt.Allow("a") || !tt.Allow("a") {
		t.Fatal("burst should allow first two")
	}
	if tt.Allow("a") {
		t.Fatal("third should deny without refill")
	}
	now = now.Add(2 * time.Minute) // two refilled tokens
	if !tt.Allow("a") || !tt.Allow("a") {
		t.Fatal("post-refill burst should allow two")
	}
	if tt.Allow("a") {
		t.Fatal("post-refill burst exhausted again")
	}
}

func TestLoginThrottle_KeysIsolate(t *testing.T) {
	tt := NewLoginThrottle(1, time.Minute)
	tt.now = staticClock(time.Unix(1_000_000, 0))

	if !tt.Allow("a") || tt.Allow("a") {
		t.Fatal("a: burst+deny expected")
	}
	if !tt.Allow("b") {
		t.Fatal("b: independent bucket should allow")
	}
}

func TestLoginThrottle_EmptyKeyAlwaysAllowed(t *testing.T) {
	tt := NewLoginThrottle(1, time.Minute)
	tt.now = staticClock(time.Unix(1_000_000, 0))
	for i := 0; i < 100; i++ {
		if !tt.Allow("") {
			t.Fatalf("empty key denied at iter %d — bucket should not be tracked", i)
		}
	}
}

// A full key map refuses new keys rather than evicting old ones: eviction
// would let a caller spraying fresh keys clear someone else's lockout.
func TestLoginThrottle_RefusesNewKeysWhenFull(t *testing.T) {
	tt := NewLoginThrottle(1, time.Minute)
	tt.maxKeys = 3
	tt.now = staticClock(time.Unix(1_000_000, 0))

	if !tt.Allow("victim") || tt.Allow("victim") {
		t.Fatal("victim: burst then lockout expected")
	}
	tt.Allow("b")
	tt.Allow("c")

	if tt.Allow("d") {
		t.Fatal("a new key was admitted to a full map")
	}
	if tt.Allow("victim") {
		t.Fatal("the victim's lockout was cleared by filling the map")
	}
}

func staticClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestValidatePassword_Floor(t *testing.T) {
	if err := validatePassword("short"); err == nil {
		t.Fatal("expected error for password < 8 chars")
	}
	if err := validatePassword("longenough"); err != nil {
		t.Fatalf("expected nil for password >= 8 chars, got %v", err)
	}
}
