# Catalogue the NFS library by agent scan

Covers DAM-1

## Steering

- I think it is worth looking at the workflow on how the metadata calls will be made. Thousands of assets means thousands of calls, and it may be better to spread that out or do that out-of-band, make a file with all the metadata/possible matches, rather than have it in the loop with the agent where it could be rate limited. — Session, 2026-10-05

## Design

Board Task DAM-1: its `## Direction` and `## Design documents` sections (the scan run: walk → known paths → plan → approve → record → report gone). It builds on reference assets (`docs/specs/history/2026-10-03-reference-assets.md`, PR #1).

## Decisions

### How do store searches run across thousands of packs?
Answer: As their own step, separate from the agent's work on the plan. dmsn searches the stores for every new pack, spaced out so that no store throttles the run, and writes each pack's possible matches to a file. The agent reads that file to choose links. It does not run a search per pack in conversation.
By: operator — "it may be better to spread that out or do that out-of-band, make a file with all the metadata/possible matches, rather than have it in the loop with the agent where it could be rate limited" (2026-10-05)

### Which stores are searched?
Answer: The ones the server already searches: Unity Asset Store, Sketchfab and itch.io, offered through dmsn. Damson itself still never searches when a path is recorded without a link.
By: recommended, operator said "go" (2026-10-05)

### What counts as a confident store match?
Answer: The agent's judgement for each pack, from the title, the publisher and other clues in the path. A pack with no confident match is recorded by path only and is marked "no match" in the plan. Every chosen link is shown in the plan before approval.
By: operator — chose "Agent judgement per pack from title and publisher and path clues" (2026-10-05)

### How do folders become tags?
Answer: Each folder between the scan root and the pack becomes its own tag. `/mnt/nfs/3D/Environments/Synty/POLYGON_Fantasy_Kingdom.unitypackage`, scanned from `/mnt/nfs`, gets the tags `3D` and `Environments`. The scan root and the publisher folder are not tags.
By: operator — chose "One tag per folder: 3D and Environments" (2026-10-05)

### How does the agent know what is already recorded?
Answer: dmsn can list every path already recorded in the library, so the agent can sort the plan into new, known and gone.
By: recommended, operator said "go" (2026-10-05)

### How many calls does recording one pack take?
Answer: One dmsn call records a pack with its name, tags, publisher, store link and Needs review.
By: recommended, operator said "go" (2026-10-05)

### How is the plan shown?
Answer: The agent writes the full plan to a file the operator can open, with one row per pack: path, name, publisher, tags, and the store link or "no match". In chat it shows the counts (new, known, gone) and anything it is unsure about.
By: recommended, operator said "go" (2026-10-05)

### Can the plan be changed before approval?
Answer: Yes. In conversation the operator can drop packs, fix names, tags or links, or say whether a folder is one product. The agent then shows the changed plan again. Nothing is recorded until the operator approves explicitly.
By: recommended, operator said "go" (2026-10-05)

### Who decides which folders are one product?
Answer: The agent. The plan shows each decision so the operator can correct it. The agent never looks inside a pack folder for more items.
By: recommended, operator said "go" (2026-10-05)

### Which gone paths are reported?
Answer: Only recorded paths under the folder that was scanned. A scan of `/mnt/nfs/3D` does not report paths under `/mnt/nfs/Audio` as gone.
By: recommended, operator said "go" (2026-10-05)

### What happens to a pack that moved folder?
Answer: It shows as one new pack and one gone path. Moves are not detected.
By: recommended, operator said "go" (2026-10-05)

### Does a rerun change packs that are already recorded?
Answer: No. Their names, tags and links are left as they are.
By: recommended, operator said "go" (2026-10-05)

### What if a store search fails or is throttled?
Answer: The pack is still recorded by path only, and the end-of-run report lists it.
By: recommended, operator said "go" (2026-10-05)

### How are the scan instructions installed?
Answer: `dmsn setup` installs them alongside the existing damson skill, so asking the agent to "scan my asset library" is enough to start a scan.
By: recommended, operator said "go" (2026-10-05)

### What does a recorded pack carry?
Answer: A name, its folder tags, the publisher where the path shows one, and a store link where a confident match was found. It is marked Needs review. A store link fills the store metadata and cover as it does today.
By: settled by DAM-1 Direction and `docs/specs/history/2026-10-03-reference-assets.md` ("Which fields does the store link fill?") (2026-10-05)

### What is one item?
Answer: One pack: a `.unitypackage`, an archive, or a folder that is one product. Never one asset per file.
By: settled by DAM-1 Direction (2026-10-05)

### Where does the scan run?
Answer: Where the agent can see the NFS store. The Damson server never mounts it.
By: settled by DAM-1 Direction (2026-10-05)

## Acceptance

- After one scan, the items on the NFS store are in Damson. Running the scan again adds no duplicates.
- I see the planned list before anything is recorded, and nothing is written until I approve.
- The plan is a file with one row per pack (path, name, publisher, tags, link or "no match"), and chat shows the counts.
- I can drop a pack or fix its name, tags or link in conversation, and the changed plan is what gets recorded.
- A Unity package with an Asset Store listing arrives with its store metadata and cover.
- Store searches for a whole scan run as one paced step that writes the possible matches to a file, not as one search per pack in the agent's conversation.
- A pack under `3D/Environments/<publisher>/` arrives tagged `3D` and `Environments`, with that publisher set.
- A rerun after adding two packs records exactly those two and names any recorded path that no longer exists. It deletes nothing.
- A rerun leaves names, tags and links on packs that were already recorded unchanged.
- Everything from a scan is in the review queue until I mark it reviewed.
- After `dmsn setup`, asking the agent to "scan my asset library" starts the scan.

## Out of scope

- An agent built into Damson.
- Importing the data of any pack.
- Purchases, and the Purchased-not-downloaded filter (DAM-3).
- Detecting a moved pack as the same asset.
- Updating or deleting recorded assets from a scan.
- Store search by the Damson server for a path recorded without a link.
- The server reading, checking or mounting the NFS store.

## Start here

- `docs/specs/history/2026-10-03-reference-assets.md`
- `cmd/dmsn/mutate.go`
- `cmd/dmsn/setup.go`
- `internal/services/metadata/search.go`
- `internal/handlers/api/assets_reference.go`

## Deferred follow-ups

None

## Implementation report

### Stages

1. **Record with publisher and tags.** `POST /api/assets/references` takes a `publisher` and a list of `tags`, applied only when the record creates the asset. `dmsn asset record` gains `--publisher` and `--tags`, so one call records a pack with its name, tags, publisher, store link and Needs review.
2. **Known paths.** `GET /api/assets/locations?under=PATH` lists every recorded path in the library, optionally only those under a folder. `dmsn scan known --under PATH` shows them with whether each still exists on disk, so the gone paths are the ones that don't.
3. **Paced store search.** The search response names any store that failed. `dmsn scan match <plan>` searches the stores for each pack in a plan file, one pack at a time with a pause between, and writes every pack's possible matches (or the failure) to a matches file.
4. **Record the plan.** `dmsn scan record <plan>` records every row of the approved plan file as a Needs-review reference asset, skips any path already recorded, and reports each row's outcome.
5. **Scan instructions.** `dmsn setup` installs a `damson-scan` skill alongside the damson skill: walk, known paths, plan file, paced match, choose links, approve, record, report gone and failed searches.

### Choices made

- `internal/handlers/api/assets_reference.go`: publisher and tags apply only when a record creates the asset, like the name. A given publisher is set before the store lookup, so it wins over the store's publisher, which fills blanks only.
- `internal/handlers/api/assets_reference.go`: tags go through the existing tag rule, so `3D` and `Environments` are stored as `3d` and `environments`. A record takes at most 32 tags, and a blank tag is refused before the store lookup runs.
- `internal/handlers/api/assets_reference.go`: `GET /api/assets/locations` is unpaged, one short row per recorded path. `?under=` matches the folder itself and anything below it by a prefix compare, so `_` in a pack name is not a wildcard and `/mnt/nfs/3Dx` is not under `/mnt/nfs/3D`.
- `cmd/dmsn/scan.go`: `scan known` checks each recorded path on the machine it runs on. It refuses a `--under` that isn't a readable directory, and treats any stat error other than "does not exist" as an error, so an unmounted store is never reported as every pack gone.
- `internal/services/metadata/search.go`: `SearchStores` is the store search plus the names of the stores that failed. `Search` keeps its old shape for the import enricher. `/api/metadata/search` returns them as `failed_stores`. It never returned 502 in practice, because `Search` never returned an error.
- `cmd/dmsn/scan.go`: the plan is a tab-separated file with the header `path name publisher tags link`. `tags` is comma-separated and `link` is a store URL, `no match`, or blank. Every row needs a path and a name, and a repeated path is refused. dmsn reads this same file to search and to record, so the file the operator approves is the file that gets recorded.
- `cmd/dmsn/scan.go`: `scan match` searches by the row's name, one pack at a time, 2 s apart by default (`--interval`). It skips rows whose link is already decided, keeps at most 3 listings per store per pack, and writes the file pack by pack, so a stopped run keeps what it found. A failing store is written as `store_failed`, and a failed request to Damson as `search_failed`. An auth or forbidden error stops the run.
