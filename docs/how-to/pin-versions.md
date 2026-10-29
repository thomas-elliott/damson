# Pin a project to a version you have tested

The pack you integrated six months ago is not the pack the store sells today.
Damson keeps every version you have imported, and lets a project say which one
it is actually using — so "upgrade the terrain pack" becomes a decision you
make rather than a thing that happens to you.

Reach for this when more than one build depends on the same library, when you
are about to take an update and want a way back, or when you need to answer
"which version of this is in the shipped game".

## How versions work

An asset owns versions; a version owns files. Nothing is ever replaced — a new
import of the same pack adds a version alongside the old one, and both keep
their own file manifests.

A version carries two separate fields, and they do different jobs:

- **Store version** — a dotted number, `2.1`, `1.10`. This is the one Damson
  *orders by*. It must parse as dotted integers (a leading `v` is stripped) or
  it is not stored.
- **Version label** — free text, for whatever you want to call it: `shipped`,
  `pre-HDRP`, `trimmed`. Damson shows it and never reasons about it.

The **current version** is the one the asset's Files section and its grid tile
describe, and the one a pin defaults to. Damson picks it, in this order:

1. the highest parseable store version;
2. failing that, the newest import date;
3. failing that, the highest row id.

A version with a store version always outranks one without, whatever the
dates. A version whose files have not finished landing is passed over, so a
half-finished import never becomes current.

> _[Screenshot: the asset panel's Versions section with three versions, the
> current one marked, each showing its file count, size and import date.]_

## Add a version

**From the CLI**, name the asset you are adding to:

```sh
dmsn upload ForestPack.unitypackage --asset-id 42
```

**From `dmsn sync push`**, automatically: a cache package whose path Damson
already holds at a different size comes in as a new version of that asset. See
[Import Unity packages](import-unity-packages.md).

**From the web UI**, there is an extra step. An upload always creates a *new
asset*; Damson then checks the name against the library and, when it matches an
existing asset exactly (lower-cased, with punctuation collapsed to spaces),
raises a notification:

> Possible new version of Forest Pack — Add `Forest_Pack_v2.2.unitypackage` as
> a version of Forest Pack instead of a separate asset?

Press **Add as version** and Damson re-sends the file attached to that asset,
then deletes the standalone one it had just created. Ignore the notification
and you keep two assets.

> _[Screenshot: the notification centre showing the "Possible new version of…"
> card with its **Add as version** action.]_

**An asset cannot hold two versions with the same store version.** A second
upload at `2.1` is refused with a 409 telling you to delete or replace the
existing one. Two versions with *no* store version are fine.

## Fix a version's numbers

Expand **Versions** on the asset panel and press the pencil on a row. You get
two small fields — store version, then label — plus a tick to save and a cross
to cancel. Enter saves; Escape cancels.

The store version must be a dotted number; `2.1-beta` is rejected with
"store_version must be a dotted number like 1.1". Put that in the label
instead.

**Editing a store version does not re-pick the current version.** The
current-version choice runs on import, on version delete, on a merge and on a
subset — not on an edit. If you correct a store version and the wrong version
is still marked current, delete and re-import, or nudge it with one of those
other operations. This is worth knowing before you spend an afternoon
wondering why the asset will not update.

## Know when a pack has been updated upstream

Damson compares the **latest store version** it last read from the storefront
against the highest store version you hold. When the store is ahead, the asset
is flagged as having an update available.

The comparison is numeric, not textual, so `1.10` is correctly newer than
`1.9`. Anything that will not parse on either side is ignored rather than
guessed at, so:

- an asset holding no orderable store version is **never** reported as out of
  date — you have not told Damson what you have;
- a deprecated listing is excluded — the store will not be publishing anything
  newer.

Both halves mean a false *negative* is possible and a false positive is not.

To refresh what Damson knows, run a store lookup. There is no control for this
in the web UI today — it is the CLI or the API:

```sh
dmsn asset refresh 42                      # one asset, waits for the result
dmsn bulk refresh --assets 1,2,3           # several
dmsn bulk refresh --store unity            # everything matching a filter
```

A refresh fills blanks and replaces what the *last lookup* wrote, but never a
field a person has edited by hand — it reports those as kept instead. Lookups
run one at a time on the server; stopping `dmsn` stops the waiting, not the
work. `dmsn bulk refresh --batch <id>` picks the report back up later.

You can also type the number in yourself: the asset panel's **Edit details**
form has a **Store version** field, which writes the same
`latest_store_version`.

Then find them:

- **Library → filter sidebar → Updates → Updates available**, or
- `dmsn assets --updates-available`.

> _[Screenshot: the library filter sidebar with the Updates section expanded
> and "Updates available" ticked, and the grid behind it showing flagged
> assets.]_

## Set up a project and pin to it

A **project** is a named set of pinned asset versions inside one library — a
game, a build, a demo. Go to **Projects → New project**. Give it a name and,
optionally, a **target distribution**: `commercial`, `open-source`, `personal`
or `unspecified`. Damson records and displays it; it does not enforce anything
against it.

To pin:

1. Open the project.
2. Press **Pin asset**.
3. Search for the asset, pick a **version** from the dropdown, and confirm.

> _[Screenshot: the Pin an asset dialog with an asset chosen and the version
> dropdown open.]_

Omitting the version through the API pins the asset's current version:

```sh
curl -sS -X POST localhost:8080/api/projects/1/pins \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"asset_id":42}'
```

There is **one pin per (project, asset)** — a project cannot hold two versions
of the same pack. Pinning an asset that is already pinned moves the pin, in
either direction, and is audited as a change rather than a new pin.

Pinning is a web-UI or API operation; `dmsn` has no project command.

## Read the project page

The **Pinned assets** table has a row per pin:

| Column | What it tells you |
|---|---|
| Asset | Links through to the asset |
| Pinned | The store version or label this project is on |
| Latest | The asset's current version — amber with an ↑ when they differ |
| Licence | The pill, so a problem is visible without leaving the page |
| Size | The pinned version's bytes |
| Status | **Up to date**, or an **Upgrade to 2.2** button |

**Upgrade to 2.2** moves the pin to the asset's current version in one click.
Nothing is downloaded and nothing is deleted — the pin is a pointer.

> _[Screenshot: a project's Pinned assets table with one row showing an
> "Upgrade to 2.2" action and the rest up to date.]_

Note that "Latest" here means *the newest version in your library*, not the
newest version on the store. The two questions are separate: the project page
answers "am I on the newest thing we have imported", and the Updates available
filter answers "is there something newer we have not imported". A pack can be
up to date on one and behind on the other.

The header carries the same counts in aggregate — `N assets pinned`, and
`N updates available` in amber when any pin is behind.

## Unpin, and get out of a corner

**Unpin** on a row removes the pin only; the asset stays in the library, and
every other project keeps its own pin.

**A pinned version cannot be deleted.** Damson refuses with "version is pinned
by N project(s); unpin before deleting". That is the safety net: an asset's
history cannot be destroyed out from under a project that depends on it. Move
or remove the pin first.

Deleting the whole *asset*, on the other hand, takes its pins with it, through
the database's cascade. Deleting the *project* removes its pins and leaves
every asset untouched.

## Reference assets

A **reference asset** records a pack you have not imported — a path on a NAS, a
store link, or both — so the catalogue is complete even when the bytes are not
in Damson. It has no versions.

Pin one and the pin carries no version; the project page shows `Reference` in
the Pinned column and an em dash in Size, because there are no bytes to
measure. Pinning the same reference twice is a no-op, and passing a
`version_id` for one is a 400.

## Things that will catch you out

- **Store version and label are not interchangeable.** Only the store version
  orders versions and drives the updates-available comparison. A label of
  `2.1` buys you nothing.
- **A subset version has no store version**, so it only becomes current when
  no other version of that asset has one either. See
  [Import Unity packages](import-unity-packages.md).
- **An asset with no store version anywhere is invisible to the
  updates-available filter.** If you want upgrade tracking, the versions need
  numbers.
- **Cache imports arrive unversioned on purpose.** `dmsn sync` suppresses
  filename version parsing, because the Unity cache has no versions in its
  filenames and a phantom one would block the real one permanently.
- **Every pin change is audited** — `project.pin`, `project.pin.change`,
  `project.unpin` — and shows in the project's **Recent activity** panel and
  the library audit log.

## Where to go next

- [Answer "are we allowed to ship this?"](track-licences.md) — the Licence mix
  bar on the same project page.
- [Group assets into collections](collections.md) — when you want a grouping
  that is not about versions at all.
- [`docs/architecture.md`](../architecture.md), Data model — versions, pins and
  the rules around them.
