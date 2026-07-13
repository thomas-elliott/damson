package metadata

import "strings"

// spdxFromSlug maps a storefront licence slug (Sketchfab, itch info-table,
// CC URLs) to an SPDX id the licence subsystem understands. Returns "" when
// there's no confident mapping — the caller keeps the raw label as a hint.
func spdxFromSlug(slug string) string {
	s := strings.ToLower(strings.TrimSpace(slug))
	s = strings.TrimSuffix(s, "-4.0")
	s = strings.TrimSuffix(s, "-3.0")
	switch s {
	case "cc0", "cc-0", "publicdomain", "public-domain", "pd":
		return "CC0-1.0"
	case "cc-by", "by":
		return "CC-BY-4.0"
	case "cc-by-sa", "by-sa":
		return "CC-BY-SA-4.0"
	case "cc-by-nc", "by-nc":
		return "CC-BY-NC-4.0"
	case "cc-by-nc-sa", "by-nc-sa":
		return "CC-BY-NC-SA-4.0"
	case "mit":
		return "MIT"
	case "apache-2.0", "apache":
		return "Apache-2.0"
	case "ofl", "ofl-1.1":
		return "OFL-1.1"
	}
	return ""
}

// spdxFromLicenceURL extracts an SPDX id from a Creative Commons (or similar)
// licence URL, e.g. https://creativecommons.org/licenses/by-sa/4.0/.
func spdxFromLicenceURL(raw string) string {
	u := strings.ToLower(raw)
	switch {
	case strings.Contains(u, "publicdomain/zero") || strings.Contains(u, "/cc0"):
		return "CC0-1.0"
	case strings.Contains(u, "/licenses/by-nc-sa"):
		return "CC-BY-NC-SA-4.0"
	case strings.Contains(u, "/licenses/by-nc"):
		return "CC-BY-NC-4.0"
	case strings.Contains(u, "/licenses/by-sa"):
		return "CC-BY-SA-4.0"
	case strings.Contains(u, "/licenses/by"):
		return "CC-BY-4.0"
	}
	return ""
}
