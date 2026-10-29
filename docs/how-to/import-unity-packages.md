# Get Unity packages in, and see what is inside them

A `.unitypackage` is an opaque blob. Unity will happily import one into a
project; it will not tell you what is in it, and neither will your file
manager. Damson unpacks the archive, lists every file it contains, renders
previews, and keeps the archive so it can do it all again later.

Reach for this when you are cataloguing a shelf of Asset Store purchases, when
you want to know whether two packs ship the same tree, or when you want one
asset out of a 4000-file pack and not the other 3999.

If you have not uploaded anything yet, start with
[`docs/getting-started.md`](../getting-started.md).

## What Damson does with the archive

A `.unitypackage` is a tar — gzipped almost always, plain occasionally — of one
directory per asset, named after that asset's Unity GUID. Each directory holds
an `asset` (the bytes), a `pathname` (where the file goes in a project), an
`asset.meta`, and sometimes a `preview.png`.

On import, Damson:

- streams every `asset` entry straight into object storage, content-addressed
  by its SHA-256, so two packs shipping the same texture store it once;
- reads each `pathname` and uses it as the file's path in the manifest;
- keeps the GUID on the file row;
- ignores `asset.meta` and anything else;
- harvests a cover image from the `preview.png` sidecars.

Which preview becomes the cover: the first one, in tar order, whose GUID is an
actual image file (`.png`, `.jpg`, `.tga`, `.psd`, `.exr`, and friends), so the
pack's tile does not end up being a mesh icon or a script glyph. Failing that,
the first preview in the archive.

Two things get dropped rather than imported, both silently and on purpose:

- **A bundle missing either its content or its pathname.** Real packages carry
  meta-only stubs; they have nowhere to go.
- **A pathname that is absolute, starts with a drive letter, or climbs out with
  `..`.** No archive gets to write outside its own tree.

The format is chosen purely by file extension, so a `.unitypackage` renamed to
`.7z` is stored as one loose blob and never extracted.

### The caps

Extraction refuses an archive that exceeds any of:

| Cap | Limit |
|---|---|
| One file, uncompressed | 2 GiB |
| All files in the package | 8 GiB |
| Entries in the tar | 200,000 |

Hitting one fails the import at the `extracting` stage with a message naming
the cap. These are zip-bomb guards, not configuration — they are not settable
from the environment or from Settings. A legitimate pack that trips one needs a
code change.

The separate, *server-wide* **Max upload bytes** (64 GiB by default,
**Settings → Server settings**) caps the transfer, and applies before any of
these.

## Import one

Drag it onto the **Library** page, or press **Upload**. The upload returns as
soon as the asset record exists, and the work happens in the background:

`uploaded` → `extracting` → `detecting` → `thumbnailing` → `enriching` →
`done`

Watch it on the **Import queue** page. A failed import keeps its stage and its
error and can be retried from there.

**Name the file properly before you upload it.** Damson peels a trailing
version token off the filename, so `Forest_Pack_v2.1.unitypackage` becomes an
asset called "Forest Pack" holding store version `2.1`. A bare trailing integer
is not treated as a version — `Rock Set 2.unitypackage` is just "Rock Set 2".

## Import the whole Asset Store cache

Unity keeps every pack you have downloaded in a local cache, laid out as
`<Publisher>/<Category>/<Name>.unitypackage`. That layout carries real
metadata, and `dmsn sync` uses it.

Build the CLI from a checkout with `just build cli`, then authenticate. Mint a
personal access token under **Settings → Access tokens** with `write` scope —
and note that **the token is pinned to whichever library was current when you
created it**. Store it:

```sh
dmsn login --server https://damson.example.com --token -
```

`--token -` reads from stdin, which keeps the secret out of your shell history
and out of the process list; `DAMSON_TOKEN` works too. `dmsn whoami` confirms
which identity, library and token you are using.

Look first:

```sh
dmsn sync scan
```

This finds the cache, sends the whole manifest to the server, and prints one
row per pack with a status:

| Status | Meaning |
|---|---|
| `new` | The library has never seen this cache path |
| `updated` | Same cache path, different size — the pack has been updated |
| `already_imported` | Same cache path, same size — nothing to do |
| `name_match` | Nothing at this path, but an asset has that name |

`name_match` is the interesting one. It means the pack got into Damson some
other way — hand-uploaded, probably — so it carries no cache path, and a name
alone cannot tell a duplicate from a newer release. Damson will not guess.

Then upload what is missing:

```sh
dmsn sync push
```

`already_imported` packs are skipped. `updated` ones go up as a **new version
of the existing asset**. `name_match` ones are skipped unless you say
otherwise:

```sh
dmsn sync push --adopt-versions
```

Useful flags: `--dry-run` prints the plan without moving bytes,
`--publisher NAME` narrows to one publisher, `--cache-root PATH` points at a
cache that is not where Unity usually puts it (or set
`ASSETSTORE_CACHE_PATH`), `--concurrency N` and `--wait` to follow each import
to a terminal stage.

> _[Screenshot: a terminal showing `dmsn sync scan` output — the cache summary
> block and the packages table with a mix of statuses.]_

### What the cache path buys you

For a pack imported this way, Damson reads the path and fills in what it says:

- **the publisher**, from the first path segment — but only if the asset's
  publisher is still blank;
- **the category**, as a tag path, so `3D/Environments/Landscapes` becomes a
  tag on the asset.

And one thing it deliberately does *not* do: **no version is parsed from the
filename.** The Unity cache holds one file per pack with no version in its
name, so a "v2" read out of a title like `Tanks v2` would be a fiction that
blocks the real version forever. Cache imports arrive with no store version at
all; set one by hand, or let a store lookup fill it (see
[Pin a project to a version you have tested](pin-versions.md)).

## Look inside a pack

Open the asset and expand **Files**. A pack with more than one file gets a
**Browse all N files** button, which opens a full-pane file browser: a folder
tree on the left, a listing in the middle, a preview pane on the right. Search
and filter by file type inside it; `Esc` steps back up a level and eventually
leaves the browser.

> _[Screenshot: the file browser open on a Unity pack, folder tree on the left,
> a texture selected and previewing on the right.]_

The GUID Damson recorded for each file is not shown anywhere in the UI or
returned by the API today. It is in `asset_files.unity_guid` if you need to
match a file against a Unity project's `.meta` by hand.

## Pull one part of a pack out

A Unity pack is usually several things in a trenchcoat. To track one of them
separately, cut a **subset version**: a new version of the same asset holding
only the files you picked.

1. Open the file browser.
2. Tick the files you want.
3. In the selection bar at the bottom, press **Create subset**.

The new version points at the same stored objects as the one it came from — no
bytes are copied — and its notes record which version it was cut from. It
carries no archive of its own; **Download original** and **Re-extract files**
keep working, because they target the newest version that still has an
archive, not the current one.

A subset is also a version, which means it competes to be the asset's current
version — and the asset's **Files** section shows the current version's files.
A version carrying a dotted store version always outranks one without, so a
subset cut from a pack that has a store version does *not* become current. Cut
one from an asset where no version has a store version and it does, on import
date. Check the **Versions** section to see which one is marked current.

## Get the original archive back

If the library keeps originals — **Settings → Storage → Retain original
packages**, on by default — the asset panel shows **Download original**. It
hands back the newest version that still has its archive, named after the
asset.

**Re-extract files** rebuilds the file manifest from that retained archive
without creating a version. Use it when extraction was wrong or incomplete and
the fix has shipped. It is Unity-only — a zip cannot be re-extracted — and the
SPA chains a preview requeue onto it, so previews come back too.

Both buttons disappear once the archive is gone.

### Turning off retention

**Retain original packages** off reclaims a lot of space: the extracted files
are what you actually browse, and the archive is a second full copy.

Know what you lose:

- **Download original** and **Re-extract files** stop working for anything
  imported after the change.
- The setting is read **at import time only**. Turning it off does not purge
  the archives you already have, and turning it back on does not bring back
  the ones you purged. The only way to get one back is to upload it again.
- Archives are content-addressed too. If two assets were imported from the
  same bytes, Damson only deletes the object when no live version still
  references it.

## Covers

By default a Unity pack's tile is the preview image harvested from inside the
archive. **Settings → Thumbnails → Unity preview harvest**, off, makes Damson
ignore it and fall back to the storefront cover (or the placeholder when
enrichment found nothing).

A storefront cover always wins over a harvested one, and is never regenerated
over: a rescan or a re-extract leaves a store cover alone.

## Things that will catch you out

- **Zips are extracted too**, through a different extractor with the same
  caps. Everything above about browsing, subsets and originals applies —
  except re-extract, which is Unity-only.
- **An entry with an unsafe pathname is skipped, not reported.** If a file you
  expected is missing from the manifest and the import said `done`, that is
  the likeliest reason.
- **Thumbnailing and enrichment never fail an import.** A pack that finishes
  with no cover and no storefront data is a successful import.
- **A `.unitypackage` that is a plain tar still works** — Damson sniffs the
  first two bytes for the gzip magic and falls through to tar.
- **Deleting an asset leaves its blobs.** Reclaiming them is the sysadmin's
  storage cleanup, documented in
  [`docs/configuration.md`](../configuration.md). The one place an import
  deletes a stored object itself is the original-archive purge described under
  [Turning off retention](#turning-off-retention), and only once no other live
  version still points at that archive.

## Where to go next

- [Answer "are we allowed to ship this?"](track-licences.md) — Asset Store
  packs all land on the same EULA.
- [Pin a project to a version you have tested](pin-versions.md) — what to do
  once `dmsn sync` starts bringing in updates.
- [`docs/architecture.md`](../architecture.md), Import pipeline — the stages
  and their traps.
