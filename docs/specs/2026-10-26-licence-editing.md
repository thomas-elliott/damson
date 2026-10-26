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
