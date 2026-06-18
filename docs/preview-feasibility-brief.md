# File Preview Feasibility — Research Brief

**Status:** scoping brief for a research agent. *Not* a spec or an implementation plan.
**Goal:** for each file type Damson can store, determine the feasible ways to render an
inline **preview** (libraries, specs, WASM modules, native deps) and recommend one.

---

## 1. Context the research must respect

Damson is a self-hosted digital-asset manager for game-dev assets (heavy on Unity
content). The scoped file browser is shipped; it shows a grid/list of files plus a
**detail panel**. Today the detail panel shows a backend-generated thumbnail and, for a
few raster formats, an inline full image. We now want to widen what the detail panel can
*preview* (a richer, larger, possibly-interactive view), per file type.

There are two distinct render surfaces — do not conflate them:

| Surface | Exists today | Where rendered | Scope of this research |
|---|---|---|---|
| **Thumbnail** (≤512px, grid cell) | yes — `internal/services/thumbnail` | backend → static image | reuse/extend only |
| **Preview** (detail panel, large/interactive) | raster images only | client, object URL | **the target** |

### Hard constraints (these eliminate most generic answers)

1. **Auth-gated blob delivery.** Files are *not* served from public URLs. The client
   fetches `GET /api/files/{objectKey}` with a bearer token and builds a `blob:` object
   URL (`web/src/components/filebrowser/useFileObjectUrl.ts`). Any preview that assumes a
   plain `<img src>` / `<video src>` / CDN URL **will not work** as-is — recommendations
   must work from an authenticated fetch → `Blob`/`ArrayBuffer`, or from a backend render.
2. **Security policy is restrictive on purpose.** Responses are served with
   `nosniff` + attachment disposition. SVG is *deliberately excluded* from inline preview
   (attacker-controlled active content). Any format that can carry script, external
   refs, or decompression bombs (SVG, HTML, PDF JS, SVG-in-font, zip) must come with an
   explicit risk note and a sandboxing/sanitisation strategy, or a "render on backend to
   a safe raster instead" recommendation.
3. **Two render locations are both on the table — pick per type:**
   - **Client-side** (browser-native, JS, or WASM) — good for text/code, simple images,
     audio/video the browser decodes natively.
   - **Backend pre-render to a safe artifact** (the existing thumbnail-service pattern:
     decode → emit PNG/JPEG/text). Good for formats browsers can't open (PSD, EXR, FBX,
     unitypackage internals) or that are unsafe to hand to the browser raw.
4. **Backend is Go, currently pure-Go decoders.** The thumbnail service uses
   `golang.org/x/image`, `hajimehoshi/go-mp3`, `jfreymuth/oggvorbis`,
   `golang.org/x/image/font` — **no CGo**. `libvips`/`vips` is reportedly available in the
   runtime image but is **not currently used**. For any backend-render recommendation,
   state whether it needs CGo / a native binary (ImageMagick, libvips, ffmpeg, Blender
   headless, Assimp) and weigh that against the current pure-Go, single-static-binary
   deployment.
5. **Assets can be large.** FBX meshes, layered PSDs, and EXRs can be tens–hundreds of MB.
   Flag where streaming, range requests, downsampling, or "preview a pre-rendered
   derivative instead of the original" is necessary.
6. **Stack for client work:** React + TS (strict) + Vite, TanStack Query v5, Tailwind,
   Radix primitives. Prefer libraries that are tree-shakeable, ESM, and ideally
   lazy-loadable (preview code should not bloat the main bundle).

---

## 2. Preview-derivative generation (background worker)

The likely delivery model — and a primary axis the research should evaluate against — is
**generating preview derivatives asynchronously after import**, not on-demand at view time.
For expensive types (large meshes, layered PSDs, EXRs, PDFs) we'd produce a smaller,
web-friendly, safe-to-serve artifact once, then the detail panel just fetches that. This
is the same idea as the existing thumbnail, extended to a richer "preview derivative"
(e.g. a downsampled glTF, a flattened PNG, a normalised JSON, a poster image).

**This infrastructure already exists — extend it, don't invent it:**

- `internal/services/imports` has a **bounded worker pool** (`worker.go`: `Pool`,
  `Job{ImportID, LibraryID}`, sync mode for tests) draining a job queue, with
  **resume-on-startup** for stuck rows.
- `pipeline.go` runs **staged transitions** (`StageDetecting`, `StageThumbnailing`, …)
  and **already calls the thumbnail generator as a stage**, gated by **per-library
  toggles** (`ThumbnailsRasterEnabled`, `ThumbnailsAudioEnabled`, `ThumbnailsFontEnabled`,
  `ThumbnailsUnityPreview` on `library_settings`).
- Derivatives are written to storage and referenced by object key (cf. `Asset.ThumbnailObjectKey`).

So "preview derivative" generation is naturally **a new pipeline stage (or a sibling
queue) emitting one more object key per file**, with a per-library/per-type enable toggle,
idempotent + resumable like the existing stages.

**Research questions this raises (answer per expensive type):**

1. **Derivative format** — what's the right web-friendly, safe-to-serve output? e.g.
   meshes → glTF/GLB or a rendered poster image; PSD/EXR/TGA → flattened PNG/JPEG;
   PDF → first-page poster + lazy pages; large JSON/CSV → truncated/normalised preview.
2. **Where it runs** — pure-Go in-process (preferred, matches the current pool), vs. a
   native tool the worker shells out to (Blender headless, ImageMagick/libvips, ffmpeg,
   Assimp). State the dependency cost against the single-static-binary deployment.
3. **Sandboxing — keep it proportional.** A goal is to *avoid* heavy isolation if we can.
   For each type that needs an untrusted native parser, say what the *minimum* viable
   containment is and whether it's even warranted:
   - in-process pure-Go with resource caps (timeout, max-dimensions, decompression-bomb
     guards) — lightest, already the thumbnail service's posture;
   - subprocess with rlimits / timeout / temp-dir scoping — moderate;
   - container/seccomp/gVisor/WASM sandbox — only if a format genuinely demands running
     untrusted native code (flag explicitly; treat as last resort).
   Note which types are safe enough to *not* need a sandbox at all.
4. **Cost controls** — per-library toggle (matches existing pattern), size ceilings above
   which we skip derivative generation and fall back to download-only, and how failures
   degrade (icon/metadata fallback, like the thumbnail service does on decode error).

Where a type is cheap and safe to render live in the browser from the auth blob (text,
code, native images/audio, fonts), the worker path is unnecessary — call that out so we
don't over-build. The deliverable per type (§4) should say **derivative-at-import vs.
live-client-render**.

## 3. File types to research

Taxonomy and extension map live in `web/src/design-system/domain/fileTypes.ts`. Grouped
below with the extensions Damson classifies today and the key question per group.

| Group | Extensions | Today | Core research question |
|---|---|---|---|
| **Raster images** | png, jpg/jpeg, gif, webp | inline preview ✓ | Anything beyond native `<img>` needed (zoom/pan, colour profile, alpha checkerboard)? |
| **Pro/HDR images** | tga, psd, exr, tif/tiff, bmp | thumbnail only | Browser can't decode these. Client-side WASM decoders vs. backend pre-render to PNG. PSD = flatten or show layers? EXR = tone-mapping. |
| **Vector** | svg | excluded (security) | Safe inline render: sanitise (DOMPurify) + `<img>`/inline, or backend rasterise. State residual risk. |
| **3D meshes** | fbx, obj, blend, dae | icon only | Interactive WebGL viewer (three.js + loaders) vs. backend-rendered turntable/still. Which formats have reliable web loaders? `.blend` and `.fbx` are the hard ones. |
| **Unity assets** | prefab, unity (scene), mat, anim, controller | icon only | These are YAML or binary Unity serialised files referencing GUIDs. Is a *meaningful* preview possible (structured summary, referenced-asset resolution) or is best-effort a formatted text/metadata view? |
| **Code/scripts** | cs, js, ts, py | icon only | Syntax-highlighted text view: which highlighter (Shiki vs. Prism vs. highlight.js), bundle cost, lazy grammar loading. |
| **Shaders** | shader, cginc, hlsl, compute | icon only | Same as code — highlighting + grammar availability for HLSL/ShaderLab. |
| **Audio** | wav, mp3, ogg, aiff, flac | waveform thumbnail ✓ | Inline `<audio>` playback from blob + waveform/scrub UI (wavesurfer.js?). Which formats decode natively vs. need transcode (aiff, flac). |
| **Fonts** | ttf, otf, woff, woff2 | specimen thumbnail ✓ | Interactive specimen (live `@font-face` from blob, editable sample text). Native browser support per format. |
| **Documents** | pdf, md, txt, rtf, doc, docx | icon only | PDF: pdf.js feasibility under blob+CSP. Markdown: render + sanitise. docx/rtf: client lib (mammoth.js?) vs. backend convert vs. download-only. |
| **Data** | json, xml, csv, asset, bytes, dll, zip | icon only | json/xml/csv: pretty/table view (size limits, virtualisation). `.asset`/`.bytes`/`.dll`/`.zip`: realistically download-only or hex/metadata — confirm. zip = list contents safely (bomb risk). |

> `.unitypackage` itself is unpacked at import time (`internal/services/extract`); its
> internal files appear in the browser as the rows above, so it needs no separate preview.

---

## 4. What to produce per file type

For each group (or notable extension within it), the research agent should return:

1. **Verdict** — one of: native-browser, client-library, client-WASM, backend-pre-render,
   metadata/structured-only, or download-only (no practical preview).
2. **Recommended approach** + 1–2 named alternatives, each with:
   - library / spec / WASM module name, maintenance status, licence, last release;
   - bundle-size or native-dependency cost;
   - whether it works from an authenticated `Blob`/`ArrayBuffer` (constraint #1);
   - security notes (constraint #2) and required sanitisation/sandboxing.
3. **Render-location recommendation** (client vs. backend) with rationale against the
   pure-Go / single-binary deployment (constraint #4), **and timing: derivative generated
   at import by the worker (§2) vs. rendered live in the client at view time.** If a
   derivative, name its output format and any native dependency + sandboxing need.
4. **Size/perf handling** for large inputs (constraint #5).
5. **Effort signal** — rough: trivial / moderate / heavy / not-worth-it.

Prioritise breadth and feasibility findings over depth; this feeds a later
per-type implementation decision, not code.

---

## 5. Pointers

- File-type taxonomy: `web/src/design-system/domain/fileTypes.ts`
- Current inline-image preview + auth blob fetch: `web/src/components/filebrowser/useFileObjectUrl.ts`
- Detail panel (where previews render): `web/src/design-system/domain/FileDetailPanel.tsx`
- Existing backend thumbnailers (the pre-render pattern to extend): `internal/services/thumbnail/` (`raster.go`, `audio.go`, `font.go`, `thumbnail.go`)
- Import worker pool + staged pipeline (where a derivative stage would live): `internal/services/imports/` (`worker.go`, `pipeline.go`, `resume.go`)
- Per-library preview toggles to mirror: `library_settings` (`Thumbnails*` fields), `internal/models/settings.go`
- File serving endpoint + security headers: `internal/handlers/api` (the `/api/files/{objectKey}` route)
- Stack spec: `docs/CONVENTIONS.md`
