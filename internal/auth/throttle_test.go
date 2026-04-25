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

func TestLoginThrottle_EvictsAtCap(t *testing.T) {
	tt := NewLoginThrottle(1, time.Minute)
	tt.maxKeys = 3
	now := time.Unix(1_000_000, 0)
	tt.now = func() time.Time { return now }

	tt.Allow("a")
	now = now.Add(time.Second)
	tt.Allow("b")
	now = now.Add(time.Second)
	tt.Allow("c")
	// Map at cap. Inserting "d" should evict the oldest ("a").
	now = now.Add(time.Second)
	tt.Allow("d")

	if _, ok := tt.buckets["a"]; ok {
		t.Fatal("expected oldest bucket 'a' evicted")
	}
	if len(tt.buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(tt.buckets))
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
