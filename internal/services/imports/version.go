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
