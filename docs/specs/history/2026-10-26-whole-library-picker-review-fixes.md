# Close the review findings on the whole-library pickers

Covers fac-1. Follows DAM-45 (PR #18).

## Why are we doing this

An independent review of PR #18 on 2026-10-26 left one medium and three low findings. There was no
separate spec: the operator approved planning from the ticket, so the findings below were the
brief.

1. MEDIUM: some assets can never be reached in the picker. It showed the first 25 matches, newest
   first, with no ranking, so an older asset named "Rock" behind more than 25 newer matches for
   "rock" could not be found, pinned or merged.
2. LOW: the pin dialog could pin an asset no longer on screen after a new search.
3. LOW: "Pin version" stayed disabled with no reason when the asset's details failed to load or it
   had no versions.
4. LOW: re-pinning an already-pinned asset silently moved its pin, and a downgrade was audited as
   project.pin.upgrade.

## Decisions

### How does the picker rank matches?

GET /api/assets takes `sort=relevance`: an exact name match first, then names starting with the
search, then the rest by `ts_rank`, then newest. The picker asks for it; the library grid does not
offer it.

### How is a re-pin audited?

Neutrally, as `project.pin.change`, since the same path writes upgrades and downgrades. Migration
031 renames the existing `project.pin.upgrade` rows.

## Review findings

### Plan review

| Severity | Finding | Addressed |
| --- | --- | --- |
| medium | The relevance test could pass on `ts_rank` alone | yes: the newer competitors are name-prefix matches, and the prefix tier is tested against a newer non-prefix name match |
| medium | "Change pinned version" could be a no-op | yes: the pinned version is preselected and the button is disabled while unchanged |
| low | The pinned note could read "Pinned · " | yes: falls back to "unversioned"; a reference reads "Pinned" |
| low | The pinned reference case was untested | yes: unit test for "Already pinned" |
| low | The picked name was planned only beside the Version field | yes: shown for every pick |
| low | Migration 031 has no automated check | yes: checked by hand (renames old rows, a second run changes nothing) |

### Code review

| Severity | Finding | Addressed |
| --- | --- | --- |
| low | The planned hand check in the running app was not recorded for the four findings | no: the unit and e2e tests cover them, and the e2e drives the real app through the buried-name pin and the version change |
| low | A saved audit URL filtered on `project.pin.upgrade` now matches nothing | no: accepted; noted in the PR |
