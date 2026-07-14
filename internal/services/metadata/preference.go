package metadata

import "sort"

// ValidPreferredStore reports whether s names a store the search fan-out knows
// about, so settings can reject typos.
func ValidPreferredStore(s string) bool {
	switch s {
	case StoreUnity, StoreSketchfab, StoreItch, StoreGeneric:
		return true
	}
	return false
}

// SortByPreferredStore stably reorders candidates so the preferred store's hits
// come first, preserving each store's own relevance order. An empty or unknown
// preferred store leaves the order unchanged.
func SortByPreferredStore(cands []Candidate, preferred string) {
	if preferred == "" {
		return
	}
	sort.SliceStable(cands, func(i, j int) bool {
		return cands[i].Store == preferred && cands[j].Store != preferred
	})
}
