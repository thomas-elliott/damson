# asset-versioning

## ADDED Requirements

### Requirement: Store version parsed from the filename

An asset version SHALL carry a `store_version` distinct from its internal
sequence label (`v1`, `v2`…). On import, `store_version` SHALL be parsed from a
trailing version token in the upload filename. A token SHALL be recognised only
when it is `v`-prefixed or contains a dotted number, so a bare trailing integer
is not mistaken for a version.

#### Scenario: Dotted version parsed
- WHEN an asset is imported from `Goblin_Warrior_AnimSet_v1.1.unitypackage`
- THEN the created version's internal label is `v1`
- AND its store version is `1.1`

#### Scenario: No version token
- WHEN an asset is imported from a filename with no trailing version token
- THEN the version's store version is empty
- AND its internal label is assigned by sequence as before

#### Scenario: Trailing integer is not a version
- WHEN an asset is imported from `Building_7.unitypackage`
- THEN `7` is treated as part of the name, not a store version

### Requirement: Store version is editable

The store version of a version SHALL be editable after import via a version
update endpoint, validated to a dotted-number (optionally `v`-prefixed) shape,
with the change recorded in the audit log. Clearing it SHALL be allowed.

#### Scenario: Edit the store version
- WHEN a user sets a version's store version to `2.0`
- THEN the version's store version is `2.0`
- AND an audit entry records the change

#### Scenario: Invalid store version rejected
- WHEN a user submits a store version that is not a dotted number
- THEN the request is rejected and the stored value is unchanged

#### Scenario: Clear the store version
- WHEN a user submits an empty store version
- THEN the version's store version is cleared

### Requirement: Versions order by semantic version

When versions carry parseable store versions, they SHALL be ordered
newest-highest by comparing dot-separated numeric components (so `1.10` is newer
than `1.9`). Versions without a parseable store version SHALL order by import
date, after those that have one. Which version is "current" SHALL NOT change as a
result of this ordering.

#### Scenario: Semantic ordering of store versions
- WHEN an asset has store versions `1.1`, `1.10`, and `1.9`
- THEN they are ordered `1.10`, `1.9`, `1.1`

#### Scenario: Unversioned falls back to import date
- WHEN some versions have no store version
- THEN they are ordered among themselves by import date, after the versioned ones

#### Scenario: Current version unchanged
- WHEN versions are re-ordered by store version
- THEN the version marked current is still the latest-imported version

### Requirement: Store version is shown in the detail panel

The asset detail panel SHALL display each version's store version alongside its
internal label.

#### Scenario: Store version displayed
- WHEN a version has store version `1.1`
- THEN the detail panel shows `1.1` for that version in addition to its internal label
