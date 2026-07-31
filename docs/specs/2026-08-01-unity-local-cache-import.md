# Import assets from the local Unity Asset Store cache

**Status:** signed off (2026-08-01) — ready for `implement`.
**Depends on:** a new personal-access-token mechanism, specified here as stage 1 because
nothing else in the tree can authenticate an unattended client.

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

**A companion CLI, `damson-sync`, as the only import surface**, driven by a new
personal-access-token credential, with all path parsing and all idempotency decisions owned by
the server.

The CLI scans the cache, sends a manifest of `(relative_path, size)` to a new preflight
endpoint, and uploads only the entries the server says it needs — as ordinary multipart
`POST /api/imports` requests carrying two extra form fields. Nothing about the import pipeline
changes: extraction, thumbnailing, enrichment and the `asset_imports.stage` state machine are
untouched.

Two deliberate placements:

- **The server parses the cache path, not the CLI.** The category-splitting heuristic is a
  heuristic; keeping it server-side means it can be corrected without redistributing a binary,
  and it gets the existing Postgres-backed Go test tier for free. The CLI sends the raw
  relative path and stays dumb.
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
its partial unique index permits unversioned rows, so this is a supported state, and
`ParseImportFilename` already yields no version for these names. Storefront enrichment then
supplies the listing version, and the existing `PATCH /api/assets/:id/versions/:vid` lets a
human correct it.

The alternatives were both worse. Stamping the storefront's *current* version is silently
wrong whenever the cached download is older than the listing — and because `store_version` is
a uniqueness key, a wrong stamp blocks the real version later. Using the file's mtime puts
something that is not a store version into the field the UI and enrichment both treat as one.

## Task breakdown

Ordered stages, roughly one commit each. Stage 1 is a prerequisite.

### Stage 1 — Personal access tokens

Local mode issues 1h JWTs (`auth.LocalTokenTTL`); OIDC mode validates IdP bearers. Neither
suits an unattended client, so this is new mechanism rather than a tweak.

- Migration `019_api_tokens.sql` — `api_tokens`: `user_id` (not null, index), `library_id`
  (not null), `label`, `token_hash` (not null, unique), `prefix`, `created_at`,
  `last_used_at`, `expires_at`, `revoked_at`. Matching `models.APIToken` with GORM tags.
- Secret format `damson_pat_<32 random bytes, base64url>`, stored as a plain SHA-256 hex
  digest — **not** bcrypt. The secret is high-entropy, so a slow KDF buys nothing and a fast
  digest allows a direct indexed lookup.
- `auth.Middleware` gains a `TokenAuthenticator` parameter (nil disables). The branch sits
  ahead of `a.Authenticate`, keyed on the `damson_pat_` prefix, so it behaves identically in
  both auth modes. On success it sets `CurrentLibraryID` to the token's library **on the
  request-scoped user copy only** — so `requireLibrary()` needs no change, and a token stays
  pinned to its library even if the user switches library in the SPA mid-sync.
  `last_used_at` updates best-effort and never fails a request.
- `GET`/`POST /api/me/tokens`, `DELETE /api/me/tokens/:id` on the `authed` group (token
  management is user-level, not library-scoped). Secret returned once, on create. Create and
  revoke are audited. The secret and its hash are never logged.
- `web/src/components/settings/ApiTokensSection.tsx`, mirroring `LibraryMembersSection`.

### Stage 2 — Import source fields and preflight

- Migration `020_import_source.sql` — `asset_versions.source_kind`,
  `.source_relative_path` (indexed), `.source_size_bytes`; `assets.publisher`.
- `POST /api/imports` accepts optional `source_kind` and `source_relative_path` form fields.
- `internal/services/imports/source.go` — `ParseUnityCachePath` and `SplitUnityCategory`.
- `imports.ApplySourceHints`, called from the import pipeline before the enrich stage so
  category tags land regardless of the `auto_enrich_enabled` toggle. Tags attach via the
  existing `tags.Attach`, which already resolves hierarchical paths; the category path is
  attached unprefixed, since the Unity categories *are* the natural vocabulary and tags are
  `unique(library_id, path)` so they merge with storefront tags.
- `exactMatchCandidate` takes the asset's publisher as a hint: prefer candidates whose
  normalised publisher agrees, and let a near-title match qualify on an exact publisher
  match. This is the payoff that turns the cache path from trivia into a disambiguator.
- `POST /api/imports/preflight` — `{entries: [{relative_path, size_bytes}]}` in,
  `{results: [{relative_path, status, asset_id?, version_id?}]}` out, with entry count
  capped:

  | status | Meaning | Client behaviour |
  |---|---|---|
  | `already_imported` | path + size match an existing version | skip |
  | `updated` | path matches, size differs | upload as a new version of `asset_id` |
  | `name_match` | no path match, but `MatchAssetByName` hits | skip unless `--adopt-versions` |
  | `new` | nothing matches | upload as a new asset |

### Stage 3 — `internal/unitycache`

A package rather than CLI-local code, so it is testable under the existing Go tier and
reusable if a browser or server-side path is ever added.

- `DefaultRoots()` — `ASSETSTORE_CACHE_PATH` first, then the per-OS default.
- `Scan(root)` → `[]Entry{RelativePath, Publisher, Category, Name, SizeBytes, ModTime}`.
  Skips dotfiles (the `.tmp` / `.tmp.json` in-progress pair), non-`.unitypackage` files, and
  paths shallower than publisher/file; tolerates extra depth by joining the middle segments.
  Symlink- and traversal-safe, mirroring the caps in `extract/unitypackage.go`.

### Stage 4 — `cmd/damson-sync`

```
damson-sync login  --server URL --token damson_pat_…
damson-sync scan   [--cache-root PATH]
damson-sync push   [--cache-root PATH] [--dry-run] [--publisher NAME]
                   [--concurrency N] [--adopt-versions]
damson-sync status
```

- Config at `os.UserConfigDir()/damson/config.json`, mode `0600`. `--token` / `DAMSON_TOKEN`
  override. `login` verifies against `GET /api/me` before writing, and never echoes the token.
- `scan` prints publisher / package / size / preflight status, so it doubles as a diff view.
- `push` = scan → one preflight round trip → upload only what needs it. Default
  `--concurrency 2`. Per-file failures are logged and skipped rather than fatal; a summary
  prints at the end and the exit code is non-zero if anything failed. Single-shot POST per
  file with retry-whole-file; `MaxUploadBytes` still applies and a 413 is reported as such.
- No new Go dependency, keeping `task mod:tidy:check` quiet.
- `task build:sync` and `task build:sync:all` (GOOS/GOARCH matrix into `dist/`), plus a
  Woodpecker step on release tags. Without distribution a CLI-only feature has no way to
  reach a user.

## Out of scope

- Any SPA import surface, including the `webkitdirectory` picker described above.
- UPM `.tgz` packages.
- Resumable uploads.
- Two-way sync, or deleting from damson when a package leaves the cache. This is import only.
