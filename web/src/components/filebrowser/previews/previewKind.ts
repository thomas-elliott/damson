import type { FileNode } from "@/design-system";

import { previewImageMime } from "../useFileObjectUrl";

// File extension (with leading dot) → Shiki language id. Unity's serialized
// assets (.prefab/.unity/.mat/.anim/.controller/.asset/.meta) are YAML
// documents, so they highlight as YAML. "text" is Shiki's grammar-less
// plaintext mode — no grammar chunk is fetched for it.
const EXT_TO_LANG: Record<string, string> = {
  ".cs": "csharp",
  ".js": "javascript",
  ".ts": "typescript",
  ".py": "python",
  ".hlsl": "hlsl",
  ".cginc": "hlsl",
  ".compute": "hlsl",
  ".shader": "shaderlab",
  ".glsl": "glsl",
  ".json": "json",
  ".xml": "xml",
  ".yaml": "yaml",
  ".yml": "yaml",
  ".txt": "text",
  ".prefab": "yaml",
  ".unity": "yaml",
  ".mat": "yaml",
  ".anim": "yaml",
  ".controller": "yaml",
  ".asset": "yaml",
  ".meta": "yaml",
};

/** Shiki language id for a file, or undefined when it isn't a text preview. */
export function shikiLang(node: FileNode | null | undefined): string | undefined {
  if (!node) return undefined;
  return EXT_TO_LANG[node.ext ?? ""];
}

export type PreviewKind = "text" | "image" | "fallback";

/**
 * Which viewer the detail panel should mount for a file. Text wins over image
 * (a file is never both); anything with no inline viewer falls back to the
 * thumbnail/placeholder.
 */
export function previewKind(node: FileNode | null | undefined): PreviewKind {
  if (!node) return "fallback";
  if (shikiLang(node)) return "text";
  if (previewImageMime(node)) return "image";
  return "fallback";
}
