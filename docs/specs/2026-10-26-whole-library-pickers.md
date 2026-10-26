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
