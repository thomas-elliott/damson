# Inline File Preview Feasibility Survey — "Damson" Digital-Asset Manager

## TL;DR

- **Render cheap, safe types live in the browser from the authenticated `fetch`→Blob/ArrayBuffer (no worker, no native dep, no sandbox):** text, code, shaders, native images (png/jpg/gif/webp), native-codec audio (wav/mp3/ogg/flac), fonts (ttf/otf/woff/woff2), Markdown, and JSON/XML/CSV. Building a derivative pipeline for these would be wasted effort.
- **Push expensive or browser-unopenable types through the existing import-worker pipeline as one stored derivative, behind a per-library toggle + size ceiling:** PSD/EXR/TIFF/TGA/BMP → flattened PNG/JPEG (all decodable in **pure Go, no CGo, no sandbox**); PDF → first-page poster + lazy client pages; FBX → GLB (the one mesh case needing a native tool); SVG → backend raster (matches your restrictive policy).
- **Some types are honestly "not worth it":** `.blend` (needs headless Blender), Unity scene/prefab visual rendering, and `.dll`/`.bytes` — these get a structured-text/metadata view or download-only, not a renderer.

---

## Key Findings

1. **Pure-Go decoders already cover almost all the "pro image" formats**, so backend derivative generation is nearly free and requires no CGo, native binary, or sandbox: PSD (`oov/psd`), TGA (`ftrvxmtrx/tga`), TIFF/BMP (`golang.org/x/image`), EXR (`mrjoshuak/go-openexr`, 100% pure Go, all 10 codecs, fuzz-tested).
2. **The only place a native dependency is genuinely warranted is PDF rasterisation and (optionally) FBX→GLB conversion.** For PDF, the pure-Go-compatible route is `klippa-app/go-pdfium` in **WebAssembly mode** (PDFium-as-WASM via the pure-Go `wazero` runtime): no CGo, single static binary, permissive MIT/BSD/Apache licences, and **memory-isolated by the WASM sandbox**.
3. **pdf.js works entirely from an in-memory ArrayBuffer and can be hardened under a strict CSP** (`getDocument({data})`, `isEvalSupported:false`, `enableScripting:false`), satisfying both the auth-blob constraint and the no-`unsafe-eval` policy.
4. **SVG must never be inlined into the DOM.** Backend-rasterise to PNG (preferred under your policy) or sanitise with DOMPurify and render via `<img src=blob>` (which does not execute SVG script).
5. **Client bundle hygiene is mandatory:** every heavy viewer (three.js, pdf.js, Shiki grammars, wavesurfer, UTIF/ag-psd, PapaParse) must be a lazy dynamic `import()`. All recommended libraries are ESM and tree-shakeable.

---

## Details

### Raster images: png, jpg/jpeg, gif, webp

**(a) VERDICT: native-browser** (already exists) + optional **client-library** zoom layer.

**(b) RECOMMENDED APPROACH.** Continue decoding via the browser: from the authenticated `fetch`→Blob, create a `blob:` object URL and feed `<img src={blobUrl}>` — the blob URL is same-origin and token-free, fully satisfying constraint #1. For the interactive panel add **zoom/pan**.
- **Recommended:** **OpenSeadragon** (v6.0.2, Apr 2026; BSD-3-Clause; npm `openseadragon`) for deep-zoom — heaviest but the gold standard; accepts a single-image source from a blob URL. For ordinary photos, `react-zoom-pan-pinch` or a CSS-transform handler is enough.
- **Alpha checkerboard:** pure CSS (`conic-gradient` checker behind the transparent image). No library.
- **Colour profile:** browsers apply embedded ICC profiles automatically for `<img>`; no action needed.

**(c) RENDER LOCATION + TIMING:** client-side, live, from the existing thumbnail/full blob. No derivative beyond the thumbnail you already produce.

**(d) SIZE/PERF:** For very large rasters, show the existing downsampled preview in the panel and only fetch the full blob on explicit "view full resolution." DZI tiling (OpenSeadragon) is the gigapixel escape hatch but requires tiling at import — not warranted for typical assets.

**(e) EFFORT: trivial** (native works); **moderate** with OpenSeadragon-grade zoom.

---

### Pro/HDR images: tga, psd, exr, tif/tiff, bmp

**(a) VERDICT: backend-pre-render** to flattened PNG/JPEG (preferred); **client-WASM** is a niche alternative for interactive EXR tone-mapping.

Browsers decode none of these natively. The backend path wins decisively on the pure-Go deployment.

**(b) RECOMMENDED — backend pre-render, pure Go:**
- **TIFF & BMP:** `golang.org/x/image/tiff` and `/bmp` — already in your stack. Decode → re-encode PNG/JPEG. No new dependency.
- **TGA:** `github.com/ftrvxmtrx/tga` (MIT; pure Go; passes the TGA 2.0 conformance suite). TGA has no magic bytes, so register it explicitly rather than via blanket `image.Decode`.
- **PSD:** `github.com/oov/psd` (pure Go; implements `image.Decode`; flattens the merged composite or walks layers). **Flatten to the merged composite PNG** — layer-by-layer UI is a much bigger build and most users want "what does this look like." Surface layer names/count as metadata if cheap. (`oov/psd` does no layer-effect blending, but the stored merged composite present in most PSDs is exactly the preview you want.)
- **EXR:** `github.com/mrjoshuak/go-openexr` — **100% pure Go, no CGo, all 10 compression codecs (None/RLE/ZIPS/ZIP/PIZ/PXR24/B44/B44A/DWAA/DWAB), fuzz-tested parsers.** Apply **tone-mapping** (exposure + gamma/sRGB) on the backend to an 8-bit PNG. A simpler reader, `github.com/mokiat/goexr`, registers with `image.Decode` and ships an exr→png tone-mapping tool but supports only a subset of EXR files — prefer `go-openexr` for coverage.

**Client-WASM alternatives (generally not recommended):**
- **PSD:** `ag-psd` (v30.1.1, ~2 months old; MIT; reads from ArrayBuffer — satisfies #1). Mature/active, but pushes a large parser into the browser and needs a canvas to rasterise. Use only for interactive client-side layer toggling.
- **TIFF:** `UTIF.js`/`utif2` (utif2 v4.1.0, May 2023; MIT; ArrayBuffer→RGBA). npm builds lag the GitHub source; the backend `x/image/tiff` path is simpler.
- **EXR:** `tinyexr` compiled to WASM (the W3C/Disney JERI viewer approach) gives WebGL exposure/gamma sliders — worthwhile only if interactive in-browser HDR tone-mapping is a product requirement.

**(c) RENDER LOCATION + TIMING:** **Backend pipeline at import**, one flattened PNG (or JPEG for huge images) per file. All decoders are pure Go, run in-process — **no CGo, no native binary, no subprocess, no sandbox.** This costs nothing against the single-binary deployment. Resource caps only: timeout, max output dimensions, and a decompression-bomb guard — call `DecodeConfig` before `Decode` (Go's `image` docs explicitly recommend this to avoid resource exhaustion on hostile inputs).

**(d) SIZE/PERF:** These can be hundreds of MB. (1) `DecodeConfig` first and **skip derivative generation above a configurable pixel/byte ceiling**, falling back to download-only. (2) Downsample with `golang.org/x/image/draw`. (3) Emit JPEG above a size threshold to cap artifact size.

**(e) EFFORT: moderate** (wire four pure-Go decoders into one stage; EXR tone-mapping is the only fiddly part).

---

### Vector: svg

**(a) VERDICT: backend-pre-render to raster** (preferred under your policy), or **client-library** sanitise-and-`<img>` as the lighter alternative. Never inline `<svg>` into the DOM.

**(b) RECOMMENDED APPROACH.**
- **Option A (safest, matches your posture): backend rasterise to PNG.** SVG is deliberately excluded from inline preview because it carries active content (scripts, external refs, `<foreignObject>`, billion-laughs risk). Rasterising server-side removes all of it. Cost: pure-Go SVG rasterisation is limited — `github.com/srwiley/oksvg` + `rasterx` (pure Go) handles a useful subset, but complex SVGs (filters, advanced text) render imperfectly. Since SVGs in an asset manager are usually simple icons/logos, **pure-Go oksvg with a "rendered approximately" caveat is proportionate.** If fidelity ever matters, this is the only place a native dep (resvg/librsvg) might be accepted.
- **Option B (client, lighter): sanitise with DOMPurify, render via `<img src=blob>`.** **DOMPurify** (v3.4.10; MPL-2.0 OR Apache-2.0; actively maintained, security-audited by cure53) strips scripts, event handlers, and `javascript:`/`data:` URIs. Crucially, rendering the sanitised SVG through `<img src={blobUrl}>` rather than inlining means the browser **does not execute any script in the SVG at all** — `<img>`-loaded SVG is a passive context — with DOMPurify as defence-in-depth. Build the blob from the *sanitised* string, not the original.

**Residual-risk statement:** Even sanitised, inline SVG could carry CSS-based exfiltration/layout attacks; the `<img>` path eliminates script execution (the primary concern), and backend rasterisation eliminates all of it at the cost of fidelity. Given your deliberately restrictive policy, **Option A (backend raster) is the recommended default**, with download-only fallback when oksvg cannot render.

**(c) RENDER LOCATION + TIMING:** Option A — backend at import (pure-Go oksvg; no sandbox; timeout + entity-expansion guard). Option B — client live in a lazy-loaded sanitise module.

**(d) SIZE/PERF:** Guard XML entity-expansion (billion laughs) and deep nesting with a parse timeout + node-count cap; cap output raster dimensions.

**(e) EFFORT: moderate.**

---

### 3D meshes: fbx, obj, blend, dae

**(a) VERDICT:** **client-library** (WebGL) for obj/dae/glTF; **backend-pre-render** (→GLB or still) for fbx; **download-only / poster-only** for `.blend`.

**(b) RECOMMENDED — three.js (r184; MIT; npm `three` v0.184.0, Apr 2026), format-specific loaders, lazy-loaded.**
- **OBJ:** `OBJLoader` (+ `MTLLoader`) — reliable, parses from text/ArrayBuffer; works directly from the auth blob.
- **DAE (Collada):** `ColladaLoader` — works; Collada is complex/large but acceptable for preview.
- **glTF/GLB:** `GLTFLoader` — the de-facto web standard and most reliable; the target derivative format.
- **FBX:** `FBXLoader` parses ASCII FBX ≥7.0 / binary ≥6400 from ArrayBuffer, but FBX is a closed Autodesk format and the loader is least reliable (embedded textures/materials/exotic nodes often break).
- All loaders accept parsed data via `loader.parse(arrayBuffer)`, so the authenticated-fetch→ArrayBuffer path works without a URL (constraint #1). Render with `WebGLRenderer` + `OrbitControls` in a lazy React component (or `@react-three/fiber`).

**The hard ones:**
- **FBX:** (1) Try client `FBXLoader` first — often works for simple meshes. (2) For reliability, **convert FBX→GLB at import on the backend** and serve GLB to `GLTFLoader`. Conversion needs a native tool: **`FBX2glTF`** (the original `facebookincubator` repo is effectively unmaintained; the `godotengine/FBX2glTF` fork is the maintained option but is maintenance-only since Godot 4.3 dropped the dependency), **Assimp**, or **Blender headless** — all native binaries, a real cost against the pure-Go single binary. **Recommendation: ship FBX→GLB as an opt-in pipeline stage shelling out to a bundled FBX2glTF binary, behind a per-library toggle, only if customers need FBX preview.** Otherwise FBX is download-only.
- **.blend:** No reliable web loader. Faithful rendering needs **headless Blender** (hundreds of MB native dep). **Skip, or render a still poster via Blender only as an explicitly enabled niche stage. Realistically not worth it for most deployments.**

**(c) RENDER LOCATION + TIMING.**
- OBJ/DAE/GLB: **client, live**, from the auth blob — no derivative, no backend. Do not build a worker path for these.
- FBX: **derivative at import → GLB** (native tool) *or* best-effort client `FBXLoader`. Output format = **GLB**; native dep = FBX2glTF/Assimp/Blender. **Sandboxing:** feeding untrusted FBX to a large C++ parser is the one mesh case where **subprocess isolation with timeout + rlimits + scoped temp dir (moderate containment) is warranted**; container/gVisor is overkill unless you are multi-tenant and untrusting.
- .blend: derivative-only (poster) via Blender, or skip.

**(d) SIZE/PERF:** Meshes can be tens–hundreds of MB. (1) Byte ceiling above which you skip conversion → download-only. (2) Serve a **decimated/Draco-compressed GLB** (`DRACOLoader`) so the browser isn't handed a 200 MB buffer. (3) Stream GLB with range requests; show a static poster until the user opts into the interactive viewer. (4) Polygon/texture caps.

**(e) EFFORT:** OBJ/DAE/GLB **moderate**; FBX **heavy** (native conversion + sandbox); .blend **not-worth-it**.

---

### Unity assets: prefab, unity (scene), mat, anim, controller

**(a) VERDICT: metadata/structured-only.** A meaningful *visual* preview is not realistically possible without a Unity runtime.

**(b) RECOMMENDED APPROACH.** Modern Unity serialises these as **YAML documents** (class-ID-tagged, e.g. `--- !u!1 &<fileID>` for a GameObject), referencing other assets by **GUID + fileID** (the GUID lives in the sibling `.meta` file). Because it's YAML, the feasible preview is a **formatted, syntax-highlighted structured view**:
- Parse the YAML and render a **collapsible tree** of objects, components, and properties.
- **Optionally resolve GUID references** to human asset names if Damson indexes `.meta` GUIDs at import — turning `{fileID:…, guid: afa5…}` into "→ PlayerPrefab" is genuinely useful. Build this only if you already ingest `.meta` files (requires a project-wide GUID→asset index).
- `.mat` → shader name + key texture/colour properties; `.anim`/`.controller` → curve/state names; `.prefab`/`.unity` → GameObject hierarchy.
- Caveat: some Unity files (and older projects) can be **binary-serialised**; detect via the `%YAML` / `--- !u!` header and fall back to hex/metadata otherwise.

**(c) RENDER LOCATION + TIMING:** **client, live**, from the auth blob (YAML is text). GUID resolution uses a backend index lookup. No derivative, no native dep, no sandbox.

**(d) SIZE/PERF:** Scenes can be large YAML — virtualise the tree and cap displayed nodes with "show more."

**(e) EFFORT: moderate** (tree + optional GUID resolution); **trivial** if you just highlight raw YAML as text.

---

### Code/scripts: cs, js, ts, py

**(a) VERDICT: client-library** (syntax-highlighted text). No backend, no derivative.

**(b) RECOMMENDED — Shiki (MIT; ESM-only; actively maintained), lazy-loaded with fine-grained bundles.**
- Shiki uses VS Code's TextMate grammars (most accurate highlighting). Bundled languages include **`csharp` (aliases `c#`/`cs`), `javascript`, `typescript`, `python`** — all four types covered.
- **Critical bundle guidance:** the full Shiki bundle is **6.4 MB minified / 1.2 MB gzip** — do **not** import `shiki` or `shiki/bundle/full`. Use **fine-grained imports** (`shiki/core`, `shiki/engine/javascript`, `@shikijs/langs/typescript`, `@shikijs/themes/...`), lazy-load grammars on demand, and use the JavaScript regex engine (not the WASM Oniguruma engine) for the smaller browser bundle.
- Highlighting works on the text decoded from the auth blob — constraint #1 trivially satisfied.

**Alternatives (with verified current figures):**
- **Prism.js** (`prismjs` v1.30.0; ~24.5M weekly downloads per npm trends — note the standalone `prism` package is a different, near-dead package at ~9,119/wk): the lightest option, **11.0 KB gzipped** per the PkgPulse npm comparison ("Prism has the smallest gzipped bundle at 11.0 KB"). Lower fidelity than Shiki; a good choice if bundle size dominates and you don't need ShaderLab.
- **highlight.js** (v11.11.1; ~24.5M weekly downloads, ~24,937 GitHub stars per npm trends): auto-detects language, zero-config, 190+ languages, but **~297.9 KB gzipped** per the same PkgPulse comparison — much heavier than Prism.

**(c) RENDER LOCATION + TIMING:** client, live. **Do not pre-render code on the backend** — highlighting hundreds of KB client-side is instant.

**(d) SIZE/PERF:** For very large files, **virtualise** rendered lines and/or cap highlighting to the first N thousand lines; above a size ceiling show plain monospace text (no tokenising) to avoid CPU spikes.

**(e) EFFORT: trivial-to-moderate** (Shiki fine-grained setup is the only nuance).

---

### Shaders: shader, cginc, hlsl, compute

**(a) VERDICT: client-library** (syntax highlighting) — identical infrastructure to code.

**(b) RECOMMENDED — Shiki, reusing the code component.** Shiki's grammar set (via `tm-grammars`, the same grammars VS Code ships) **includes `hlsl` (scope `source.hlsl`) and `shaderlab` (scope `source.shaderlab`)**, plus `glsl`:
- `.hlsl`, `.compute` → `hlsl`.
- `.shader` (Unity ShaderLab, which embeds HLSL/Cg blocks) → `shaderlab`.
- `.cginc` (Unity Cg include) → highlight as `hlsl` (Cg ≈ HLSL) or fall back to plain text.
- Grammars originate from `tgjones/shaders-tmLanguage` (MIT) — the same ones VS Code uses (added in VS Code PR #20129), so coverage is solid.

**Alternatives:** Prism has HLSL and GLSL components but **no ShaderLab grammar** — Shiki is clearly better for Unity shader files. highlight.js has GLSL but weak HLSL/ShaderLab.

**(c) RENDER LOCATION + TIMING:** client, live, from blob. No backend, no derivative, no sandbox.

**(d) SIZE/PERF:** Shader files are small; lazy-load `hlsl`/`shaderlab` grammars only when a shader opens.

**(e) EFFORT: trivial** (reuse the code component; add two grammars).

---

### Audio: wav, mp3, ogg, aiff, flac

**(a) VERDICT: native-browser** playback + **client-library** waveform; **backend-pre-render (transcode)** only for AIFF (which browsers generally can't decode).

**(b) RECOMMENDED APPROACH.**
- **Playback:** build a `blob:` URL from the auth fetch and feed `<audio src={blobUrl} controls>` (or wavesurfer's MediaElement backend). Satisfies #1.
- **Waveform/scrub:** **wavesurfer.js** (v7.12.8, ~3 days old as of mid-2026; actively maintained; BSD; ESM; TypeScript types included). v7 decodes via the Web Audio `decodeAudioData`, so **it supports exactly the formats the browser's codecs support.**
- **Format reality (`decodeAudioData`):** **wav, mp3, ogg, flac decode natively** in current Chromium/Firefox/Safari; **AIFF does NOT decode reliably** (documented `decodeAudioData` failures; AIFF is on the community "did not work" list across browsers). FLAC is broadly supported in 2026 browsers.
- So wav/mp3/ogg/flac → **client, live, native**; AIFF → rely on `<audio>` (Safari decodes via OS codecs; Chrome/Firefox generally don't) or transcode on the backend.

**(c) RENDER LOCATION + TIMING.**
- wav/mp3/ogg/flac: **client, live** — no derivative. Don't over-build.
- AIFF: for universal playback, add a **backend transcode-to-MP3/Opus derivative** at import. Pure Go has `go-mp3`/`oggvorbis` for *decoding* but no robust general transcoder, so transcoding realistically means **shelling out to ffmpeg** (native dependency). Given AIFF is a minority format, **download-only fallback for AIFF is defensible** rather than adding ffmpeg.
- Reuse your existing **waveform thumbnail as pre-decoded peaks** for wavesurfer (its recommended approach for large files).

**(d) SIZE/PERF:** wavesurfer decodes the **entire** file in memory (a Web Audio constraint) and can fail on large/long clips. For large audio, **supply pre-decoded peaks** (reuse your waveform thumbnails) and use the MediaElement backend for streaming playback so the browser doesn't decode hundreds of MB.

**(e) EFFORT: trivial** for wav/mp3/ogg/flac; **moderate** for AIFF transcoding (a real native-dep cost — probably skip).

---

### Fonts: ttf, otf, woff, woff2

**(a) VERDICT: native-browser** (interactive specimen via `@font-face` from blob). No backend, no derivative beyond the existing specimen thumbnail.

**(b) RECOMMENDED APPROACH.** All four formats are natively supported by the browser font engine. Build a `blob:` URL from the auth fetch, inject a uniquely-named `@font-face` rule (`src: url(blob:…) format('…')`), and render **editable sample text** in that family — using a unique family name per load to defeat caching. This is exactly how client-side font converters build live previews. The **`FontFace` API** (`new FontFace(name, 'url(blob:…)').load()` → `document.fonts.add(...)`) is the cleaner programmatic route and works from a blob.
- **Format support (2026):** woff2, woff, ttf, otf all render in every modern browser; woff2 is universal. No transcoding needed.
- Optionally use `opentype.js` or `fontkit` (lazy-loaded) to extract glyph count, family/style names, units-per-em, and OpenType features for a richer specimen — but the live `@font-face` render needs **no library at all.**

**(c) RENDER LOCATION + TIMING:** client, live, from blob. **No worker path needed.** The existing specimen thumbnail covers the grid/list view.

**(d) SIZE/PERF:** Fonts are small; no special handling. (A malicious font is a theoretical parser-exploit vector, but you're handing it to the browser's own hardened font engine — the same as any web font — so no extra sandboxing is warranted.)

**(e) EFFORT: trivial.**

---

### Documents: pdf, md, txt, rtf, doc, docx

**(a) VERDICT:** **client-library** for pdf (pdf.js) and md/txt; **backend-pre-render** poster as the safe alternative for pdf and the realistic path for doc; **client-library or download-only** for docx/rtf.

**(b) RECOMMENDED APPROACH, per sub-type.**

- **PDF — pdf.js (Mozilla; `pdfjs-dist` v6.0.227, May 2026; Apache-2.0).** Renders from an **in-memory ArrayBuffer**: `getDocument({data: arrayBuffer})` (or pass the typed array) — exactly the authenticated-fetch path, no URL needed (constraint #1). Renders to `<canvas>` (no WASM needed for core rendering). **Security under your CSP:**
  - Set **`isEvalSupported: false`** — disables the only code paths that use the `Function` constructor (PostScript Type-4 and operator-list JIT), so pdf.js runs under a strict CSP **without `'unsafe-eval'`** (this also mitigated CVE-2024-4367; you're well past the fix on 6.0.x). Minor cost: some complex shading functions render slightly slower via the interpreter.
  - Set **`enableScripting: false`** — prevents execution of JavaScript embedded in the PDF. The low-level `getDocument` API doesn't run document JS unless you wire up a sandbox, but set this explicitly as defence-in-depth.
  - pdf.js uses a **Web Worker** (`pdf.worker.mjs`); allow it via `worker-src 'self'` and serve the worker as a same-origin module file (rather than a blob, to avoid needing `worker-src blob:`).
  - **Recommendation:** render PDFs **client-side with pdf.js**, first page eagerly and **subsequent pages lazily** on scroll; also generate a **first-page poster at import** for the grid/detail thumbnail.
  - **Backend alternative / fallback (safest):** pre-render the **first-page poster (and optionally each page) to PNG** on the backend, serving only flat rasters. There is **no mature native pure-Go PDF renderer.** `pdfcpu` (pure Go) manipulates but does **not** rasterise pages; `gen2brain/go-fitz` wraps MuPDF (CGo, and **AGPL** — a licensing red flag for a distributed product). The pure-Go-compatible option is **`github.com/klippa-app/go-pdfium` in WebAssembly mode** — PDFium-as-WASM via the pure-Go `wazero` runtime, embedded with `go:embed` so (per its README) "you won't have to download and distribute PDFium yourself," renders pages to `image.Image`/PNG/JPEG, and runs in a sandbox: the README notes the WASM build "basically runs in a sandbox: No chance of crashing the Go process like with cgo… No access to other local resources," with "Full control over file access (you decide which folders Wazero exposes to PDFium)." Trade-off, per the README: it's "about 2x as slow as the full native cgo implementation (but about 2x as fast as the multi-threaded cgo go-plugin implementation)," plus a multi-MB embedded blob and higher memory until the instance is closed. **For Damson, the WASM-PDFium route preserves the single-binary/no-CGo deployment and adds a real security bonus (memory isolation) over native CGo.**

- **Markdown (.md):** render with **react-markdown** (remark/rehype; builds a React vdom, never `dangerouslySetInnerHTML`) and **sanitise with `rehype-sanitize`** (or DOMPurify on output). Raw HTML in Markdown is disabled by default; if you enable `rehype-raw` you **must** pair it with `rehype-sanitize`. Client, live, from blob, lazy-loaded.

- **txt:** plain monospace view, client-side from blob (reuse the code component without a grammar). Trivial.

- **docx:** **mammoth.js** (v1.12.0, published 12 Mar 2026; BSD-2-Clause; **4,330,479 weekly downloads** per npm) converts docx→semantic HTML and **reads from an `arrayBuffer` in the browser** (satisfies #1). **Critical security note: mammoth performs NO sanitisation** and can emit `javascript:` links from the source document — you **must** run its HTML output through **DOMPurify** before rendering. Client, live, lazy-loaded. Conversion is imperfect for complex layouts (acceptable for preview).

- **rtf:** weak client options; realistically **download-only**, or backend convert via LibreOffice headless (heavy native dep, not warranted).

- **doc (legacy binary Word):** no good client library; **download-only**, or backend LibreOffice-headless convert (heavy; only if a customer demands it).

**(c) RENDER LOCATION + TIMING:**
- md/txt/docx: client, live, from blob (docx needs DOMPurify on output). No native dep.
- pdf: client live (pdf.js) **plus** a backend first-page poster at import (WASM-PDFium, pure-Go-compatible; the wazero WASM runtime already memory-isolates it, so no extra sandbox is needed beyond a timeout/size cap).
- rtf/doc: download-only.

**(d) SIZE/PERF:** PDFs can be large/many-paged — **lazy-render pages**, never render all pages at full resolution at once (Mozilla explicitly warns against rendering 100 pages at high DPI). Use range requests / progressive loading; cap poster generation above a size ceiling. For docx, cap converted HTML size.

**(e) EFFORT:** md/txt **trivial**; pdf client **moderate**, pdf backend poster **moderate** (WASM-PDFium integration); docx **moderate** (mammoth + DOMPurify); rtf/doc **trivial** (download-only).

---

### Data: json, xml, csv, asset, bytes, dll, zip

**(a) VERDICT:** **client-library** for json/xml/csv (pretty/table view); **metadata/structured-only or download-only** for asset/bytes/dll; **metadata/structured-only** (safe content listing) for zip.

**(b) RECOMMENDED APPROACH.**
- **JSON:** collapsible tree (or pretty-print + Shiki `json`). Client, from blob. Enforce a **size ceiling** and **virtualise** large trees.
- **XML:** pretty-print + Shiki `xml`, or a collapsible tree. Parse defensively — client-side `DOMParser` does not resolve external entities (helps against XXE) — and guard entity-expansion (billion laughs) with size/time caps.
- **CSV:** parse with **PapaParse** (mature, streaming, handles large files) and render a **virtualised table** (TanStack Table + a row virtualiser). Client, from blob; stream/scroll; cap rendered rows with "load more."
- **`.asset` / `.bytes`:** `.asset` is usually Unity YAML (treat like Unity assets — structured text view) or binary (hex/metadata). `.bytes` is arbitrary binary — **hex/metadata view or download-only.**
- **`.dll`:** **download-only** (optionally a metadata view: PE header, .NET assembly name/version). Never execute. No visual preview is meaningful.
- **`.zip`:** **list contents safely.** Read the central directory client-side (`fflate` / `zip.js`, lazy-loaded) to show the **file list** (names, declared sizes) **without extracting.** **Decompression-bomb defence:** do **not** auto-extract; read only the directory and flag suspicious compression ratios. If you ever extract a single entry on demand, cap output size and stop on overflow. This is metadata/structured-only, not a content render.

**(c) RENDER LOCATION + TIMING:** all client, live, from blob. No backend derivative, no native dep. The only "processing" is defensive parsing with caps; zip directory listing reads only the central directory (no bomb risk without inflation).

**(d) SIZE/PERF:** JSON/CSV/XML can be huge — **virtualise rendering, cap parse size, stream where possible** (PapaParse streams; for JSON, cap at a configurable byte ceiling and show "file too large — download"). zip directory read is cheap regardless of uncompressed size.

**(e) EFFORT:** json/xml/csv **moderate** (virtualisation is the work); asset/bytes/dll **trivial** (download-only/hex); zip listing **moderate** (safe directory read).

---

## Cross-Cutting Notes

**Shared libraries/patterns reusable across types:**
- **One auth-fetch→Blob/ArrayBuffer hook** (TanStack Query v5) underlies *every* client preview. Centralise: fetch with bearer token → `Blob` → `URL.createObjectURL` (revoke on unmount) for `<img>`/`<audio>`/`@font-face`/pdf.js; or hand the `ArrayBuffer` directly to parsers (pdf.js, three.js loaders, mammoth, UTIF, ag-psd). This is the single most reused pattern and the answer to constraint #1 everywhere.
- **One lazy-loaded "code/text" component** serves code, shaders, txt, JSON/XML pretty-print, and raw Unity YAML — all via **Shiki** with on-demand grammars.
- **DOMPurify** is the shared sanitiser for any HTML-producing path (Markdown via rehype-sanitize, docx via mammoth output, SVG Option B). Treat "HTML touched the DOM" as the trigger for sanitisation.
- **`golang.org/x/image` + `x/image/draw`** is the shared backend resize/encode path for every raster derivative (PSD/EXR/TIFF/TGA/BMP/SVG-raster/PDF-poster) — one downsample+encode helper.
- **`DecodeConfig`-before-`Decode`** is the shared decompression-bomb guard on the backend; **size ceilings + per-library toggles + icon/metadata fallback** is the shared cost-control pattern for every expensive type.
- **Lazy dynamic `import()`** for every heavy viewer keeps the main Vite bundle lean — mandatory for three.js, pdf.js, Shiki grammars, wavesurfer, UTIF/ag-psd, PapaParse.

**Security pitfalls:**
- **mammoth.js does not sanitise** — always DOMPurify its output.
- **pdf.js:** set `isEvalSupported:false` and `enableScripting:false`; serve the worker same-origin (`worker-src 'self'`).
- **SVG:** never inline `<svg>`; rasterise on backend or sanitise + `<img>`.
- **XML/zip:** XXE and decompression-bomb guards; read the zip directory only, never inflate by default.
- **Native parsers:** the WASM-PDFium route is *already* memory-isolated by wazero — a genuine security win over native CGo. FBX native conversion is the one case warranting subprocess + rlimits + temp-dir scoping; containers/gVisor are overkill unless multi-tenant and untrusting.

**What changes the build-vs-skip calculus:**
- A pure-Go decoder existing (PSD, EXR, TGA, TIFF, BMP) flips the answer to "build the backend derivative — it's nearly free." A *missing* pure-Go decoder (PDF render, FBX→GLB, .blend, AIFF transcode, rtf/doc convert) raises the cost to "native dep or skip," and the default should lean **skip / download-only** unless a customer demands it.
- **WASM-in-wazero (PDFium) is the pattern that rescues the single-binary deployment** for a format with no pure-Go renderer — worth watching for other formats, since it's how you add native-grade rendering without CGo.
- Anything cheap and safe to render live in the browser (text, code, shaders, native images, native-codec audio, fonts, Markdown, JSON/CSV/XML) should **not** get a worker derivative — building one is wasted effort.

---

## Recommendations

**Stage 1 — ship the free wins (client-only, no backend changes).** Wire the central auth-fetch→Blob hook, then implement live previews for: native images (+ checkerboard/zoom), code/shaders/txt/Markdown via a lazy Shiki+react-markdown component, native-codec audio (wav/mp3/ogg/flac) via `<audio>`+wavesurfer, fonts via `@font-face`/`FontFace`, and JSON/XML/CSV viewers (with virtualisation + size caps). Effort: trivial–moderate; covers the majority of file types with zero deployment risk.

**Stage 2 — add the pure-Go backend derivative stage.** Extend the worker pipeline with one "preview derivative" stage (idempotent, resumable, per-library toggle) emitting a flattened PNG/JPEG for PSD/EXR/TIFF/TGA/BMP (using `oov/psd`, `go-openexr`, `x/image`, `ftrvxmtrx/tga`) and a backend-rasterised PNG for SVG (oksvg, with caveat). Add `DecodeConfig` bomb-guards, size ceilings, and download-only fallback. No CGo, no sandbox.

**Stage 3 — PDF.** Render client-side with pdf.js (`getDocument({data})`, `isEvalSupported:false`, `enableScripting:false`, same-origin worker, lazy pages) and generate a first-page poster at import via WASM-PDFium (`klippa-app/go-pdfium`, webassembly backend). Keeps the binary pure-Go.

**Stage 4 — meshes and the long tail.** Add the three.js viewer (lazy) for OBJ/DAE/GLB. Treat FBX as opt-in: try client `FBXLoader`, optionally add a bundled FBX2glTF→GLB stage (subprocess + rlimits + temp-dir) behind a toggle. Add the Unity-YAML structured tree (+ optional GUID resolution if you index `.meta`). Add safe zip directory listing and hex/metadata for `.bytes`/`.dll`.

**Explicitly skip unless a customer demands it:** `.blend` visual rendering (needs Blender), Unity scene/prefab visual reconstruction, AIFF transcoding (needs ffmpeg), and rtf/doc conversion (needs LibreOffice). Default these to download-only.

**Benchmarks/thresholds that change these calls:**
- If WASM-PDFium's ~2× slowdown or memory profile proves unacceptable at your volumes, fall back to a backend-only PDF poster via an external `pdftoppm`/Ghostscript binary (accepting a non-single-binary deployment) rather than reintroducing AGPL MuPDF.
- If FBX preview demand is broad, promote FBX→GLB conversion from opt-in to default and budget for the native-binary footprint + subprocess sandbox.
- If interactive HDR inspection (per-pixel exposure/gamma) becomes a product requirement, add the tinyexr-WASM client viewer on top of the backend EXR poster.
- Revisit "skip" decisions (.blend, doc/rtf, AIFF) only when a paying customer's library is dominated by that format — the native-dependency and maintenance cost is otherwise disproportionate to the single-binary deployment goal.

---

## Caveats

- This is a **feasibility survey**, not a final design; library versions and maintenance status were verified as of **17 June 2026** and will drift.
- **`facebookincubator/FBX2glTF` is effectively unmaintained**; the `godotengine` fork is maintenance-only (Godot dropped the dependency in 4.3). Treat any FBX→GLB toolchain as a component you may have to vendor/maintain yourself, and note FBX2glTF binaries embed the proprietary Autodesk FBX SDK.
- `gen2brain/go-fitz` and MuPDF are **AGPL** — avoid for a distributed self-hosted product; this is a hard reason to prefer the BSD-licensed PDFium (via `go-pdfium`).
- **Shiki's HLSL/ShaderLab grammar coverage was confirmed via grammar provenance** (tgjones/shaders-tmLanguage → tm-grammars → Shiki) rather than a live scrape of Shiki's JS-rendered language table; the grammar IDs `hlsl`, `shaderlab`, `csharp`, `glsl` are current for mid-2026 but verify against your pinned Shiki version.
- **Web Audio AIFF support is browser/OS-dependent** (Safari may decode via OS codecs where Chrome/Firefox don't); test against your target browser matrix before deciding AIFF playback strategy.
- **pdf.js with `isEvalSupported:false`** trades a small rendering-speed cost on complex PostScript Type-4 shadings for CSP compliance — acceptable for previews but worth confirming on representative PDFs.
- **oksvg renders only a subset of SVG faithfully**; complex filters/text will look wrong. If SVG fidelity becomes important, that is the one place to reconsider a native rasteriser (resvg/librsvg) against the no-native-dep goal.
- Pure-Go decoder edge cases: `oov/psd` does not blend layer effects (relies on the stored merged composite); `mokiat/goexr` supports only a subset of EXR (prefer `mrjoshuak/go-openexr`); validate against a corpus of real customer files before enabling a derivative type by default.