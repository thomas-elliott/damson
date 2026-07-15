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

// Audio extensions browsers decode natively (so wavesurfer/<audio> can play
// them). AIFF is deliberately absent — browsers can't reliably decode it, so it
// stays download-only rather than showing a dead player.
const AUDIO_MIME: Record<string, string> = {
  ".wav": "audio/wav",
  ".mp3": "audio/mpeg",
  ".ogg": "audio/ogg",
  ".oga": "audio/ogg",
  ".flac": "audio/flac",
};

/** MIME to re-type a natively-playable audio file to, or undefined. */
export function audioMime(node: FileNode | null | undefined): string | undefined {
  if (!node) return undefined;
  return AUDIO_MIME[node.ext ?? ""];
}

const FONT_EXTS = new Set([".ttf", ".otf", ".woff", ".woff2"]);

/** Whether a file is a browser-renderable font for a live specimen. */
export function isFontFile(node: FileNode | null | undefined): boolean {
  return !!node && FONT_EXTS.has(node.ext ?? "");
}

export type MeshFormat = "obj" | "dae" | "gltf" | "fbx";

// Mesh formats with a reliable web (three.js) loader. .blend has none, so it
// stays download-only. FBX is best-effort (closed format; the loader handles
// simple meshes).
const EXT_TO_MESH: Record<string, MeshFormat> = {
  ".obj": "obj",
  ".dae": "dae",
  ".gltf": "gltf",
  ".glb": "gltf",
  ".fbx": "fbx",
};

/** three.js loader format for a 3D file, or undefined when not web-loadable. */
export function meshFormat(node: FileNode | null | undefined): MeshFormat | undefined {
  if (!node) return undefined;
  return EXT_TO_MESH[node.ext ?? ""];
}

export type PreviewKind =
  "text" | "audio" | "font" | "archive" | "pdf" | "mesh" | "image" | "fallback";

/**
 * Which viewer the detail panel should mount for a file. Each type maps to at
 * most one viewer; anything with no inline viewer falls back to the
 * thumbnail/placeholder.
 */
export function previewKind(node: FileNode | null | undefined): PreviewKind {
  if (!node) return "fallback";
  if (shikiLang(node)) return "text";
  if (audioMime(node)) return "audio";
  if (isFontFile(node)) return "font";
  if (node.ext === ".zip") return "archive";
  if (node.ext === ".pdf") return "pdf";
  if (meshFormat(node)) return "mesh";
  if (previewImageMime(node)) return "image";
  return "fallback";
}
