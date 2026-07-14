# storefront-metadata

## ADDED Requirements

### Requirement: Search terms are derived from a cleaned filename

Storefront searches SHALL use a cleaned form of the import filename rather than
the raw filename. This applies to both auto-enrich and the manual picker's
default query. Cleaning SHALL strip the file extension, remove a trailing version
token, and replace separator characters with spaces. Interior camelCase words
SHALL NOT be split.

#### Scenario: Version suffix and separators removed for search
- WHEN an asset imported as `Goblin_Warrior_AnimSet_v1.1.unitypackage` is searched
- THEN the search query is `Goblin Warrior AnimSet`
- AND the query is not `Goblin_Warrior_AnimSet_v1.1`

#### Scenario: Import default name is cleaned the same way
- WHEN an asset is imported from `Goblin_Warrior_AnimSet_v1.1.unitypackage`
- THEN its default name is `Goblin Warrior AnimSet`

#### Scenario: CamelCase words are preserved
- WHEN a filename contains a camelCase word such as `AnimSet`
- THEN the cleaned name keeps `AnimSet` as one word

### Requirement: Preferred store ordering

A library SHALL have a `preferred_store` setting defaulting to `unity`. Search
results SHALL be ordered so candidates from the preferred store appear first,
preserving each store's own relevance order within its group. The conservative
auto-match SHALL prefer an exact-title candidate from the preferred store when
more than one store returns an exact match.

#### Scenario: Preferred store sorts to the top
- WHEN a search returns candidates from Unity, Sketchfab, and itch
- AND the library's preferred store is `unity`
- THEN Unity candidates are listed before the others
- AND the relative order of the non-preferred stores is unchanged

#### Scenario: Auto-match prefers the preferred store
- WHEN more than one store returns a candidate whose title exactly matches the asset name
- AND one of them is from the preferred store
- THEN the preferred store's candidate is the one applied

#### Scenario: Default preferred store
- WHEN a library has no explicit preferred-store setting
- THEN the preferred store is `unity`

### Requirement: Gallery preview images are fetched, stored, and displayed

The system SHALL download, store, and display a listing's gallery images beyond
the single cover when metadata is fetched or auto-enriched with a confident
match. Stored images SHALL be deduplicated by source URL and bounded in count.
An image download failure SHALL NOT fail the fetch or the import.

#### Scenario: Unity gallery images captured
- WHEN a Unity listing with multiple screenshots is fetched
- THEN its gallery images are downloaded and stored as asset images
- AND they appear in the detail panel gallery strip

#### Scenario: Re-fetch does not duplicate images
- WHEN the same listing is fetched again
- THEN images whose source URL is already stored are not stored a second time

#### Scenario: Image fetch failure is non-fatal
- WHEN a gallery image cannot be downloaded or is not a valid image
- THEN it is skipped and logged
- AND the rest of the fetch/enrich completes normally

#### Scenario: Gallery images served inline
- WHEN the detail panel requests a stored gallery image
- THEN it is served by the thumbnail endpoint, scoped to the asset's library

### Requirement: Confident match renames the asset to the store title

A confident storefront match SHALL rename the asset to the listing title. On
auto-enrich the rename SHALL be applied unconditionally. On a manual fetch the
rename SHALL be offered as a confirmable conflict alongside the other fields.

#### Scenario: Auto-enrich renames from the store
- WHEN import auto-enrich confidently matches a listing titled `Goblin Warrior Animation Set`
- THEN the asset is renamed to `Goblin Warrior Animation Set`

#### Scenario: Manual fetch confirms the rename
- WHEN a manual fetch returns a title different from the current asset name
- THEN the name is presented as a conflict the user can confirm or decline
- AND declining leaves the current name unchanged

### Requirement: Descriptions preserve formatting as Markdown

When a store provides an HTML description, it SHALL be converted to Markdown
rather than flattened to unformatted plain text, preserving paragraph and list
structure. Conversion failure SHALL fall back to the plain-text form.

#### Scenario: Unity HTML description becomes Markdown
- WHEN a Unity listing description contains paragraphs and a bulleted list
- THEN the stored description is Markdown preserving those paragraphs and list items

#### Scenario: Conversion failure falls back
- WHEN HTML-to-Markdown conversion fails
- THEN the stored description is the plain-text (tag-stripped) form

### Requirement: Files section summarises the current version

The asset detail panel's Files section SHALL present the current version's file
count and total size together with the browse, rescan, and regenerate-previews
actions, and SHALL NOT list individual files (the scoped file browser is the
surface for that).

#### Scenario: Files section shows a summary
- WHEN an asset with many files is opened in the detail panel
- THEN the Files section shows the file count and total package size
- AND it does not render a list of individual file paths

#### Scenario: Actions remain available
- WHEN the Files section is shown
- THEN the rescan and regenerate-previews actions are available
- AND a browse action is available when the asset is browsable
