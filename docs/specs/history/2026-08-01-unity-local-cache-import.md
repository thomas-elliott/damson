# Import assets from the local Unity Asset Store cache

**Status:** implemented — signed off and reconciled against main 2026-08-01, merged 2026-08-01
(PR #38, `531ea5b`).
**Depends on:** the `dmsn` CLI slice, which took this spec's original stage 1 (personal access
tokens) and stage 4 (the CLI binary) wholesale and **has shipped** — merged as
[`2026-08-01-dmsn-cli.md`](2026-08-01-dmsn-cli.md). What remains here is the Unity-specific half:
the cache-path parsing, the source columns, and the precision that turns preflight from a name
check into a real idempotency check.

## Problem

Every asset in damson arrives the same way: a human drags a `.unitypackage` onto the library
grid, or picks it out of a file dialog. Anyone with a real Unity history already has their
Asset Store purchases on disk — the reference machine has **85 packages, ~12 GB**, in
`%APPDATA%\Unity\Asset Store-5.x` — and moving those into damson today means hand-picking
files out of a hidden folder one at a time, with no way to tell which are already in the
library.

Two things make this worth solving properly rather than with a better file dialog:

1. **The cache path is metadata damson cannot get anywhere else cheaply.** Unity lays
   downloads out as `<Publisher>/<CategoryPath>/<Name>.unitypackage`. Publisher and the store
   category tree are free, and publisher in particular is what storefront enrichment is
   missing — `enrichAsset` currently matches candidates on title alone.
2. **Ingest should be repeatable.** A cache grows over time. Re-running an import must upload
   only what changed, which the current upload path has no notion of.

### What is actually on disk

Verified against the reference machine, not from documentation alone.

```
%APPDATA%\Unity\Asset Store-5.x\<Publisher>\<CategoryPath>\<Name>.unitypackage
  A2 Games\3D ModelsCharactersHumanoids\Satomi Anime-Style Character For Games And VRChat.unitypackage
  Demigiant\ScriptingAnimation\DOTween HOTween v2.unitypackage
  Blink\Editor ExtensionsGame ToolkitsRPG Toolkits\RPG Builder.unitypackage
```

- macOS `~/Library/Unity/Asset Store-5.x`, Linux `~/.local/share/unity3d/Asset Store-5.x`.
  Overridable by the `ASSETSTORE_CACHE_PATH` environment variable. There is **no** documented
  `.upmconfig.toml` key for this cache — that file governs the *UPM global* cache only — and
  the Preferences-window value lands in the Editor's own prefs store. So a client must accept
  an explicit root rather than trying to read Unity's configuration.
- **Category segments are concatenated with no separator**, and split cleanly on a
  lowercase→uppercase boundary. Checked against all 40 samples inspected:
  `3D ModelsCharactersHumanoids` → `3D Models / Characters / Humanoids`;
  `Editor ExtensionsGame ToolkitsRPG Toolkits` → `Editor Extensions / Game Toolkits / RPG Toolkits`;
  `Textures MaterialsGUI Skins` → `Textures Materials / GUI Skins`;
  `AudioSound FX` → `Audio / Sound FX`. A bare `3D Models` correctly stays whole, because that
  boundary is uppercase-space-uppercase rather than lowercase-uppercase.
- **No version information exists locally at all.** Not in the filename, not in the archive:
  the only non-GUID tar entry is `.icon.png`. Whatever version was downloaded is unknowable
  from the cache.
- In-progress downloads appear as `.<Name>-content__NNNN.tmp` plus a `.tmp.json` sidecar
  holding the CDN URL and a signed key. Both must be skipped.

## Chosen approach

**The `dmsn` CLI grows a cache-aware sync**, driven by the personal-access-token credential that
slice already added, with all path parsing and all idempotency decisions owned by the server.

The CLI scans the cache, sends a manifest of `(relative_path, size)` to the existing preflight
endpoint, and uploads only the entries the server says it needs — as ordinary multipart
`POST /api/imports` requests carrying two extra form fields. Nothing about the import pipeline
changes: extraction, thumbnailing, enrichment and the `asset_imports.stage` state machine are
untouched.

Two deliberate placements:

- **The server's parse is the authoritative one, not the CLI's.** The category-splitting
  heuristic is a heuristic; what gets *persisted* is derived server-side from the raw relative
  path, so a correction ships with the server and reaches every already-installed binary. The
  CLI sends the raw path and never asserts a parse. It does share the code — one module, so
  `internal/unitycache` is imported by both — which is how `sync scan` can show a publisher
  and category column that provably match what the import will store.
- **Idempotency keys off `(source_relative_path, source_size_bytes)` persisted on the version
  row**, so a repeat sync is one JSON round trip and nobody hashes gigabytes. Exact-content
  dedup remains available as a later refinement at zero cost, because `putHashed` returns the
  hex SHA-256 — `asset_versions.original_package_object_key` *is* the content hash.

### Rejected alternatives, and why

- **A browser directory picker.** The obvious answer, and the modern API for it does not work:
  Chromium's File System Access blocklist marks `DIR_ROAMING_APP_DATA` and
  `DIR_LOCAL_APP_DATA` as `kBlockAllChildren` (and `~/Library` on macOS), so
  `showDirectoryPicker()` cannot open the Unity cache on Windows or macOS at all — it fails
  with a bare `AbortError`. The legacy `<input type="file" webkitdirectory>` has no such
  blocklist, works in every browser, and exposes `webkitRelativePath`, so a zero-install
  browser path *is* buildable and would reuse the whole server side specified here. It is out
  of scope by choice, not by constraint; if it is ever added it is a pure front-end slice
  against these same endpoints.
- **A server-side path scan** (`DAMSON_UNITY_CACHE_ROOTS` + an admin endpoint that reads the
  server's own filesystem). Cheapest to build, and a clean Docker story via a read-only
  bind-mount of `Asset Store-5.x`, but only works when damson can already see the workstation.
  Rejected as too narrow to be the only path.
- **Downloading straight from Unity's CDN.** Technically reachable — the `.tmp.json` sidecar
  shows the shape (`https://assetstorev1-prd-cdn.unity3d.com/download/<uuid>` plus a signed
  `key`), and the community `kharma_session` cookie approach is documented well enough to
  copy. Rejected: undocumented endpoints that change without notice, requires custody of the
  user's Unity credentials, and is ToS-adjacent.
- **UPM tarballs from the global cache**
  (`%LOCALAPPDATA%\Unity\cache\npm\<registry>\<pkg>\<version>\package.tgz`). Genuinely better
  metadata — a real version plus `package.json` with displayName, description and license —
  and the tar reader in `extract/unitypackage.go` would mostly carry over. Rejected for now
  because all 154 cached entries on the reference machine are first-party `com.unity.*` engine
  packages, which are noise in an asset library; Asset-Store-delivered UPM packages are rare
  in practice. Revisit if that changes.

### Versioning

`store_version` stays **blank**. Migration 018 made `store_version` the version identity and
its partial unique index permits unversioned rows, so this is a supported state. Storefront
enrichment then supplies the listing version, and the existing
`PATCH /api/assets/:id/versions/:vid` lets a human correct it.

Blank does not happen by itself. `ParseImportFilename` peels a trailing `v`-prefixed or
dotted token off the filename, and cache names contain those as *product names*:
`DOTween HOTween v2.unitypackage` parses to name `DOTween HOTween`, store version `2`. That
is a fiction, and because `store_version` is a per-asset uniqueness key it blocks the real
`2.1.0` later. `postImport` only overrides the parsed version when the `store_version` form
field is non-empty, so a client cannot currently ask for blank.

So **`source_kind = unity_cache` suppresses the filename version parse**: the name is parsed
as usual, the version is discarded, and an explicit `store_version` form field still wins if
one is sent. One rule, server-side, no extra form field.

The alternatives were both worse. Stamping the storefront's *current* version is silently
wrong whenever the cached download is older than the listing. Using the file's mtime puts
something that is not a store version into the field the UI and enrichment both treat as one.

### Publisher precedence

`ApplySourceHints` writes the cache folder's publisher to `assets.publisher` before the enrich
stage, and `enrichFillBlanks`' existing blanks-only rule then leaves it alone — no special case
in the enrich path. The folder name *is* the publisher's own store name, so the two almost
always agree; where they disagree, damson keeps the folder name. Accepted, and noted here so a
later reader does not mistake it for an oversight.

## Task breakdown

Ordered stages, roughly one commit each.

**Already delivered by the `dmsn` CLI slice** — these were this spec's stages 1 and 4:

- Personal access tokens: migration `020_api_tokens.sql`, `models.APIToken`,
  `auth.TokenAuthenticator` branched on the `damson_pat_` prefix, `read`/`write` scopes,
  `GET`/`POST`/`DELETE /api/me/tokens`, and `ApiTokensSection.tsx` in the SPA.
- The CLI itself: `cmd/dmsn`, with `login` / `whoami` / `assets` / `upload` and the AXI output
  contract in `internal/toon` + `internal/axi`.
- `POST /api/imports/preflight` in its **name-only** form, returning `new` / `name_match`.
  This spec widens it rather than creating it.

### Stage 1 — `internal/unitycache`

The parser first, because stage 2 imports it. A package rather than CLI-local code, so it is
testable under the existing Go tier, usable from the server, and reusable if a browser or
server-side path is ever added. **One implementation, not two:** `internal/services/imports`
imports this package rather than carrying its own copy of the heuristic — same module, no
dependency cost, and the CLI's display of publisher/category cannot drift from the values the
server persists.

- `ParseCachePath(rel)` → `Parsed{Publisher, Category, Name}` and `SplitCategory(seg)` — the
  lowercase→uppercase category split. Pure, table-tested, no I/O. This is the half the server
  imports; it never touches a filesystem, so pulling it into `internal/services/imports` costs
  the server nothing.
- `DefaultRoots()` — `ASSETSTORE_CACHE_PATH` first, then the per-OS default.
- `Scan(root)` → `[]Entry{RelativePath, Parsed, SizeBytes, ModTime}`.
  Skips dotfiles (the `.tmp` / `.tmp.json` in-progress pair), non-`.unitypackage` files, and
  paths shallower than publisher/file; tolerates extra depth by joining the middle segments.
  Uses `filepath.WalkDir`, which does not follow symlinks, and caps entry count and depth so
  a pathological tree cannot run away. `ModTime` is carried for `sync scan`'s display and
  ordering only — it is deliberately *not* a version source, see Versioning.

### Stage 2 — Import source fields

- Migration `021_import_source.sql` — `asset_versions.source_kind`,
  `.source_relative_path` (indexed), `.source_size_bytes`. (`assets.publisher` already exists:
  migration `019_asset_metadata_fields.sql` added it, along with its index and its weight-C
  entry in the search vector.)
- `POST /api/imports` accepts optional `source_kind` and `source_relative_path` form fields.
  `source_kind` is a closed set — `unity_cache` is the only value this slice defines, and an
  unrecognised one is a 400 rather than a silently stored string. `source_relative_path` is
  untrusted persisted input: length-capped, and rejected if absolute or containing `..`.
  `source_size_bytes` is **not** a form field — the handler records the size storage returns
  from the blob put, which for a single-file upload is exactly what the client stat'd. The
  client's manifest size is used only to *ask* the question in preflight, never to answer it.
- `source_kind = unity_cache` suppresses the filename version parse (see Versioning).
- `imports.ApplySourceHints`, called from the import pipeline before the enrich stage so
  category tags land regardless of the `auto_enrich_enabled` toggle. Tags attach via the
  existing `tags.Attach`, which already resolves hierarchical paths; the category path is
  attached unprefixed, since the Unity categories *are* the natural vocabulary and tags are
  `unique(library_id, path)` so they merge with storefront tags. Re-running it for a new
  version of the same asset is a no-op: `tags.Attach` is idempotent and the publisher write
  is blanks-only.
- `exactMatchCandidate` takes the asset's publisher as a hint: prefer candidates whose
  normalised publisher agrees, and let a near-title match qualify on an exact publisher
  match. This is the payoff that turns the cache path from trivia into a disambiguator.

### Stage 3 — Widen preflight to path and size

`POST /api/imports/preflight` already exists and already answers the name-only question. This
stage adds the precision the cache needs: each entry gains an optional `relative_path` and
`source_kind`, and two statuses join the two that ship today. Same route, additive on both
sides — an entry with no `relative_path` still answers exactly as it does now, so the shipped
`upload --skip-existing` keeps working untouched.

| status | Meaning | Client behaviour | Owned by |
|---|---|---|---|
| `already_imported` | path + size match an existing version | skip | this spec |
| `updated` | path matches, size differs | upload as a new version of `asset_id` | this spec |
| `name_match` | no path match, but the name matcher hits | skip unless `--adopt-versions` | shipped |
| `new` | nothing matches | upload as a new asset | shipped |

Precision details, all of which the shipped name-only half did not have to answer:

- **The lookup is library-scoped through `assets`.** `asset_versions` carries no `library_id`,
  so the path query joins `asset_versions → assets` filtered on `assets.library_id` — the
  tenancy rule from `AGENTS.md` holds here as everywhere. Matching is on
  `(source_kind, source_relative_path)`, not path alone, so a future source cannot collide
  with a cache path.
- **One read, not one query per entry.** The batch matcher (`imports.NewNameMatcher`) already
  reads the library once per request; the path lookup joins the same way, with a single
  `WHERE source_relative_path IN (…)` over the manifest.
- **A path can match several versions**, because an `updated` re-upload writes a second
  version carrying the same path. The newest by `import_date` wins — that is the one whose
  size answers "do I already hold these bytes?".
- **Entry cap.** `maxPreflightEntries` is 2000 and exceeding it is a 400, not a truncation.
  `sync push` chunks its manifest at that size rather than relying on the cache being small.

### Stage 4 — `dmsn sync`

Two subcommands on the existing binary, not a second one:

```
dmsn sync scan [--cache-root PATH]
dmsn sync push [--cache-root PATH] [--dry-run] [--publisher NAME]
               [--concurrency N] [--adopt-versions]
```

- Credentials, the HTTP client, the AXI render layer, `--dry-run`, `--concurrency` and the
  fixed-pool per-item failure handling come from the CLI slice unchanged.
- **The upload path does not.** Today `uploadOne` reads `asset_id` and `store_version` off the
  shared `*uploadFlags`, and `Client.PostImport(ctx, filename, body, assetID, storeVersion)`
  is positional. A cache sync is per-item on both counts — an `updated` entry targets its own
  `asset_id`, and every entry carries its own `source_relative_path`. So this stage first
  turns those into per-item data: fields on `item`, and an options struct on `PostImport` in
  place of the two trailing strings. A mechanical change, but it is a change to shipped code
  and the golden-output fixtures for `upload` must stay green across it.
- `scan` prints publisher / package / size / preflight status, so it doubles as a diff view.
  It needs credentials for the status column; without them it still lists the cache and says
  so, rather than failing.
- `push` = scan → chunked preflight → upload only what needs it, passing `source_kind` and
  `source_relative_path` on each `POST /api/imports`.
- `--adopt-versions` governs `name_match` only — the status that means *the library holds
  something by this name, but nothing with this path*. Off (the default), those entries are
  skipped, because a name match cannot tell a duplicate from a newer version and uploading
  blind would double up an existing asset. On, they upload as a new version of the matched
  `asset_id`, which also back-fills the source path so the next sync answers precisely. The
  first sync against a library populated by hand is exactly the case it exists for.
- No new Go dependency, keeping `task mod:tidy:check` quiet.
- No distribution plumbing: `task build:cli` builds it, as it does the rest of the CLI.

### Stage 5 — Agent surface and docs

- `dmsn setup`'s skill template gains `sync scan` / `sync push`; a command an agent cannot see
  is a command it will not use.
- `architecture.md` §18a gains the sync commands and the source columns; `progress.md` gains a
  slice entry; this spec stays in place as the record of why.

## Testing

Mirroring the CLI slice's split, so nothing here needs a new test tier.

- Stage 1 is pure table tests — every category sample in *What is actually on disk*, plus the
  `3D Models` case that must **not** split, plus `Scan` against a `t.TempDir()` fixture tree
  carrying an in-progress `.tmp` pair, a stray non-package file, and a symlink.
- Stages 2 and 3 sit in the Postgres-backed Go tier (`newTestHandler`): the source columns
  round-tripping through `POST /api/imports`, `unity_cache` suppressing the version parse on
  `DOTween HOTween v2.unitypackage` specifically, the rejection of absolute and `..` paths,
  all four preflight statuses, the newest-version tie-break, and cross-library isolation —
  another library's identical cache path must read as `new`.
- Stage 4 tests against an `httptest` server with golden-output fixtures, as `upload` does,
  including a manifest over the 2000 cap to prove chunking.

## Out of scope

- Any SPA import surface, including the `webkitdirectory` picker described above.
- Personal access tokens and the CLI itself — both shipped in
  [`2026-08-01-dmsn-cli.md`](2026-08-01-dmsn-cli.md).
- UPM `.tgz` packages.
- Resumable uploads.
- Two-way sync, or deleting from damson when a package leaves the cache. This is import only.
