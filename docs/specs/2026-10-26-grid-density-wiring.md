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
