# Test the density and licence-bar wiring, and stop the grid shifting on cold load

Covers DAM-56

## Why are we doing this

A library set to small or large density draws at medium and then re-flows on a cold load. And
nothing would catch a page that stopped passing density to the grid, or a project card that stopped
drawing its licence counts.

## Steering

None recorded

## Design

none: operator approved the approach in the spec round on 2026-10-26

## Decisions

### How does the grid avoid drawing at the wrong density first?
Answer: Library and Collection detail wait for the library settings before drawing the grid. If the
settings fail to load, the grid draws at medium.
By: recommended, operator said "go" (2026-10-26)

### Which project card reads the licence counts?
Answer: the card on the Projects list, which draws `licence_counts` from the projects API.
By: settled by web/src/pages/ProjectsPage.tsx (2026-10-26)

## Acceptance

- A test fails if Library or Collection detail stops passing the library's density to the grid.
- A test fails if the Projects list card stops drawing the licence counts from the projects API.
- A cold load with density set to small or large draws the grid at that density first, with no
  visible re-flow.
- When the settings request fails, the grid still draws, at medium.

## Out of scope

None

## Start here

- web/src/pages/LibraryPage.tsx
- web/src/pages/CollectionDetailPage.tsx
- web/src/pages/ProjectsPage.tsx

## Deferred follow-ups

None

### Cleanup

- Built by the stagecraft `spec-to-pr` factory as work item `dam-56`; no `/cleanup` pass ran. `just check` passed on 48986d7 (lint, format, tidy, build, Go, Vitest, stories, e2e).
- No `docs/intent/` in the repo, so nothing graduated. Retired with these medium and low findings from the plan and code reviews:
  - MED · addressed · the no-re-flow tests must assert that every density the grid was drawn with is the library's, since the old first render passed `undefined`, not "medium" (web/src/pages/LibraryPage.test.tsx, web/src/pages/CollectionDetailPage.test.tsx)
  - LOW · addressed · the Collection detail cold-load test must let the collection and assets load before `/api/settings` answers, or it passes against the old code (web/src/pages/CollectionDetailPage.test.tsx)
  - LOW · addressed · the Projects card test uses four distinct non-zero licence counts per project, so a swapped or dropped field fails (web/src/pages/ProjectsPage.test.tsx)
  - LOW · addressed by fac-4 · the settings query key is not scoped to a library, so after a library switch the grid draws once at the previous library's density and re-flows; not a cold load, so outside this spec (web/src/lib/queryKeys.ts)
  - LOW · addressed by fac-4 · when `/api/settings` fails, the grid waits behind "Loading…" through the app-wide retry (about a second) before drawing at medium; `retry: false` on the settings query would remove the wait (web/src/hooks/useDataQueries.ts)
  - LOW · addressed by fac-4 · while the settings load, a failed asset list or a failed or missing collection shows "Loading…" instead of its error or not-found state; gating only the grid branch on the settings would fix it (web/src/pages/LibraryPage.tsx, web/src/pages/CollectionDetailPage.tsx)
- Follow-up `fac-4` (branch `fac-4-board/grid-density-review-fixes`) fixed the three open findings above. Its reviews left these low findings:
  - LOW · addressed · `/api/settings` answers for the current library, so refetching the settings during a switch stored the new library's settings under the old library's key; the switch now marks them stale without refetching (web/src/hooks/useMutations.ts)
  - LOW · addressed · with retry off, the Settings page shows "Settings unavailable" after one failure; the hook's comment says so (web/src/hooks/useDataQueries.ts)
  - LOW · addressed · the switch test checks every render with `/api/me` answering late, so it does not pass on timing alone (web/src/hooks/useDataQueries.test.tsx)
  - LOW · open · Collection detail now needs `/api/me` for the library id; if `/api/me` fails with a non-auth error, a collection with assets stays on "Loading…" (web/src/pages/CollectionDetailPage.tsx)
  - LOW · open · `refetchType: "none"` stops only the switch's own refetch; anything that mounts or refetches the old library's settings before `/api/me` lands can still store the new library's settings under the old key (web/src/hooks/useMutations.ts)
