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

### Follow-up: fac-2

- The code-review findings above, with two more from the same review, were fixed on branch
  `fac-2-board/licence-review-fixes` by the stagecraft `spec-to-pr` factory as work item
  `fac-2-board`, planned from ticket fac-2. `just check` passed on 5b97481.
  - LOW · Save with nothing changed replaced the licence. Addressed: Save waits for a change to the
    text, type or an attribute (web/src/components/detail/LicenceEditor.tsx)
  - LOW · the bulk Licence action changed behaviour. Addressed: a named type is stored with no
    detector confidence or diff again; only the panel's PUT carries them over
    (internal/handlers/api/licences.go, TestBulkAssets_SetLicence)
  - LOW · a failed save's error came back after a successful Detect. Addressed: Detect resets the
    save when it starts
  - LOW · Detect carried hidden custom terms over to a different licence. Addressed: custom terms
    survive Detect only when it finds the drafted type
  - LOW · no test made /detect fail. Addressed (web/src/components/detail/LicenceEditor.test.tsx)
- Open findings from fac-2's own reviews:
  - MED · plan review: resetting the save when Detect starts would detach an in-flight save.
    Addressed: Detect is disabled while a save is pending
  - LOW · plan review: the test for a cleared save error checked only after Detect finished.
    Addressed: it checks while Detect is still running
  - LOW · code review: no test covers Detect being disabled while a save is pending. Not addressed
    (web/src/components/detail/LicenceEditor.test.tsx)
