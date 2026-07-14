package imports

import (
	"sort"
	"testing"
)

func TestParseStoreVersion(t *testing.T) {
	if _, ok := ParseStoreVersion(""); ok {
		t.Error("empty string should not parse")
	}
	if _, ok := ParseStoreVersion("1.x"); ok {
		t.Error("non-numeric segment should not parse")
	}
	parts, ok := ParseStoreVersion("v1.10")
	if !ok || len(parts) != 2 || parts[0] != 1 || parts[1] != 10 {
		t.Errorf("ParseStoreVersion(v1.10) = %v, %v", parts, ok)
	}
}

func TestCompareStoreVersions(t *testing.T) {
	p := func(s string) []int { v, _ := ParseStoreVersion(s); return v }
	if CompareStoreVersions(p("1.10"), p("1.9")) != 1 {
		t.Error("1.10 should be greater than 1.9")
	}
	if CompareStoreVersions(p("1.1"), p("1.10")) != -1 {
		t.Error("1.1 should be less than 1.10")
	}
	if CompareStoreVersions(p("1.0"), p("1")) != 0 {
		t.Error("1.0 should equal 1")
	}
}

func TestSemanticOrdering(t *testing.T) {
	got := []string{"1.1", "1.10", "1.9"}
	sort.Slice(got, func(i, j int) bool {
		a, _ := ParseStoreVersion(got[i])
		b, _ := ParseStoreVersion(got[j])
		return CompareStoreVersions(a, b) > 0
	})
	want := []string{"1.10", "1.9", "1.1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordering = %v, want %v", got, want)
		}
	}
}
