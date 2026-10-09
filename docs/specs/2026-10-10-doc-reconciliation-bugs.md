# Doc-reconciliation bugs

**Status:** signed off 2026-10-10, not started.

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
