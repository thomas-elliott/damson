package licence

import "slices"

// StatusFor maps a detected type and its attributes to the status the UI
// shows: ok, warn, err or unk. StatusSQL is the same rule for queries; change
// both together. TestLicenceStatus_AgreesAcrossGridProjectFilterAndDetail
// checks that they agree.
func StatusFor(detectedType string, attrs Attributes) string {
	if detectedType == "" || detectedType == "unknown" {
		return "unk"
	}
	// Commercial-use blockers flag as `err`; anything with all affirmative
	// common-use attributes is `ok`; everything else is `warn`.
	if attrs.CommercialUse != nil && !*attrs.CommercialUse {
		return "err"
	}
	if slices.Contains(PermissiveTypes, detectedType) {
		return "ok"
	}
	return "warn"
}

// PermissiveTypes are the types StatusFor reports as ok unless their
// attributes bar commercial use.
var PermissiveTypes = []string{
	"MIT", "BSD-2-Clause", "BSD-3-Clause", "Apache-2.0", "Zlib", "ISC",
	"CC-BY-4.0", "CC-BY-3.0", "CC0-1.0", "OFL-1.1", "Unlicense",
}

// StatusSQL is StatusFor as SQL over a licences row aliased alias (all NULL
// when the asset has no licence). Its one placeholder takes PermissiveTypes.
func StatusSQL(alias string) string {
	return `CASE
		WHEN COALESCE(` + alias + `.detected_type, '') IN ('', 'unknown') THEN 'unk'
		WHEN ` + alias + `.attributes_json->>'commercial_use' = 'false' THEN 'err'
		WHEN ` + alias + `.detected_type IN ? THEN 'ok'
		ELSE 'warn' END`
}
