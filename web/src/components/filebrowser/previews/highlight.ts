import { createHighlighterCore, type HighlighterCore } from "shiki/core";
import { createJavaScriptRegexEngine } from "shiki/engine/javascript";

// Syntax highlighting for the text preview. Built on Shiki's fine-grained core
// with the JavaScript regex engine (no WASM) and per-language grammars loaded
// on demand — each `import("shiki/langs/<id>.mjs")` is a static specifier so
// Vite emits one lazy chunk per grammar, keeping the main bundle lean. The
// whole module is itself loaded only when a text file is opened (TextPreview is
// React.lazy'd), so none of this ships in the initial bundle.

export const HIGHLIGHT_THEME = "github-dark";

// Static loader thunks — Vite can only code-split `import()` with a literal
// path, so the language set is enumerated rather than templated.
const LANG_LOADERS: Record<string, () => Promise<unknown>> = {
  csharp: () => import("shiki/langs/csharp.mjs"),
  javascript: () => import("shiki/langs/javascript.mjs"),
  typescript: () => import("shiki/langs/typescript.mjs"),
  python: () => import("shiki/langs/python.mjs"),
  hlsl: () => import("shiki/langs/hlsl.mjs"),
  shaderlab: () => import("shiki/langs/shaderlab.mjs"),
  glsl: () => import("shiki/langs/glsl.mjs"),
  json: () => import("shiki/langs/json.mjs"),
  xml: () => import("shiki/langs/xml.mjs"),
  yaml: () => import("shiki/langs/yaml.mjs"),
};

let corePromise: Promise<HighlighterCore> | undefined;
const loaded = new Set<string>();

function getCore(): Promise<HighlighterCore> {
  return (corePromise ??= createHighlighterCore({
    themes: [import("shiki/themes/github-dark.mjs")],
    langs: [],
    engine: createJavaScriptRegexEngine(),
  }));
}

/**
 * Render `code` to themed HTML for `lang`. Loads the grammar on first use and
 * caches it. An unknown grammar, or one that fails to load, degrades to
 * plaintext rather than throwing — a preview should never blow up on an
 * unexpected file. The returned HTML escapes the source (Shiki wraps escaped
 * text nodes in styled spans), so it is safe to inject.
 */
export async function highlight(code: string, lang: string): Promise<string> {
  const core = await getCore();
  let resolved = lang;
  if (lang !== "text") {
    const loader = LANG_LOADERS[lang];
    if (!loader) {
      resolved = "text";
    } else if (!loaded.has(lang)) {
      try {
        await core.loadLanguage(loader() as never);
        loaded.add(lang);
      } catch {
        resolved = "text";
      }
    }
  }
  return core.codeToHtml(code, { lang: resolved, theme: HIGHLIGHT_THEME });
}
