package metadata

import (
	"regexp"
	"strings"
)

// ccVersions are the Creative Commons versions the licence package has
// attributes for. Any other version maps to nothing.
var ccVersions = []string{"4.0", "3.0"}

// spdxFromSlug maps a storefront licence slug (Sketchfab, itch info-table,
// CC URLs) to an SPDX id the licence subsystem understands. Returns "" when
// there's no confident mapping — the caller keeps the raw label as a hint.
// A Creative Commons slug without a version is taken as 4.0.
func spdxFromSlug(slug string) string {
	s := strings.ToLower(strings.TrimSpace(slug))
	version := "4.0"
	for _, v := range ccVersions {
		if base, ok := strings.CutSuffix(s, "-"+v); ok {
			s, version = base, v
			break
		}
	}
	switch s {
	case "cc0", "cc-0", "publicdomain", "public-domain", "pd":
		return "CC0-1.0"
	case "cc-by", "by":
		return "CC-BY-" + version
	case "cc-by-sa", "by-sa":
		return "CC-BY-SA-" + version
	case "cc-by-nc", "by-nc":
		return "CC-BY-NC-" + version
	case "cc-by-nc-sa", "by-nc-sa":
		return "CC-BY-NC-SA-" + version
	case "mit":
		return "MIT"
	case "apache-2.0", "apache":
		return "Apache-2.0"
	case "ofl", "ofl-1.1":
		return "OFL-1.1"
	}
	return ""
}

// ccLicenceURL matches a Creative Commons licence path and its version, e.g.
// /licenses/by-sa/4.0/.
var ccLicenceURL = regexp.MustCompile(`/licenses/([a-z-]+)(?:/(\d+\.\d+))?`)

// spdxFromLicenceURL extracts an SPDX id from a Creative Commons licence URL,
// e.g. https://creativecommons.org/licenses/by-sa/4.0/. A URL without a
// version is taken as 4.0. NoDerivatives licences map to nothing: the licence
// table has no id for them.
func spdxFromLicenceURL(raw string) string {
	u := strings.ToLower(raw)
	if strings.Contains(u, "publicdomain/zero") || strings.Contains(u, "/cc0") {
		return "CC0-1.0"
	}
	m := ccLicenceURL.FindStringSubmatch(u)
	if m == nil {
		return ""
	}
	version := m[2]
	if version == "" {
		version = "4.0"
	}
	return spdxFromSlug("cc-" + m[1] + "-" + version)
}
