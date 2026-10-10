# Doc-reconciliation bugs

**Status:** implemented — 2026-10-10, branch `doc-reconciliation-bugs` (PR not yet opened).

Covers DAM-32, DAM-33, DAM-34, DAM-35 and DAM-36

## Why are we doing this

The 2026-10-10 doc reconciliation found 29 defects. They include a token that can escalate to a session, uploads that fail behind the real server, and gallery images that get deleted. A library should never lose or misreport what it holds, and the checks the docs promise should actually run.

## Steering

- Session, 2026-10-10, operator: "the five doc-reconciliation bug sessions, as one unit of work, in this order" (DAM-32, DAM-33, DAM-34, DAM-35, DAM-36).
- DAM-33 Direction: "DAM-29: audit the import pipeline's writes, so the rule that every library-scoped write is audited holds."
- DAM-34 Direction: "DAM-15: make the Unity preview setting control the harvested preview." / "Preview formats tier 2 is scoped in docs/roadmap.md (Shaped): PSD, EXR and TGA decoding, SVG rasterising, PDF posters. Still out: .blend, Unity scene/prefab reconstruction, AIFF transcoding, rtf/doc conversion."
- DAM-35 Direction: "DAM-27: remove the library default-theme setting rather than wire it up."
- DAM-36 Direction: "Runs after the other four sessions: once the gates are on, they fail on anything those sessions have not fixed yet." / "The roadmap's Storybook a11y gate item stays out of this session."

## Design

none — bug fixes against the standing docs, with the answers below. Preview formats tier 2 fits `docs/roadmap.md` (Shaped) and `docs/specs/history/2026-06-18-file-previews.md`, which keeps decoding pure Go: no CGo and no external binary.

## Decisions

### What does a personal access token get from /auth/refresh? (DAM-8)
Answer: the same 403 "not available to personal access tokens" as every other route that refuses tokens. No session is minted.
By: settled by `internal/auth/middleware.go` (2026-10-10)

### Which upload chunks must get through the server? (DAM-9)
Answer: chunk uploads up to the full 32 MiB chunk size succeed behind the real server, from both the web app and `dmsn`. JSON bodies elsewhere keep their 1 MiB cap.
By: settled by `docs/progress.md`, Known gaps: "Chunk size is a Go constant" (2026-10-10)

### Which proxies does the server trust for the client IP? (DAM-11)
Answer: none by default. A new `DAMSON_TRUSTED_PROXIES` env var (a list of CIDRs) names the proxies whose X-Forwarded-For is believed. The operator sets it in gitops for the ingress.
By: operator — chose "None by default + DAMSON_TRUSTED_PROXIES env var (you set it in gitops)"

### What happens when the login throttle's key map is full? (DAM-11)
Answer: new keys are refused with 429. Existing buckets are never evicted, so nobody can clear someone else's lockout.
By: recommended, operator said "go"

### What does the loser of a first-run race see? (DAM-12)
Answer: local sign-up gets the existing 409 "already bootstrapped" and leaves no user row behind. On OIDC, the losing first login becomes a normal member. Either way there is exactly one sysadmin and one Personal library.
By: recommended, operator said "go"

### What happens when the container runs without extra config? (DAM-22)
Answer: the image ships a `/data` owned by the `damson` user, so the README run command works. If `/data` is not writable, startup fails with a message naming `/data` and the env vars that override it. Data is never silently ephemeral.
By: recommended, operator said "go"

### What happens to gallery images the sweep already deleted? (DAM-10)
Answer: they are repaired. Gallery rows whose object is gone are dropped and fetched again from the stored storefront listing. From then on, the sweep never deletes a gallery image or an object an import is deduping onto.
By: recommended, operator said "go"

### Does the import pipeline replace a storefront cover? (DAM-14)
Answer: no. The storefront cover always wins, as it does for re-extract and rescan.
By: settled by `internal/handlers/api/assets_rescan.go` (2026-10-10)

### What happens to file previews after re-extract or subset? (DAM-19)
Answer: subset keeps the source version's file previews. Re-extract queues fresh previews, as an import does.
By: recommended, operator said "go"

### Which licence status is right when grid and detail disagree? (DAM-21)
Answer: detail's full rule, which also reads the licence attributes. The grid, the project lists and the licence-status filter all match it. For example, a custom licence with commercial use off shows as an error everywhere.
By: recommended, operator said "go"

### Who does an import's audit row name? (DAM-29)
Answer: creating an asset or a version is audited as the importing user (`asset.create` / `asset.version.create`). Storefront enrichment and publisher hints are audited as system (`asset.update`). The audit feed's labels are also corrected where they expect `asset.tag.add` / `collection.asset.add` but the server writes `asset.tag` / `collection.add`.
By: recommended, operator said "go"

### What does the Unity preview setting do? (DAM-15)
Answer: when it is off, a package gets no cover harvested from its `preview.png` files, and the store cover or the placeholder applies instead. It defaults to on. Changing it affects new imports only. The help text says what Off gives.
By: recommended, operator said "go"

### Which zip image becomes the cover? (DAM-16)
Answer: the first image that decodes, trying names like cover, preview or thumbnail first. It is resized like any other thumbnail. If none decodes, the zip gets no cover.
By: recommended, operator said "go"

### What does a very large audio file get? (DAM-17)
Answer: a waveform, whatever its size, made without loading the whole file into memory. Users see no size limit.
By: recommended, operator said "go"

### What does a huge image get? (DAM-18)
Answer: an image over 100 megapixels gets the placeholder instead of a thumbnail. This is the same limit previews use.
By: recommended, operator said "go"

### Which formats does tier 2 add, and where? (DAM-37)
Answer: TGA, EXR and Radiance HDR are added. EXR and HDR are tone-mapped at a fixed exposure, with no user control. Grid covers and panel previews use one set of decoders, so PSD, TGA, EXR, HDR and TIFF files get a grid cover as well as a panel preview. Files already imported gain them without being re-imported.
By: recommended, operator said "go"

### Are PDF posters built? (DAM-37)
Answer: no. PDFs keep their icon in the grid, and the panel keeps pdf.js. Record PDF posters as rejected.
By: operator — chose "Leave out — PDFs keep their icon in the grid; record as rejected"

### What does an invalid Input or Textarea announce? (DAM-23)
Answer: screen readers are told the control is invalid. Error messages stay with FieldRow.
By: recommended, operator said "go"

### How far does Arrow Up or Arrow Down move in the grid? (DAM-24)
Answer: by the number of columns the grid shows, stopping at the last asset.
By: settled by `moveSelection` in `web/src/pages/LibraryPage.tsx` (2026-10-10)

### What does a chip's remove button say? (DAM-25)
Answer: it names its filter, for example "Remove type filter: Image".
By: recommended, operator said "go"

### What line height does body text use? (DAM-26)
Answer: the `text-body` token's line height.
By: settled by `docs/design-system.md` (2026-10-10)

### What happens to the library default-theme setting? (DAM-27)
Answer: it is dropped from the database and the settings API. A client that still sends it is ignored.
By: recommended, operator said "go"

### Which text sizes must reach the 12px floor? (DAM-31)
Answer: all of them, including the 9.5px badges on asset and file cards, with no exemptions. A lint guard blocks new text below 12px.
By: recommended, operator said "go"

### Which checks gate CI and `just check`? (DAM-28)
Answer: CI adds frontend lint (ESLint and check-tokens), Prettier and the Storybook story tests, and `just check` adds the story tests. The story gate goes on now, even though colour contrast stays unchecked until the roadmap's Storybook item. The import e2e spec fails on console errors, like every other happy-path spec.
By: recommended, operator said "go"

### Should a tag re-run the tests? (DAM-30)
Answer: no. The comments are corrected to say a tag promotes the main image that has already been tested. If building the image does not actually wait for the tests to pass, make it wait.
By: recommended, operator said "go"

### How are the contrast and Button comments corrected? (DAM-30)
Answer: the `tokens.css` contrast comment is changed to agree with `docs/design-system.md`, without quoting ratios. The `Button.tsx` claim is removed rather than backed by a new lint rule.
By: recommended, operator said "go"

### Where do doc corrections go?
Answer: anything this work makes untrue in `docs/` is recorded in this spec, not edited in `docs/`. That covers the 12px rule, the audit claim in architecture §14, the tier-2 roadmap entry, PDF posters as rejected and `DAMSON_TRUSTED_PROXIES`. `/distil` applies them.
By: settled by `AGENTS.md` (2026-10-10)

## Acceptance

### DAM-32 — auth and server hardening
- A write-scope personal access token gets a 403 from `POST /auth/refresh` and never receives a session.
- A 32 MiB chunk upload succeeds through the real server stack, from both the web app and `dmsn`.
- A forged X-Forwarded-For from an untrusted caller does not change the IP the login and bootstrap throttles count against.
- With `DAMSON_TRUSTED_PROXIES` set, requests through the listed proxy are throttled by the real client IP.
- Once the throttle's key map is full, a new key gets 429 and an existing lockout stays in force.
- Two concurrent first-run sign-ups, local or OIDC, leave exactly one sysadmin and one Personal library. The local loser gets 409 and leaves no user behind.
- Migrations and the queue both start cleanly after a restart, with no advisory lock left held.
- The image starts using only `DAMSON_POSTGRES_DSN` and the README's `/data` volume. If `/data` is not writable, startup fails with a message naming it.

### DAM-33 — asset data integrity
- The orphan sweep never deletes a gallery image, and an import deduping onto an object while the sweep runs keeps that object.
- Gallery images the sweep already deleted are fetched again from the storefront listing, and no gallery row points at a missing object.
- A new version or a retried import of an asset with a storefront cover keeps that cover.
- After re-extract, file previews are queued and appear. After subset, the new version shows the same previews as its source.
- After a restart, each in-flight import resumes once, for its own version only.
- An asset shows the same licence status in the grid, the project lists and the detail panel, and the licence-status filter agrees with it.
- Creating an asset or a version through import shows in the audit feed under the importing user. Enrichment and publisher hints show under system.
- Tag and collection entries in the audit feed show their proper labels.

### DAM-34 — thumbnail and extractor safety, preview formats tier 2
- With Unity preview off, a newly imported package with `preview.png` files gets no cover taken from them. With it on, it does. The help text says what Off gives.
- A zip's cover is a resized image that browsers can show. A zip with no decodable image has no cover.
- A very large WAV or MP3 gets a waveform without the import worker running out of memory.
- An image over 100 megapixels gets the placeholder, and the import worker is unaffected.
- PSD, TGA, EXR, HDR and TIFF files get a grid cover and a panel preview. EXR and HDR appear tone-mapped.
- PSD, TGA, EXR, HDR and TIFF files imported before this change gain covers and previews without being re-imported.
- PDFs keep their grid icon and panel viewer.

### DAM-35 — frontend design system and accessibility
- A screen reader announces an Input or Textarea marked invalid as invalid.
- Arrow Up and Arrow Down move exactly one visible row at every window width, including with the sidebar and detail panel open.
- Each filter chip's remove button names its filter.
- Body text uses the `text-body` line height.
- `theme_default` no longer exists in the database or in the settings API.
- No text in the SPA renders below 12px, and `just lint frontend` fails on new text below 12px.

### DAM-36 — CI gates and stale comments
- A CI pipeline fails on an ESLint error, a check-tokens violation, a Prettier difference or a failing story test.
- `just check` runs the story tests.
- The import e2e spec fails if the page logs a console error.
- No comment named in DAM-30 still contradicts the code.
- A failing test step stops the image from being built and pushed.

### Every session
- Each defect has a test that fails before its fix and passes after it.
- `just check` passes.

## Out of scope

- PDF posters (rejected).
- `.blend`, Unity scene and prefab reconstruction, AIFF transcoding and rtf/doc conversion.
- The roadmap's "Turn the Storybook a11y gate back on" item: the Tailwind fix for the story project and the palette decisions behind it.
- Re-running tests on a tag.
- Turning Damson's Chip into a wrapper over the package Chip.
- A lint rule enforcing the Button wrapper.
- Size caps on the panel's audio player.
- Editing `docs/`.

## Start here

- `docs/architecture.md`
- `docs/CONVENTIONS.md`
- `docs/design-system.md`
- `docs/specs/history/2026-06-18-file-previews.md`
- `docs/roadmap.md`

## Deferred follow-ups

- blocked: the operator sets `DAMSON_TRUSTED_PROXIES` for the ingress in gitops before the DAM-32 change reaches production. Until then, every login shares one throttle bucket.

## Implementation report

Mode: committing — branch `doc-reconciliation-bugs`, one commit per stage (AGENTS.md: "/implement commits per stage in this repo").

### Stages

DAM-32 — auth and server hardening
1. DAM-8: refuse personal access tokens on `POST /auth/refresh`.
2. DAM-9: let 32 MiB chunk PUTs through the server's body cap.
3. DAM-11: `DAMSON_TRUSTED_PROXIES`; a full throttle key map refuses new keys with 429.
4. DAM-12: serialise first-run bootstrap (local and OIDC).
5. DAM-13: pin the migration advisory lock to one connection.
6. DAM-22: ship a writable `/data` in the image; fail startup naming `/data` when it is not writable.

DAM-33 — asset data integrity
7. DAM-10: the orphan sweep keeps gallery images and objects reused mid-sweep; repair swept gallery rows.
8. DAM-14: the import pipeline keeps a storefront cover.
9. DAM-19: re-extract queues previews; subset copies the source's previews.
10. DAM-20: `ResumeInFlight` joins the import's own version.
11. DAM-21: one licence-status rule for grid, project lists, filter and detail.
12. DAM-29: audit import creation, enrichment and publisher hints; fix the audit feed's tag and collection labels.

DAM-34 — thumbnail and extractor safety, preview formats tier 2
13. DAM-15: the Unity preview setting controls the harvested `preview.png` cover.
14. DAM-16: a zip's cover is the first decodable image, resized by the generator.
15. DAM-17: audio waveforms stream instead of loading the whole file.
16. DAM-18: raster thumbnails refuse images over 100 megapixels.
17. DAM-37: TGA, EXR and HDR decoders; one decoder set for covers and previews; existing files gain them.

DAM-35 — frontend design system and accessibility
18. DAM-23: Input and Textarea set `aria-invalid` on the control.
19. DAM-24: Arrow Up/Down use the grid's own column count.
20. DAM-25: Chip remove buttons name their filter.
21. DAM-26: body text uses the `text-body` line height.
22. DAM-27: drop `theme_default` from the database and settings API.
23. DAM-31: no text below 12px; `check-tokens` guards it.

DAM-36 — CI gates and stale comments
24. DAM-28: CI runs ESLint, check-tokens, Prettier and story tests; `just check` runs story tests; import e2e fails on console errors.
25. DAM-30: correct the stale comments; image build waits for tests.

### Choices made

- Stage 2: the body cap exempts the chunk route by its registered path (`api.UploadChunkRoute`, `cmd/server/main.go` `jsonBodyCap`), not by content type, so other `application/octet-stream` bodies stay capped. The handler's own `MaxBytesReader` caps the chunk at its expected size.
- Stage 3: `DAMSON_TRUSTED_PROXIES` is comma-separated IPs or CIDRs (`internal/config/config.go`); an invalid entry stops startup. A full throttle map still drops idle buckets on its 10-minute sweep, so a spray locks out new keys for minutes, not forever (`internal/auth/throttle.go`).
- Stage 7: a write that reuses an existing object refreshes its modification time (`storage.Service.Touch`; the Put methods and the upload-open dedup call it), and `DeleteOrphans` stats each key just before deleting it. On S3 the touch is an in-place `CopyObject`, so retrying or re-importing a large package copies its objects within the bucket; Garage does this as a metadata operation, MinIO copies bytes. A reuse that lands between one key's stat and its delete — one storage round trip — is still not covered; closing it needs a lock that every writer takes.
- Stage 7: the gallery repair runs on every boot (`imports.RepairGallery`, called from `cmd/server/main.go`): one listing of `thumbs/`, and fetches only when a row's object is missing.
- Stage 10: `asset_imports.asset_version_id` (migration `027_import_version.sql`, `ON DELETE SET NULL`) records the version an import fills, backfilled with the version the pipeline used to pick (newest by import date). The pipeline, resume and retry all read it; an import whose version was deleted fails rather than running against another version (`internal/services/imports/pipeline.go` `loadJobContext`).
- Stage 12: a new asset from an import is audited as `asset.create`, a version added to an existing asset as `asset.version.create`, both in the transaction that creates the rows (`internal/handlers/api/import_create.go`). Enrichment and publisher hints write one `asset.update` row with no user, diffing the asset's columns before and after, written after the change and only logged on failure, as metadata apply does (`internal/services/imports/audit.go`). The feed also gained labels for `asset.licence.set` and `asset.version.create`, which the server writes.
- Stage 13: the Unity preview setting gates the cover the import pipeline takes from the extractor; re-extract (a user action on an existing version) still applies the harvested cover regardless (`internal/handlers/api/assets.go`). The pipeline no longer runs the thumbnail generator on a raw archive, which could never produce an image.
- Stage 14: a zip's cover candidates are its image entries up to 16 MiB; names containing "cover", "preview" or "thumbnail" go first, then archive order (`internal/services/extract/zip.go` `pickCover`). A failed cover write now leaves the zip without a cover instead of failing the extraction, as for every other thumbnail.
- Stage 17: decoders are `github.com/blezek/tga` (a fork of the TGA decoder the file-previews spec removed, without its global `image.RegisterFormat`), `github.com/mrjoshuak/go-openexr` and `github.com/mdouchement/hdr`; all pure Go, and `CGO_ENABLED=0 go build ./cmd/server` passes. PSD, TGA, EXR and HDR are chosen by extension, the rest sniffed (`internal/services/imaging`). EXR and HDR are tone-mapped with the Reinhard operator, v/(1+v), at exposure 1.0, then a 2.2 gamma. EXR is read into memory twice (once for its header, once to decode), as TGA is.
- Stage 17: existing files are backfilled at boot: `preview.QueueNewlySupported` queues unsupported files that now have a renderer, and `imports.BackfillCovers` renders a cover for assets with none a browser can show, which also repairs zip covers stored raw before stage 14. An image that still fails to decode is retried on every boot. The SPA's file-type map gained `hdr`.
- Outside the stages, to make the gate pass: migrations 001, 002 and 024 look up their four `assets` constraints by name in every schema, so after a local e2e run had migrated `damson_test.public`, each per-test schema got none, and `TestReferenceAsset_SchemaInvariants` failed. `029_schema_scoped_constraints.sql` adds any of the four the current schema lacks; in production it changes nothing. The applied migrations are left as they were.
- Stage 23: the 12px guard in `web/scripts/check-tokens.mjs` checks arbitrary Tailwind sizes (`text-[Npx]`, `text-[Nrem]`), stylesheet `font-size` and inline `fontSize`, and, like the other value checks, skips stories and tests. Six sites were below the floor, not three: the 9.5px badges on asset and file cards are counted separately. All now use `text-label`.
- Stage 24: the story tests run as their own CI step, `stories`, on the Playwright image (which carries Chromium), between `web` and `e2e-build`; lint and Prettier join the `web` step before the build. `just check` runs `just test stories` after the unit suites.

### Review — stages 1…25

#### Verdict
MED · introduced — `BackfillCovers` renders a boot-time cover from the first image file of any asset that has no store cover and no browser-viewable cover, archives included, so on the next restart a Unity package imported with Unity preview off gets a texture as its cover (against DAM-15's "the store cover or the placeholder applies") and a zip whose only images exceed the 16 MiB cover cap gets one too (against DAM-16); fix: exclude Unity packages whose library has the setting off, and apply the zip cover cap, or limit the backfill to files `imaging.NeedsConversion` reports (internal/services/imports/backfill.go:37) → fixed
MED · introduced — `checkDataDirs` requires the JWT key file's directory to be writable even when the key file already exists, so a deployment that mounts an existing key read-only at `DAMSON_JWT_KEY_FILE` (which `keyfile.go` calls safe) now fails to start where `LoadOrCreateJWTKey` would only have read it; fix: skip the check when the file exists (cmd/server/datadir.go:17) → fixed
LOW · introduced — `RepairGallery` lists `thumbs/` before it reads `asset_images`, so a gallery image that a resumed import stores in between counts as missing: its row is deleted and fetched again, and if the storefront fetch fails, the object is left unreferenced for the sweep; fix: read the rows first, or skip rows created after the listing (internal/services/imports/gallery.go:96) → fixed
LOW · introduced — `parseList` was inserted between `parseInterval`'s doc comment and `parseInterval`, so the comment "parseInterval reads a Go duration string" now heads `parseList` and `parseInterval` has none; fix: move `parseList` and its comment above that comment (internal/config/config.go:258) → fixed

#### Checked
repo rules (library_id scoping on new queries, 500 generic, no token logging, migration plus GORM struct, `just check tidy` drift), documented hazards (auth: PAT refresh, `noTokens`; storage: touch and stat before sweep delete; import pipeline: version pinning, resume join; River resume ordering), mechanical (shape conformance on grid, project and licence rows, completeness, coverage of `asset_version_id` on every `AssetImport` creator, cross-boundary audit action names checked against the server's `ActionType` strings, shared-library precedence for Chip), logic, error handling, silent failure, input validation (`DAMSON_TRUSTED_PROXIES`, `jsonBodyCap` by `FullPath`, which gin sets before global middleware runs), idempotency (migrations 027–029, backfills on every boot), credentials, security (refresh `noTokens`, trusted proxies default none, throttle refuses when full, bootstrap advisory xact lock: OIDC and local races traced under read committed), concurrency (bootstrap lock, migrate lock on one connection, sweep reuse window, gallery repair against resumed imports), external contracts (S3 self-copy with `MetadataDirective: REPLACE`), resource management (temp files, blob close), logging, performance (boot-time scans of unsupported files and coverless assets), smells · issues in: correctness/logic (backfill against DAM-15/16), error handling (startup check), concurrency (gallery repair), comments (config) · n/a: none · validation: in-context (quick depth starts no agent), 4 candidates, 4 confirmed, 0 refuted
unconfirmed: S3 `touch` replaces metadata without passing `ContentType`, so a self-copied object's Content-Type may reset to `binary/octet-stream`; not sure whether anything serves or reads the S3 Content-Type rather than the database's content type (internal/services/storage/s3.go `touch`) → checked: no S3 write path sets a Content-Type (PutObject, the staged copy and multipart all omit it), and serving takes the content type from the database, so the touch changes nothing a reader sees

### Review — cleanup

#### Verdict
MED · introduced — `RepairGallery` runs unattended on every boot and treats an empty `thumbs/` listing as the truth, so a server that starts against empty storage with a populated database (the image now ships a writable empty `/data`, so a forgotten volume passes `checkDataDirs`; LocalFS `List` returns no error for a missing directory; an S3 bucket renamed or new) deletes every `asset_images` row and refetches the lot from the storefront into the wrong store; once storage is corrected, the galleries whose listing URLs no longer resolve are gone and their real objects become sweep orphans; fix: skip the repair (and log) when the listing is empty or most rows are missing, or drop a row only after its replacement is stored (internal/services/imports/gallery.go:121, called from cmd/server/main.go:169) → fixed: the repair skips, with a warning, when fewer than half the asset covers are in storage (the sweep never deleted covers); `TestRepairGallery_LeavesRowsAloneWhenStorageIsNotTheDatabases`
LOW · introduced — `streamPCM` sizes its read buffer from the WAV header, `frame*4096` with `channels` an untrusted uint16, so a 44-byte WAV declaring 65535 channels at 32 bits allocates 1 GiB per thumbnail job (the old code's memory followed the file size), against DAM-17's "memory is bounded by the waveform's width"; fix: size the buffer in bytes (for example 64 KiB rounded down to whole frames, at least one frame) or refuse more than a sane channel count (internal/services/thumbnail/audio.go:175) → fixed: a header claiming more than 32 channels is refused; `TestGenerator_Audio_AHeaderClaimingHugeChannelsStaysWithinMemory`
LOW · introduced — `BackfillCovers` renders a zip's replacement cover from its first image by path, not the cover-, preview- or thumbnail-named entry `pickCover` tries first, so a zip whose cover was stored raw before stage 14 gets a different image from the one an import now picks (DAM-16), though the comment says it applies "the rules an import applies"; fix: order the zip candidates with `isCoverName` first, or share `pickCover`'s ordering (internal/services/imports/backfill.go:48) → fixed: `extract.IsCoverName` exported and the backfill sorts cover-like names first; `TestBackfillCovers_PrefersACoverLikeNameAsAZipImportDoes`
LOW · introduced — rescan's candidate list `thumbnailExts` still names only the old raster set, so after DAM-37's "one set of decoders" a rescan of a loose PSD, TGA, EXR or HDR asset finds no direct candidate and regenerates nothing when its preview has not rendered (previews off, or still queued), where an import gives it a cover; nothing is lost, the existing cover stays; fix: score candidates with `imaging.IsImage` instead of the hard-coded raster extensions (internal/handlers/api/assets_rescan.go:266) → fixed: images scored by `imaging.IsImage`; `TestPickThumbnailCandidate_TakesEveryImageTheDecodersRead`

#### Checked
repo rules (library_id scoping on the new boot scans and the backfill and repair queries, 500 generic, no token logging, migration plus GORM struct for 027 and 028, `just check tidy`), documented hazards (auth: `noTokens` on refresh, trusted proxies default none, throttle refuses when full; storage: touch on every reuse path, LocalFS and S3 key layout shared by `Touch`, `HeadOriginal` and `HeadBlob`, S3 multipart `Complete` rewrites the object so needs no touch, per-key stat before delete; import pipeline: every `AssetImport` creator sets `AssetVersionID`, retry with a null version answers 409, resume leaves a null version failed; River boot order: `QueueNewlySupported` before preview `ResumeInFlight`), mechanical (shape of grid and project rows with `licence_status`, `licenceStatusSQL` against `LicenceStatusFor` including the `commercial_use` JSON key and jsonb `GROUP BY`, audit action strings against `auditFormat.ts` verbs, Chip type change at every call site, Playwright image tag against `@playwright/test`), logic, error handling, silent failure, input validation (WAV header, EXR and HDR configs, zip candidates), idempotency (repair, both backfills and migration 027 on every boot), credentials, security (bootstrap advisory xact lock traced for OIDC, where each new user is uncommitted until it holds the lock, and for local; `jsonBodyCap` exemption by `FullPath`), concurrency (repair against a resumed import, backfill against preview workers, sweep against touch), external contracts (mdouchement/hdr `HDRRGBA` alpha is 0xFFFF and clamps to 1; go-mp3 streaming), resource management (blob close on every backfill path, temp write-check file removed), logging, performance (boot scans of unsupported files and coverless assets, one `thumbs/` listing per boot, S3 self-copy on dedup), deployment and rollback, tests, dependencies (three pure-Go decoders, CGO off), smells, cross-stage (sweep/touch with gallery repair and cover backfill; cover backfill with the Unity setting, zip cover rules and store covers; preview backfill with subset copies and re-extract; version pinning with retry, resume and audit; decoder set with rescan, zip cover and preview classify; `/data` check with gallery repair) · issues in: idempotency and resilience (gallery repair on empty storage), resource management (WAV buffer), cross-stage (backfill zip order; rescan decoder list) · n/a: none · validation: in-context (one MED; re-read `RepairGallery`, LocalFS `List` and the boot call from scratch), 1 candidate, 1 confirmed, 0 refuted
overlap: the four findings in `### Review — stages 1…25` are fixed in `cd2642b` and the fixes hold (backfill excludes Unity packages and zip entries over 16 MiB, existing key file skips the write check, repair reads rows before listing, `parseList` sits above `parseInterval`'s comment); not reported again
not findings: migration 028 drops `theme_default` in the same release as the code, so rolling back to the previous image breaks settings creates and saves; the spec decides the drop, and the repo's migrations are forward-only. The 12px guard does not read theme token values, but no `--text-*` token is below 12px. Stats on a skipped orphan put `err.Error()` in the sysadmin-only response, as `delete failed:` already did
unconfirmed: EXR's `exrConfig` reads the whole file before the 100 MP check, and `Decode` keeps that copy in `head` while `exrDecode` reads a second, so a large EXR (common for HDRI skies) costs about twice its file size plus the decoded image, and boot runs it through both the cover backfill and the preview workers; the double read is a recorded choice, and not sure whether any real file exceeds the server's memory limit, which would turn every boot into a retry (internal/services/imaging/imaging.go `exrConfig`)

#### Smells
possible Speculative Generality — `SeedImport` has no caller and creates an `asset_imports` row with no `asset_version_id`, which `loadJobContext` now refuses; delete it (internal/services/imports/resume.go:125) → fixed: deleted

### Results

| Item | Status | Notes |
| --- | --- | --- |
| build | passed | `CGO_ENABLED=0 go build ./...`; `just build frontend` |
| gate: `just check` | passed | lint, format, tidy, frontend build, Go and Vitest unit suites, story tests (24 files, 92 tests), e2e 18 passed |
| review: stages 1…25 | passed | `### Review — stages 1…25`: 0 HIGH, 2 MED and 2 LOW, all fixed (`cd2642b`) |
| DAM-32: PAT gets 403 from `/auth/refresh` | passed | `TestPostAuthRefresh_RefusesAPersonalAccessToken` (real middleware, Postgres); fails without `noTokens` on the route (got 200 and a JWT) |
| DAM-32: 32 MiB chunk through the server stack, web and dmsn | passed | `TestUploadChunk_FullSizeChunkPassesTheServerStack` drives `newRouter` + real handler with a session and a PAT; fails without the exemption (413) |
| DAM-32: forged X-Forwarded-For ignored | passed | `TestAuthThrottles_KeyOnTheClientIP` (login and bootstrap); fails when every proxy is trusted |
| DAM-32: trusted proxy throttles by client IP | passed | same test, trusted-proxy case |
| DAM-32: full key map refuses new keys, lockout stays | passed | `TestLoginThrottle_RefusesNewKeysWhenFull`; fails with eviction |
| DAM-32: concurrent first run → one sysadmin, one library, local loser 409 | passed | `TestCreateFirstLocalAdmin_ConcurrentSignUpsBootstrapOnce`, `TestResolveUser_ConcurrentFirstLoginsBootstrapOnce` (real Postgres); failed 3 of 3 runs before (4 winners; up to 4 sysadmins) |
| DAM-32: no advisory lock left held | passed | `TestMigrate_ReleasesTheAdvisoryLockItTook`; fails when the unlock runs on the pool. The queue's migrator already pinned its connection |
| DAM-32: image starts with DSN + `/data`; unwritable `/data` fails naming it | passed | real image (`just image-build`): `/api/health` ok with an empty volume; with an unwritable tmpfs at `/data`, exit 1 and the message names `/data` and the env vars. `TestCheckDataDirs` covers the check |
| DAM-33: sweep keeps gallery images and objects reused mid-sweep | passed | `TestSweep_NeverDeletesAGalleryImage`, `TestSweep_KeepsAnObjectAnImportDedupsOnto` (LocalFS, Postgres); both failed before. Residual window recorded in Choices |
| DAM-33: swept gallery images refetched, no row points at a missing object | passed | `TestRepairGallery_RefetchesImagesWhoseObjectIsGone`; the storefront fetch is a fake |
| DAM-33: new version or retry keeps a store cover | passed | `TestPostImport_NewVersionKeepsTheStorefrontCover`; fails without the `ThumbnailFromStore` check |
| DAM-33: re-extract queues previews; subset keeps them | passed | `TestPostReExtract_QueuesFreshPreviews`, `TestPostAssetVersionSubset_KeepsTheSourcePreviews`; both failed before (`unsupported`) |
| DAM-33: restart resumes each import once, for its own version | passed | `TestResumeInFlight_ResumesEachImportOnceForItsOwnVersion`; failed before (enqueued and marked failed) |
| DAM-33: one licence status in grid, projects, filter and detail | passed | `TestLicenceStatus_AgreesAcrossGridProjectFilterAndDetail` (5 cases); 3 disagreed before |
| DAM-33: import creation audited under the importer; enrichment and hints under system | passed | `TestPostImport_AuditsTheAssetAndVersionUnderTheImporter`, `TestEnrichAsset_AuditsItsChangesAsSystem`, `TestApplySourceHints_AuditsThePublisherAsSystem`; all failed before (no rows) |
| DAM-33: tag and collection feed labels | passed | `auditFormat.test.ts`; 5 of 6 failed before |
| DAM-34: Unity preview setting controls the harvested cover; help text | passed | `TestPostImport_UnityPreviewSettingControlsTheHarvestedCover`; the off case failed before. Help text changed in `SettingsPage.tsx`, copy only, no test |
| DAM-34: zip cover resized and browser-safe; none if nothing decodes | passed | `TestZipExtractor_CoverIsTheFirstDecodableImageResized`, `TestZipExtractor_NoDecodableImageMeansNoCover`; both failed before |
| DAM-34: large WAV/MP3 waveform without running out of memory | passed | `TestGenerator_Audio_LargeFilesStayWithinMemory`: live heap rose 572 MiB (WAV) and 714 MiB (MP3) before, under 64 MiB after |
| DAM-34: over 100 MP gets the placeholder | passed | `TestGenerator_Raster_RefusesImagesOver100Megapixels` (a real 400 MP PNG bomb); thumbnailed before |
| DAM-34: PSD, TGA, EXR, HDR, TIFF get a cover and a preview; EXR/HDR tone-mapped | passed | `TestPostImport_Tier2FormatsGetACoverAndAPreview` (PSD, TGA, EXR, HDR had no cover before), `TestDecode_Tier2Formats` |
| DAM-34: earlier imports gain covers and previews | passed | `TestBackfill_Tier2FilesImportedEarlierGainACoverAndAPreview`; `TestBackfillCovers_RespectsArchiveCoverRules` |
| DAM-34: PDFs keep their icon and viewer | passed | `TestSupports` asserts no backend preview for `.pdf`; no PDF code changed |
| DAM-35: invalid Input/Textarea announced | passed | `invalidControls.test.tsx`; failed before |
| DAM-35: Arrow Up/Down move one visible row | passed | `LibraryPage.test.tsx` with the grid reporting its columns; moved by the page's own estimate before. The browser width cases are not exercised: jsdom has no layout |
| DAM-35: chip remove buttons name their filter | passed | `FileBrowserToolbar.test.tsx`; failed before. `removeLabel` is now required in the type |
| DAM-35: body text uses the `text-body` line height | passed | e2e `body text uses the text-body line height` (Chromium); 22.5px against 23.25px before |
| DAM-35: `theme_default` gone from DB and API | passed | `TestLibrarySettings_HaveNoDefaultTheme`; failed before (column present) |
| DAM-35: no text below 12px; lint fails on new | passed | `node scripts/check-tokens.mjs` flagged 6 sites before the fix and passes after. Proof by command, not a test file |
| DAM-36: CI fails on ESLint, check-tokens, Prettier, story test | unproven | `.woodpecker.yml` `web` step runs `lint` and `format:check`, new `stories` step; Woodpecker not run from this branch (no push) |
| DAM-36: `just check` runs story tests | passed | `tools/check.just`; the gate run above ran them |
| DAM-36: import e2e fails on a console error | passed | injected `console.error` failed the spec; passes without it |
| DAM-36: no DAM-30 comment contradicts the code | passed | 14 comments corrected; dead `VITE_OIDC_*` env removed (`oidc-callback.spec.ts` 3 passed) |
| DAM-36: failing test step stops the image build | unproven | steps run in order with no `depends_on` or `failure: ignore` on test steps, so `build-push` comes after them; `ops ci list` shows no failed pipeline on main to confirm it, so nothing was changed |
| S3 storage paths (touch, stat) | unproven | covered by the fake S3 client in `s3_put_test.go`; no real S3 or Garage reached |
| every session: a test fails before each fix | passed | rows above; exceptions are DAM-31 (lint command), DAM-30 (comments) and the CI rows |

### Docs this made untrue (for `/distil`)

- `docs/design-system.md`: restore the 12px text floor; `check-tokens` now enforces it. Remove "CI does not run them" and "Neither `just check` nor CI runs the storybook project".
- `docs/architecture.md` §14 Scope: the import path now audits asset and version creation (importer) and enrichment and hints (`asset.update`, no user). §18: CI now runs lint, Prettier and story tests; drop the trap "ESLint and Prettier have no CI step". §5 traps: dedup now touches reused objects. §4.2: `asset_imports.asset_version_id`. §6.1: login throttle refuses new keys when full; client IP trusts `DAMSON_TRUSTED_PROXIES` only. §11: one decoder set (`internal/services/imaging`), tier-2 formats. §18.1: `/data` and the startup check.
- `docs/roadmap.md`: Shaped "Preview formats, tier 2" has shipped (TGA, EXR, HDR); `docs/progress.md` Known gaps: add PDF posters as rejected.
- `internal/config/config.go` documents `DAMSON_TRUSTED_PROXIES`; any env table in the docs needs it.

### Cleanup pass

Review coverage: `### Review — stages 1…25` covers every built stage; `### Review — cleanup` is the whole-branch pass. All five of its findings fixed; nothing left for triage, no open `decision:`.

Simplify: 11 edits, behaviour unchanged — the one-line `objectPresent` wrapper inlined, the 16 MiB zip cover cap shared as `extract.MaxZipCoverBytes`, an unreachable guard in `binPeaks` dropped, comments that restated a callee's doc or listed call sites cut, long comment lines re-wrapped.

Test gate:
- Cut the e2e `body text uses the text-body line height` (`web/e2e/tests/library.spec.ts`): it walks no user journey to assert one style rule, and with `body` now reading `--theme(--text-body…)` the plausible regression is a deliberate edit of that one line. It proved the fix once (22.5px against 23.25px before).
- Slimmed `TestGenerator_Audio_LargeFilesStayWithinMemory`: the MP3 case is 3 minutes, not 13 (55s → 14s). Against the pre-fix `audio.go` it still fails (141 MiB against the 64 MiB budget).
- Kept every other test: each guards a defect with a recorded fail-before, at the lowest level that reaches it.

Gate: `just check` passed after the last edit — lint, format, tidy, build, Go suite, Vitest 223, story tests 92, e2e 17.

Acceptance, as judged at cleanup: all met except the lines below.
- unproven — Arrow Up/Down "at every window width, including with the sidebar and detail panel open": the jsdom test proves the page uses the grid's own column count, not real widths. Check: in the browser, resize with sidebar and panel open and press Arrow Down.
- unproven — CI fails on ESLint, check-tokens, Prettier or a story test: Woodpecker not run from this branch. Check: the first pipeline on the PR, or a deliberate failing commit.
- unproven — a failing test step stops the image build: step order only. Check: a pipeline with a red test step shows `build-push` skipped.
- unproven (not an acceptance line) — S3 `touch` and `stat` are proved against a fake S3 client only. Check: a sweep and a dedup re-import against Garage.

Open at retirement: the `blocked:` entry in `## Deferred follow-ups` (`DAMSON_TRUSTED_PROXIES` in gitops) and the review's unconfirmed EXR double-read memory cost.
