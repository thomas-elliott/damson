# Group assets you keep coming back to

A **collection** is a named handful of assets inside a library — bookmarks,
basically. "Boss fight audio". "Things to evaluate". "The kit we actually use
for prototypes." An asset can be in as many as you like, and being in one
changes nothing else about it.

Reach for this when a grouping is a *judgement* rather than a fact. Tags
describe what an asset is; a collection records what you decided about it.

## Collections, tags and projects

Damson has three groupings and they are not interchangeable:

| | What it is | Shape |
|---|---|---|
| **Tag** | What the asset *is* — `environment/trees/deciduous` | Hierarchical path; searchable; filters the grid, including descendants |
| **Collection** | What you *decided* about it | Flat, unordered, many-to-many, no semantics |
| **Project** | Which *version* a build is using | One pinned version per asset; tracks updates and licence risk |

If the grouping should be searchable and should nest, use a tag. If it names a
set of versions a build depends on, use a project (see
[Pin a project to a version you have tested](pin-versions.md)). If you just
want a shelf to put things on, use a collection.

## Make one

**Collections** in the left rail, then **New collection**. Name is required;
description optional. You land on the new collection straight away.

> _[Screenshot: the Collections page with several collection cards showing
> their asset counts, and the New collection button in the toolbar.]_

Names are unique per library, case-insensitively — a second "Boss fight audio"
is refused with a 409.

From the CLI:

```sh
dmsn collection new "Boss fight audio" --description "Shortlist for the Act 3 fight"
dmsn collections
```

## Put assets in it

In the web UI there is exactly one route, and it is the bulk bar:

1. Go to **Library**. Filter or search down to what you want.
2. Hover a card and tick the checkbox at its top-left corner. Once one is
   ticked, the rest stay visible.
3. The bulk bar appears at the bottom. Press **Collection**.
4. Choose the collection from the dropdown and press **Add to collection**.

> _[Screenshot: the Library grid with assets selected, the bulk bar visible,
> and the "Add N assets to a collection" dialog open with its dropdown.]_

The dialog only lists collections that already exist — make the collection
first. Up to 1000 assets go in per request, in one transaction, and adding an
asset that is already in the collection is a quiet no-op rather than an error.

Over the API, one at a time:

```sh
curl -sS -X POST localhost:8080/api/collections/3/assets \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"asset_id":42}'
```

Or in bulk from the CLI:

```sh
dmsn collection add 3 --assets 42,43,44
```

## Take assets out again

**There is no control for this in the web UI.** The collection page has no
per-asset remove, and the asset panel's **Collections** section is a read-only
list. This is a gap, not a design statement — the endpoint exists and works:

```sh
dmsn collection remove 3 --assets 42,43
```

or directly:

```sh
curl -sS -X DELETE localhost:8080/api/collections/3/assets/42 \
  -H "Authorization: Bearer $TOKEN"
```

Removing an asset that is not in the collection is a quiet no-op. Nothing about
the asset changes — a collection membership is a link row and nothing else.

## Browse one

Click a collection card. You get the library's asset grid filtered to that
collection, at the library's default grid density, paged.

> _[Screenshot: a collection's page showing its asset grid and the pager at
> the bottom.]_

What you do *not* get is the library's filter sidebar. A collection page is
the collection, whole; you cannot narrow it by tag or licence status from
there. For that, go through the API or the CLI, which take `collection_id`
alongside every other asset filter:

```sh
dmsn assets --collection 3 --licence-status unk
dmsn assets --collection 3 --updates-available
```

```
GET /api/assets?collection_id=3&licence_status=unk
```

The library page itself does not read `collection_id` from its URL, so there
is no way to combine the two in the browser today.

To see which collections one asset belongs to, open it and expand
**Collections**.

## Rename or delete one

```sh
dmsn collection edit 3 --name "Act 3 audio"
```

In the UI, the collection page's toolbar has a **Delete** button, behind a
confirmation that tells you plainly: "Assets stay in your library — only the
collection is removed." That is exactly what happens — the link rows go, the
assets do not.

One wrinkle: **a collection's description cannot be cleared once set.** The
update endpoint ignores an empty description rather than treating it as a
deliberate blank, so `--description ""` does nothing. Overwrite it with
something else instead.

## Ideas that work well

- **"Needs a decision."** Triage inbox for packs nobody has evaluated. Empties
  over time, which is the point.
- **"Approved for commercial use."** A human-curated shortlist that survives
  licence rescans, next to the automatic
  [licence status](track-licences.md) filters.
- **"Prototype kit."** The things you reach for on day one of a jam, so a new
  starter does not have to search a library of 2000 packs.
- **A shadow of a project.** A collection of everything you are *considering*
  for a build, before any of it is pinned.

Collections are cheap. The counts show up on the Collections page and in
`dmsn`'s home view, and nothing slows down because you have forty of them.

## Things that will catch you out

- **Adding to a collection is only reachable through the bulk bar.** There is
  no "add to collection" button on an asset's own panel.
- **Removing is API-only.** Worth knowing before you add 900 assets to the
  wrong collection.
- **No ordering.** The grid sorts by the library's rules, not by the order you
  added things.
- **No nesting.** Collections are flat. If you want a hierarchy, you want
  tags.
- **A collection is library-scoped** like everything else, and cannot span
  two. See [Keep two teams' assets apart](separate-libraries.md).
- **Everything is audited** — `collection.create`, `collection.update`,
  `collection.delete`, `collection.add`, `collection.remove` — so an
  accidental bulk add is at least traceable in the library audit log.

## Where to go next

- [Pin a project to a version you have tested](pin-versions.md) — the grouping
  that does carry meaning.
- [Answer "are we allowed to ship this?"](track-licences.md) — how the bulk
  bar's other actions work.
