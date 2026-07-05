package config

import (
	"testing"
	"time"
)

func TestParseOrphanMinAge(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
	}{
		{"", defaultOrphanMinAge},
		{"garbage", defaultOrphanMinAge},
		{"0s", defaultOrphanMinAge},
		{"-5h", defaultOrphanMinAge},
		{"24h", 24 * time.Hour},
		{"336h", 14 * 24 * time.Hour},
	}
	for _, c := range cases {
		if got := parseOrphanMinAge(c.raw); got != c.want {
			t.Fatalf("parseOrphanMinAge(%q): got %v, want %v", c.raw, got, c.want)
		}
	}
	if defaultOrphanMinAge != 7*24*time.Hour {
		t.Fatalf("default min age drifted from 7 days: %v", defaultOrphanMinAge)
	}
}
