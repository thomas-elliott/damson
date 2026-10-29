<!--
Title: imperative, sentence case, no feat:/fix: prefix, about the
behaviour rather than the files. For example: "Draw the grid at the library's
density on a cold load".

CONTRIBUTING.md has the long version.
-->

## What and why

<!-- What changes, and what problem it solves. The diff says what; this says why. -->

## How to check it

<!-- The path a reviewer should walk, or the test that now fails without the fix. -->

## Notes for the reviewer

<!-- Optional: what you ruled out, what you are unsure about, anything deliberately left for later. -->

---

- [ ] `just check` passes locally
- [ ] Tests cover the change, or there is a reason here why they don't
- [ ] Docs updated in this PR if the change made one untrue
- [ ] A schema change lands in both a new `database/migrations/NNN_*.sql` and the GORM struct, or there is no schema change
- [ ] Scoped to its stated purpose, with no unrelated refactoring
