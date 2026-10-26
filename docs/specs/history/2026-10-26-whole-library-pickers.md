# Pickers that see the whole library

Covers DAM-45

## Why are we doing this

Pinning, merging and browsing a collection only reach the first 50 assets, so in a real library
most assets cannot be pinned, merged or seen in their collection.

## Steering

None recorded

## Design

none: operator approved the approach in the spec round on 2026-10-26

## Decisions

### How do the pin and merge dialogs find an asset?
Answer: one shared picker, used by both, that searches the server as the user types. It shows the
first page of results and, when there are more, a note to refine the search.
By: recommended, operator said "go" (2026-10-26)

### How does Collection detail show more than 50 assets?
Answer: it pages its grid with the same pager as Library.
By: recommended, operator said "go" (2026-10-26)

### Which of DAM-46's pin items does this take?
Answer: all of them: search in the pin dialog, a version choice, asset names linked in the pin
table, and a confirm on unpin. Editing a project's name, description and distribution stays in
DAM-46.
By: recommended, operator said "go" (2026-10-26)

### How is a version chosen when pinning?
Answer: after picking an asset that has versions, the pin dialog offers its versions with the
current one preselected. A reference asset, which has no versions, pins as the asset.
By: recommended, operator said "go" (2026-10-26)

## Acceptance

- In a library of several hundred assets (`just seed`), the pin dialog and the merge dialog each
  find any asset by typing part of its name.
- When a search matches more than one page of assets, the picker says so and asks for a narrower
  search.
- The pin dialog offers the picked asset's versions with the current one preselected, and the pin
  table shows the version chosen.
- Asset names in a project's pin table open that asset.
- Unpin asks for confirmation before removing the pin.
- A collection with more than one page of assets shows them across pages with the Library pager.

## Out of scope

- Editing a project's name, description or distribution (DAM-46).

## Start here

- web/src/pages/ProjectDetailPage.tsx
- web/src/components/detail/MergeDialog.tsx
- web/src/pages/CollectionDetailPage.tsx
- web/src/components/UrlPager.tsx

## Deferred follow-ups

None

### Cleanup

- Built by the stagecraft `spec-to-pr` factory as work item `dam-45`; no `/cleanup` pass ran. `just check` passed on the reviewed commit (lint, format, tidy, build, Go, Vitest, stories, e2e).
- No `docs/intent/` in the repo, so nothing graduated. Retired with the reviews' medium and low findings:
  - MED · plan review: a version chosen for one asset could be posted with another, and pin errors were not shown. Addressed: the choice resets when another asset is picked, `pin.error` shows in the dialog, and a test switches assets (web/src/pages/ProjectDetailPage.tsx)
  - MED · plan review: a listbox of buttons with `aria-selected` is invalid ARIA. Addressed: plain buttons with `aria-pressed`, no listbox (web/src/components/AssetPicker.tsx)
  - LOW · plan review: `useDeferredValue` does not debounce, and a searching state keyed on `loading` never shows with placeholder data. Addressed: a 250 ms timeout, and searching shows on `loading || placeholder` (web/src/components/AssetPicker.tsx)
  - LOW · plan review: the refine note must compare the total with the server's page, not the rows left after excluding the merge target. Addressed (web/src/components/AssetPicker.tsx)
  - LOW · plan review: seeding past a page through `uploadImport` is slow. Addressed: `createReferenceAsset` in web/e2e/fixtures.ts, and collection paging uses 26 assets at `page_size=25`
  - LOW · plan review: one `useConfirm` per pin row. Addressed: the page's `confirm` is passed in (web/src/pages/ProjectDetailPage.tsx)
  - LOW · code review: for a versioned asset whose detail fails to load or has no versions, "Pin version" stays disabled with no reason given. Not addressed (web/src/pages/ProjectDetailPage.tsx)
  - LOW · code review: the refine note prints the server page's size, so with the merge target excluded it says "first 25" over 24 rows. Not addressed (web/src/components/AssetPicker.tsx)
  - LOW · code review: search matches the start of a word in the name ("zepp" finds "zeppelin hangar", "pelin" does not), because the API builds a prefix tsquery. Not addressed; no code change asked (internal/handlers/api/assets.go)
  - Observed, not changed: a version with an empty label shows as a blank "Pinned" cell in the pin table; the pin API returns `COALESCE(version_label, '')` and the row has no store version to fall back on.
