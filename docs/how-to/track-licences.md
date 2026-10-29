# Answer "are we allowed to ship this?"

Damson tracks one licence per asset, with the terms broken out as six
yes/no/unknown attributes, so you can filter a library by what it permits and
look at a project and see whether anything in it blocks a commercial release.

Reach for this when you are about to ship, when someone asks where a model came
from, or when you have just imported a hundred packs and want to know which of
them nobody has ever looked at.

This guide assumes you have a library with assets in it — see
[`docs/getting-started.md`](../getting-started.md) if you do not.

## What Damson stores

An asset has at most one licence row. It holds:

- **the original text**, verbatim, always — whatever Damson matched against;
- **a detected type**: an SPDX id (`MIT`, `GPL-3.0`, `CC-BY-NC-4.0`),
  `unity-eula`, `custom`, or `unknown`;
- **a confidence** and, when the text was edited away from the canonical
  wording, a note saying so;
- **six attributes**, each `yes`, `no`, or *unknown*: commercial use,
  modification, redistribution, attribution required, share-alike,
  open-source required;
- **a review mark** — who confirmed it, and when.

Unknown is a real third state, not a `no`. Damson never guesses an attribute it
cannot derive from the detected type, and the UI shows the difference, because
"this licence says nothing about redistribution" and "this licence forbids
redistribution" are not the same answer.

## The four statuses

Every screen that summarises licences uses the same four:

| Status | Label in the UI | What makes an asset land here |
|---|---|---|
| `ok` | Compliant | A permissive type, and commercial use is not barred |
| `warn` | Review | A licence with terms a human should read |
| `err` | Violation | Commercial use is explicitly `no` |
| `unk` | Unknown | No licence, or a type of `unknown` |

"Permissive" is a fixed list in the code: `MIT`, `BSD-2-Clause`,
`BSD-3-Clause`, `Apache-2.0`, `Zlib`, `ISC`, `CC-BY-4.0`, `CC-BY-3.0`,
`CC0-1.0`, `OFL-1.1`, `Unlicense`. Everything else that has a licence at all is
**Review** — including `unity-eula` and the GPL family. That is deliberate: a
Unity Asset Store pack is usually fine to ship inside a game and never fine to
redistribute on its own, and Damson will not collapse that into a green tick.

So a freshly imported library of Asset Store packs reads as a wall of amber.
That is the correct answer, not a bug. Your job is to turn the amber into
*reviewed* amber.

## Where a licence comes from

Four writers, and they do not have equal authority:

1. **Import enrichment.** When the storefront listing says something about a
   licence, the `enriching` stage writes it. A Unity Asset Store listing yields
   `unity-eula` with its published terms: commercial use yes, modification yes,
   redistribution **no**.
2. **A rescan.** **Rescan package** on the asset panel looks through the
   current version's files for one named `LICENSE`, `LICENSE.md`,
   `LICENSE.txt`, `COPYING`, `COPYING.LESSER` or `UNLICENSE` — matched
   case-insensitively, anywhere in the tree, preferring the shallowest — then
   reads its first 128 KiB and runs the detector over that.
3. **A store lookup.** `dmsn asset refresh` and `dmsn bulk refresh` re-read the
   listing and apply what it says.
4. **A person**, from the asset panel or the bulk bar.

The first three are *automatic* writers. **An automatic writer never replaces a
licence someone has marked reviewed.** That one rule is what makes reviewing
worth doing: it pins your answer down so a later rescan cannot quietly undo it.

A person's write always replaces whatever was there — and clears the review
mark, because the thing that was reviewed is gone.

> _[Screenshot: the asset detail panel's Licence section showing a
> `unity-eula` pill, a "95% match" badge, the six-row attribute grid, and the
> **Mark reviewed** button.]_

## Set a licence by hand

Open the asset, expand **Licence**.

**If the asset has no licence yet**, you get a textarea.

1. Paste the licence text.
2. Press **Detect**. Damson matches the text and shows you what it found —
   the type, the confidence, and the attributes it would store — without saving
   anything.
3. Press **Save detected** to take the detector's answer, or **Save as custom**
   to keep your text and store it as type `custom` with no attributes set.

**If the asset already has one**, press the **Edit** button in the licence
header. The form lets you change the text and flip each attribute between
**Yes**, **No** and **Unknown**.

Two things about that form are worth knowing before you fight them:

- **Changing the text makes Save unavailable until you press Detect again.**
  The form will not let you save a type that no longer matches its text. Run
  **Detect**, then save.
- **Save is unavailable until something actually changes.** Any save replaces
  the licence row and clears its review, so Damson refuses to do that for a
  no-op.

A save never stores the type `unknown`. If the detector finds no match, the
form saves `custom` instead and keeps the attributes you set — because the
status rule ignores the attributes of an `unknown` licence, and a licence you
took the trouble to describe should count.

## Mark it reviewed

Press **Mark reviewed** in the licence header. It records you and the
timestamp, shows a **reviewed** badge, and from then on no rescan, import
enrichment or store lookup will overwrite that licence.

Reviewing does *not* change the status. A reviewed `unity-eula` is still
**Review** — the status describes the terms, the review mark describes whether
a human has read them.

> This is a different thing from the asset-level **Needs review** chip. An
> import that creates a new asset sets that flag, and it is about the whole
> asset — name, tags, metadata — not just its licence. Clear it by clicking
> the chip, or with **Mark reviewed** on the bulk bar.

## Do a whole batch at once

When a set of assets share a licence — everything from one publisher, say —
set it in one go.

1. Go to **Library**.
2. Hover a card and tick the checkbox at its top-left corner. More checkboxes
   appear as soon as one is ticked.
3. With a selection made, the bulk bar appears at the bottom. Press
   **Licence**.
4. Paste the text and press **Apply licence**.

> _[Screenshot: the Library grid with four cards selected and the bulk action
> bar showing Mark reviewed / Needs review / Tag / Collection / Licence /
> Delete.]_

The bulk action always runs the detector over the text you pasted and stores
its answer — there is no "save as custom" here. It is one transaction over up
to 1000 assets, and it is a person's write, so it replaces reviewed licences
too. Every asset gets its own audit entry.

## Find the assets that need you

The **Licence** section of the library filter sidebar has one chip per status:
**Compliant**, **Review**, **Violation**, **Unknown**. Clicking one filters the
grid; clicking it again clears it.

> _[Screenshot: the library filter sidebar with the Licence section expanded
> and **Unknown** active.]_

Start with **Unknown** — that is the set Damson knows nothing about, and it is
the only one where the answer is genuinely missing rather than merely amber.

To filter by one exact licence type rather than a status, add `spdx=` to the
library URL by hand:

```
/library?spdx=GPL-3.0
```

The page reads that parameter, but nothing in the sidebar sets it, so typing
the URL is the only way to get at it today.

From the CLI, the same two filters:

```sh
dmsn assets --licence-status unk
dmsn assets --licence GPL-3.0
```

## Answer the question for a project

A project is a set of pinned asset versions (see
[Pin a project to a version you have tested](pin-versions.md)). Its page opens
with a **Licence mix** bar: a stacked bar plus counts of compliant / review /
violation / unknown across every pin.

> _[Screenshot: a project page's Licence mix bar with its four counts
> underneath, above the Pinned assets table.]_

Each row in the **Pinned assets** table carries its asset's licence pill, so
anything red is one glance away. A non-zero **violation** count on a project
whose target distribution is `commercial` is the signal to stop and look.

Set that target with the project's **Target distribution** field —
`commercial`, `open-source`, `personal` or `unspecified`. Damson records it and
shows it; it does not enforce anything against it. The enforcement is you
reading the bar.

## Things that will catch you out

- **Status is derived, never stored.** Change an attribute and every count
  moves with it. There is no separate "approved" flag to get out of sync.
- **A rescan can only find a licence file that was extracted.** A loose upload,
  or a pack with no `LICENSE` at the root of its tree, has nothing for the
  detector to read. Paste the text instead.
- **Rescan is non-destructive by design.** If it reports nothing changed on an
  asset that plainly has a `LICENSE` file, check whether its licence is already
  marked reviewed — that is the rule doing its job.
- **Unity's EULA is matched by fingerprint, not by a full SPDX match.** Damson
  looks for "Unity Asset Store" plus "End User License Agreement" or "EULA" in
  the first 4 KiB, and stores the Asset Store's published terms. It is a
  heuristic, and you can override it.
- **`custom` means "a human described this".** It is not a failure state.
  `unknown` is the failure state.
- **Every licence write is audited** — `asset.licence.set`,
  `asset.licence.update`, `asset.licence.review` — and shows up in the asset's
  **Audit** section and the library audit log.

## Where to go next

- [Import Unity packages](import-unity-packages.md) — where most of these
  `unity-eula` rows come from.
- [Pin a project to a version you have tested](pin-versions.md) — the other
  half of the shipping question.
- [`docs/architecture.md`](../architecture.md), Licence detection — the rules
  behind all of the above.
