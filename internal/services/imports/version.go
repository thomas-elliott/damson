package imports

import (
	"strconv"
	"strings"
)

// ParseStoreVersion splits a dotted-integer store version into numeric
// components. ok is false when the string is empty or has a non-numeric
// segment, meaning it can't participate in semantic ordering.
func ParseStoreVersion(s string) (parts []int, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "vV")
	if s == "" {
		return nil, false
	}
	for _, seg := range strings.Split(s, ".") {
		n, err := strconv.Atoi(seg)
		if err != nil {
			return nil, false
		}
		parts = append(parts, n)
	}
	return parts, true
}

// CompareStoreVersions orders two parsed store versions component-wise,
// treating a missing trailing component as zero (so 1.0 == 1). Returns -1 when
// a sorts before b, 1 when after, 0 when equal.
func CompareStoreVersions(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}

// StoreUpdateAvailable reports whether latestStoreVersion is newer than every
// store version the asset already holds, and returns the highest version held.
//
// Comparison is semantic, not textual: "1.10" is newer than "1.9". A version
// either side that isn't dotted-numeric can't be ordered, so it is ignored
// rather than guessed at — an unorderable latest version means "no update
// known", never a false positive.
func StoreUpdateAvailable(latestStoreVersion string, heldVersions []string) (available bool, highestHeld string) {
	latest, ok := ParseStoreVersion(latestStoreVersion)
	if !ok {
		return false, ""
	}
	var best []int
	for _, held := range heldVersions {
		parts, ok := ParseStoreVersion(held)
		if !ok {
			continue
		}
		if best == nil || CompareStoreVersions(parts, best) > 0 {
			best = parts
			highestHeld = strings.TrimSpace(strings.TrimLeft(held, "vV"))
		}
	}
	// An asset with no orderable version at all has nothing to compare, so it
	// isn't reported as out of date; the user hasn't told us what they have.
	if best == nil {
		return false, ""
	}
	return CompareStoreVersions(latest, best) > 0, highestHeld
}
