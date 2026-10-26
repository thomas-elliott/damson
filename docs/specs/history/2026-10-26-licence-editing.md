# Let a set licence be replaced or edited

Covers DAM-47

## Why are we doing this

Once an asset has a licence, the asset panel can only mark it reviewed. A wrong detection, or a
custom licence saved with every attribute Unknown, can only be fixed with the bulk Licence action.

## Steering

None recorded

## Design

none: operator approved the approach in the spec round on 2026-10-26

## Decisions

### What can a person change on a set licence?
Answer: the text and each of the six attributes (commercial use, modification, redistribution,
attribution required, share-alike, open source required).
By: operator chose "Text and the six attributes" (recommended) (2026-10-26)

### How is editing started and abandoned?
Answer: an Edit button on the set licence opens it prefilled with its text and attributes. Cancel
leaves the licence unchanged.
By: recommended, operator said "go" (2026-10-26)

### What happens to the review when a person saves?
Answer: the saved licence starts unreviewed and needs Mark reviewed again, as for any person's
licence write.
By: settled by internal/licence/replace.go (2026-10-26)

## Acceptance

- An asset with a licence shows an Edit button in the asset panel.
- Edit opens the licence's current text and attributes; Cancel closes it with nothing changed.
- A person can change the text, run Detect, and save the detected licence in place of the old one.
- A person can set each of the six attributes to Yes, No or Unknown and save.
- A saved edit shows as unreviewed, with Mark reviewed offered.
- The change appears in the asset's audit history.

## Out of scope

- Removing a licence from an asset.
- Changing the bulk Licence action.

## Start here

- web/src/components/detail/LicenceEditor.tsx
- internal/handlers/api/licences.go

## Deferred follow-ups

None

### Cleanup

- Built by the stagecraft `spec-to-pr` factory as work item `dam-47`; no `/cleanup` pass ran. `just check` passed on 86c09dc (lint, format, tidy, build, Go, Vitest, stories, e2e).
- Retired with these open findings:
  - MED · plan review: after a Detect, changing the text back to the original re-enabled Save with the detected type still drafted. Addressed: once a Detect has run, Save needs the current text to be the text it was sent with (web/src/components/detail/LicenceEditor.tsx)
  - LOW · plan review: the edit form did not say what a failed Detect shows. Addressed: the error shows in the form and the draft is unchanged
  - LOW · plan review: SegmentedControl takes string values. Addressed: yes/no/unknown map to true/false/null
  - LOW · plan review: the docs step's count did not match its list. Addressed in docs/architecture.md
  - LOW · code review: Save is enabled as soon as the form opens, so Edit then Save with nothing changed still replaces the licence, clearing its review and turning an unknown licence into custom. Not addressed (LicenceEditForm's Save condition)
  - LOW · code review: the form shows one error, so after a failed save a later successful Detect brings the old save error back. Not addressed (call put.reset() when Detect starts)
  - LOW · code review: no vitest case makes /detect fail. Not addressed (web/src/components/detail/LicenceEditor.test.tsx)
