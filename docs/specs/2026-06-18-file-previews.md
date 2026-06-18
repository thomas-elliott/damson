# File Previews for the Scoped File Browser

**Status:** signed off (lavish alignment, 2026-06-18). Ready for `implement`.
**Source research:** `docs/preview-research.md` · **Scoping brief:** `docs/preview-feasibility-brief.md`

## Problem

The scoped file browser (`web/src/components/filebrowser/FileBrowser.tsx`) opens a detail
panel per file, but only previews four raster image formats inline (png/jpg/gif/webp via
`useFileObjectUrl.ts`); every other file type falls back to a type icon. We want a real
inline preview per file type — larger, richer, sometimes interactive — using the safest
feasible path for each, across the full taxonomy in
`web/src/design-system/domain/fileTypes.ts`.

Two facts about the current code shape the whole approach:

1. **An enforcing, OIDC-aware CSP already exists** (`securityHeaders` in
   `cmd/server/main.go`, applied globally). It whitelists the IdP origin in
   `connect-src`/`frame-src` for hidden-iframe silent renew, and already allows
   `img-src 'self' data: blob:` — so blob-URL image previews work under it today. Preview
   work therefore does **not** introduce a CSP; it only makes small additive widenings in
   the stages that need them (`media-src 'self' blob:` + `font-src … blob:` in Stage 3;
   `'wasm-unsafe-eval:'` for Draco in Stage 8). Widening an enforcing policy is low-risk —
   it only permits more, never locks anyone out. *(Correcting an earlier draft that wrongly
   claimed no CSP existed — it lives in `cmd/`, outside the original grep.)*
2. **Derivatives are per-asset today, but previews are per-file.** `ThumbnailObjectKey`
   lives on `Asset`; `AssetFile` has no derivative slot, and the pipeline generates one
   thumbnail from a single representative blob. Backend previews therefore need a new
   per-file column **and** a new pipeline traversal over each `AssetFile` — not a tweak to
   the existing thumbnail step.

## Chosen approach

Two delivery models, chosen per file type:

- **Live client render** — fetch the file as an authenticated blob and hand the
  `Blob`/`ArrayBuffer` to a lazy-loaded browser viewer. No backend, no derivative. Used for
  everything cheap and safe: text, code, shaders, native images/audio, fonts, JSON/XML/CSV,
  web-native 3D (obj/dae/glb), Unity-YAML, zip listing.
- **Derivative at import** — the import worker pre-renders a small, safe, web-friendly
  artifact once; the panel fetches that. Used for browser-unopenable or unsafe types:
  pro/HDR images, SVG (rasterised), PDF poster.

Backend derivatives are produced by a **new `previewing` pipeline stage** that runs after
`thumbnailing`, iterates the version's `AssetFile` rows, and stores a per-file derivative
content-addressed in the existing thumb namespace — mirroring the `thumbnail.Generator`
pattern, gated by a library toggle, idempotent and resumable like the existing stages.

On the frontend, a **`FilePreview` dispatcher** keyed by file type lazy-`import()`s the
right viewer (keeping three.js / pdf.js / Shiki out of the main bundle) and falls back to
the current thumbnail/icon when no viewer applies or a derivative is missing.

This beat the alternatives recorded in *Resolved questions*: a separate derivatives table
(rejected as premature — one derivative per file suffices), client-side SVG sanitising
(rejected in favour of backend raster to match our hostile-blob posture), and a
backend-foundation-first ordering (rejected so the zero-risk client wins ship first).

## Task breakdown

Ordered, independently committable stages. `implement` commits roughly one stage per
commit. Stages 1–3 are client-only (no deploy risk); 4–7 add the backend derivative path;
8 is the interactive 3D viewer.

### Stage 1 — Preview foundation (dispatcher + blob hook)

- **CSP:** none introduced — the enforcing policy in `cmd/server/main.go` already permits
  `img-src blob:`. Per-stage widenings (`media-src`/`font-src` in Stage 3, `wasm` in
  Stage 8) land with the stages that need them. A regression test pins the directives
  previews rely on so a future edit can't silently remove `img-src blob:`.
- **Frontend:** generalise `useFileObjectUrl` into a reusable object-URL primitive
  (`useObjectUrl(objectKey, mime)`) that does the auth `fetchBlob` → `createObjectURL` →
  revoke lifecycle for any MIME, and refactor `useFileObjectUrl` to sit on top of it. Build
  a `FilePreview` dispatcher component keyed by `FileType` (+ ext) with an extension seam
  where later stages lazy-`import()` viewers; for now it renders the inline image and
  otherwise falls back to `FileThumb` (thumbnail/icon). Wire it into `FileDetailPanel`,
  replacing the `imageSrc`-only path (`previewUrl` prop).
- **Done when:** the existing raster-image inline preview and icon fallback render through
  the new dispatcher with no regression, and the CSP regression test passes.

### Stage 2 — Text-family previews (client only)

- **Shiki** (fine-grained imports, JS regex engine, lazy grammars) for code (cs/js/ts/py),
  shaders (hlsl, shaderlab; cginc→hlsl), and plain txt.
- **react-markdown + rehype-sanitize** for `.md` (never `dangerouslySetInnerHTML`; raw HTML
  stays disabled).
- **JSON** collapsible tree, **XML** pretty-print (defensive parse — no external entities),
  **CSV** via **PapaParse** + a virtualised table.
- **Unity YAML** (prefab/unity/mat/anim/controller) → collapsible structured tree; detect
  the `%YAML`/`--- !u!` header and fall back to plain text for binary-serialised files.
  (GUID→asset-name resolution is **deferred** — see Resolved questions.)
- Enforce size ceilings: above the cap, show plain monospace text (no tokenising) or a
  "file too large — download" affordance.

### Stage 3 — Media previews (client only)

- **Native images:** extend the inline image preview with zoom/pan and an alpha
  checkerboard (CSS `conic-gradient`); a lightweight pan-zoom handler is enough.
- **Audio:** `<audio controls>` from a blob URL + **wavesurfer.js** waveform/scrub for
  wav/mp3/ogg/flac; reuse the existing waveform-thumbnail peaks where practical. AIFF →
  download-only.
- **Fonts:** live `@font-face`/`FontFace` from the blob with editable specimen text
  (unique family name per load). No library required.
- **Zip:** safe central-directory listing via a lazy `fflate`/`zip.js` — names + declared
  sizes only, **no extraction**; flag suspicious compression ratios.
- **dll / bytes:** hex/metadata view or download-only.

### Stage 4 — Per-file derivative foundation (backend + migration)

- **Migration parity:** next sequential SQL migration (`database/migrations/009_*.sql`)
  adding `preview_object_key`, `preview_content_type` to `asset_files`, **and** the matching
  GORM tags on `models.AssetFile`. Add a `PreviewsEnabled bool` (default true) to
  `library_settings` (SQL + GORM + the `loadOrCreateLibrarySettings` defaults in both the
  api and imports packages).
- **Pipeline:** add a `StagePreviewing` constant (`events.go`) and a stage in `pipeline.go`
  after `thumbnailing` — iterate the version's `AssetFile` rows, and for each eligible file
  fetch its blob, run a new preview generator, store the derivative via the content-
  addressed `PutThumb`, and write the file's preview columns. Idempotent (content-addressed
  + column rewrite) and covered by `ResumeInFlight` since it's part of the pipeline.
- **Generator:** new `internal/services/preview` (or extend `thumbnail`) with a
  classify→dispatch that returns a zero result for unsupported types (icon fallback). Seed
  it with **TIFF + BMP → PNG** re-encode using the already-present `golang.org/x/image` —
  zero new dependencies — so the stage is end-to-end verifiable before adding new decoders.
- **Serving:** a preview-blob endpoint (extend `getThumb` or add `getPreview`) gated on
  library membership + an `asset_files.preview_object_key` reference, with an image-only
  content-type allowlist (png/jpeg/webp).
- **DTO + SPA:** add `preview_object_key` (+ content type) to `assetFileView`
  (`assets_crud.go`) and its query; the dispatcher renders a backend-rendered `<img>` when
  the key is present.

### Stage 5 — Pro/HDR image derivatives (backend, pure-Go)

Add pure-Go decoders and wire them into the preview generator: **PSD** (`oov/psd`, flatten
to the merged composite), **EXR** (`mrjoshuak/go-openexr`, tone-map exposure+gamma to an
8-bit PNG), **TGA** (`ftrvxmtrx/tga`, explicit registration — no magic bytes). Apply the
`DecodeConfig`-before-`Decode` decompression-bomb guard, a configurable pixel/byte ceiling
above which we skip to download-only, `x/image/draw` downsampling, and JPEG output above a
size threshold. No CGo, no native binary, no sandbox.

### Stage 6 — SVG safe preview (backend raster)

Add `srwiley/oksvg` + `rasterx`; render SVG → PNG in the preview generator with an XML
entity-expansion / node-count / timeout guard, and download-only fallback when oksvg can't
render the file. Matches the hostile-blob posture — no active content reaches the browser.

### Stage 7 — PDF (client pdf.js + backend WASM-PDFium poster)

- **Client:** `pdfjs-dist` viewer rendering from the in-memory `ArrayBuffer`
  (`getDocument({data})`), with `isEvalSupported:false`, `enableScripting:false`, and the
  worker served same-origin (`worker-src 'self'` from Stage 1, not a blob worker). First
  page eager, subsequent pages lazy on scroll. Lazy-loaded.
- **Backend:** first-page poster via `klippa-app/go-pdfium` in **WebAssembly mode** (PDFium
  compiled to WASM, run on the pure-Go `wazero` runtime, embedded with `go:embed`) — stays
  CGo-free and single-binary; the wazero sandbox already memory-isolates it, so only a
  timeout/size cap is needed. Poster powers the grid thumbnail. **Avoid** `gen2brain/go-fitz`
  / MuPDF (CGo + AGPL).

### Stage 8 — 3D meshes (client, web-native)

Lazy **three.js** viewer: `OBJLoader`(+`MTLLoader`), `ColladaLoader`, `GLTFLoader`,
`OrbitControls`, `DRACOLoader` for compressed glb. **Best-effort** `FBXLoader` (works for
simple meshes; no backend conversion). Byte ceiling → download-only; static poster until
the user opts into the interactive viewer. `.blend` → download-only.

### Out of scope (documented skips — default to download-only)

`.blend` rendering (needs headless Blender), Unity scene/prefab **visual** reconstruction,
AIFF transcoding (needs ffmpeg), rtf/doc conversion (needs LibreOffice). Re-open only if a
customer's library is dominated by one of these.

## Reuse

- **Auth blob fetch:** `fetchBlob` (`web/src/api.ts`) + the `useFileObjectUrl` hook —
  generalised in Stage 1; the single answer to "no plain `<img src>`".
- **Backend render pattern:** `internal/services/thumbnail` (`Generator`, `Toggles`,
  content-addressed `putThumbBytes`) is the template for the preview generator.
- **Storage:** `storage.Service.PutThumb`/`GetThumb` — content-addressed, so retries and
  duplicate derivatives collapse onto the same key.
- **Pipeline:** the `imports` worker pool + staged state machine (`pipeline.go`,
  `events.go`), idempotency, and `ResumeInFlight` (`resume.go`) — the new stage slots in.
- **Toggles:** the `library_settings` `Thumbnails*` pattern (`models/settings.go`,
  `loadOrCreateLibrarySettings` in both api and imports) — mirror for `PreviewsEnabled`.
- **Decoders already vendored:** `golang.org/x/image` (tiff/bmp/webp/draw) — Stage 4 needs
  no new dep; `x/image/draw` is the shared downsample/encode path for every raster
  derivative.
- **Frontend taxonomy:** `fileTypes.ts` (`FileType`, `fileTypeFromName`, `isImageType`) and
  `FileThumb`/`FileGlyph` for the dispatcher's fallback.
- **Existing CSP to extend, not replace:** `securityHeaders` in `cmd/server/main.go`
  already enforces an OIDC-aware policy with `img-src blob:`; later stages widen it
  additively.
- **Reuse search that came up empty:** no existing per-file derivative column and no
  preview dispatcher — both are genuinely new.

## Resolved questions

- **D1 — per-file derivative data model:** columns on `AssetFile`
  (`preview_object_key`, `preview_content_type`), mirroring `Asset.ThumbnailObjectKey`. A
  separate `asset_file_derivatives` table was rejected as premature — each file needs at
  most one derivative today.
- **D2 — SVG:** backend rasterise to PNG via pure-Go `oksvg`, with download-only fallback
  for files it can't render. Chosen over client sanitise+`<img>` to match the deliberately
  restrictive hostile-blob posture.
- **D3 — PDF:** full path — client pdf.js viewer **plus** a backend first-page poster via
  WASM-PDFium. Accepts the multi-MB embedded WASM blob to keep the deployment CGo-free and
  to give PDFs a grid thumbnail.
- **D4 — 3D meshes:** three.js for obj/dae/glb live from the blob, best-effort client
  `FBXLoader`, **no** native FBX→GLB conversion stage (avoids a native binary + subprocess
  sandbox). FBX that won't load client-side → download-only.
- **D5 — staging:** one full spec, client-first order (the 8 stages above), all in this
  document; `implement` commits ~one stage each.
- **Library-settings toggle granularity:** a single `PreviewsEnabled` flag gates the whole
  backend derivative stage — not per-category mirrors of the `Thumbnails*` flags — to avoid
  settings sprawl. Revisit if per-type control is requested.
- **Stage 4 proof:** the foundation ships TIFF+BMP (already in `x/image`, no new dep) as its
  end-to-end smoke test; the new-dependency decoders (PSD/EXR/TGA) land in Stage 5.
- **Unity GUID resolution:** deferred. The first cut is a structured YAML tree; resolving
  `{fileID, guid}` references to asset names needs a project-wide `.meta` GUID index we
  don't build here.
- **AIFF / rtf / doc / .blend:** download-only for now (each needs a heavy native tool
  disproportionate to the single-binary goal).
