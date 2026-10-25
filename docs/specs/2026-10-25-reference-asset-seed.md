# Seed a realistic reference-asset library for local dev

Covers DAM-41

## Why are we doing this

The main use case, recording what I own by path or store link and then browsing it at scale, can't
be tried or judged on a local `just dev` stack, because `just seed` loads no reference assets.

## Steering

None recorded

## Design

none — operator approved the recommendations in the DAM-41 spec round on 2026-10-25

## Decisions

### How does the seed write the reference assets?
Answer: devseed writes them straight into the dev Postgres through `internal/models`, not through
`POST /assets/references`. Every field the filters and the asset panel read is filled in, including
store, licence, release date and latest store version. No network calls are made, and the output
is the same on every run. The seeded rows get no audit entries.
By: recommended, operator said "go" (2026-10-25)

### Which recipe runs it?
Answer: `just seed` writes the reference assets after the three sample packs. Running it again
writes no duplicate seeded assets: anything already written, matched by path or link, is skipped.
By: recommended, operator said "go" (2026-10-25)

### Do the seeded assets get images, and where do their links point?
Answer: no covers or thumbnails. Store links use real store hosts (Unity Asset Store, itch.io,
Sketchfab, Fab) with made-up product ids. A store fetch from the asset panel will fail against
those ids.
By: recommended, operator said "go" (2026-10-25)

### What does the seeded library look like?
Answer:
- About 300 assets, generated from a fixed seed.
- Paths are shaped like the NFS store (`/mnt/assets/<Publisher>/<Category>/<Pack>`) and tagged by
  folder, the way `dmsn scan` tags them.
- Roughly a third each are path-only, link-only, and path plus link.
- The publishers and stores vary, and about 15% are marked needs-review.
- Some names are duplicates, and some are near-duplicates (`v1.2` beside `v1.3`, `_URP` suffixes).

By: recommended, operator said "go" (2026-10-25)

## Acceptance

- After `just dev` and `just seed`, the Library holds about 300 reference assets besides the three
  sample packs, spread over several pages.
- `just seed` completes with the network unavailable to the server and devseed, apart from
  localhost.
- Running `just seed` a second time adds no assets.
- Search by name finds seeded assets, including both members of a near-duplicate pair.
- Each of these filters narrows the seeded set to a non-empty subset smaller than the whole:
  needs review, publisher, store, licence status, released after and not downloaded.
  (Amended 2026-10-25: "updates available" struck. A reference asset holds no versions, so that
  filter can never match one; the operator chose to keep the seed reference-only.)
- The asset panel for a path-only asset, a link-only asset and a path-plus-link asset shows its
  path, its link, or both, along with its publisher, store, licence and tags.
- `AGENTS.md`, `README.md` and any other doc that says what `just seed` loads describe the new
  seed.

## Out of scope

- Live store lookups during seeding, and any change to the record endpoint.
- Covers, thumbnails or gallery images for seeded assets.
- Audit entries for seeded rows.

## Start here

- `tools/devseed/main.go`
- `internal/handlers/api/assets_reference.go`
- `internal/handlers/api/assets.go`
- `docs/architecture.md`

## Deferred follow-ups

None
