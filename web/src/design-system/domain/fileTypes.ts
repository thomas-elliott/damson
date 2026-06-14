import {
  Box,
  Boxes,
  Circle,
  Clapperboard,
  Code,
  Database,
  File,
  FileText,
  Image,
  Music,
  Play,
  type LucideIcon,
} from "lucide-react";

/**
 * File-type taxonomy for the scoped file browser. `ftype` drives the icon
 * (via `FileGlyph`), the "File type" sidebar filter, and the preview style.
 * Pure data + functions — no components, so it stays a `.ts` module.
 */
export type FileType =
  | "prefab"
  | "mesh"
  | "texture"
  | "material"
  | "scene"
  | "script"
  | "shader"
  | "anim"
  | "audio"
  | "font"
  | "sprite"
  | "doc"
  | "data"
  | "file";

export interface FileTypeMeta {
  /** Plural label for the sidebar filter group, e.g. "Textures". */
  label: string;
  Icon: LucideIcon;
}

export const FILE_TYPE_META: Record<FileType, FileTypeMeta> = {
  prefab: { label: "Prefabs", Icon: Boxes },
  mesh: { label: "Meshes", Icon: Box },
  texture: { label: "Textures", Icon: Image },
  material: { label: "Materials", Icon: Circle },
  scene: { label: "Scenes", Icon: Clapperboard },
  script: { label: "Scripts", Icon: Code },
  shader: { label: "Shaders", Icon: Code },
  anim: { label: "Animation", Icon: Play },
  audio: { label: "Audio", Icon: Music },
  font: { label: "Fonts", Icon: FileText },
  sprite: { label: "Sprites", Icon: Image },
  doc: { label: "Docs", Icon: FileText },
  data: { label: "Data", Icon: Database },
  file: { label: "Files", Icon: File },
};

/** Type metadata with a generic-file fallback for unknown types. */
export function fileTypeMeta(ftype: FileType): FileTypeMeta {
  return FILE_TYPE_META[ftype] ?? FILE_TYPE_META.file;
}

/**
 * Extension → display type. Package-extracted files leave `detected_type`
 * empty, so the file extension is the source of truth for classification;
 * callers may pass a `detectedType` (loose uploads carry a MIME) to refine
 * the few cases the extension can't disambiguate (e.g. png sprite vs texture).
 */
const EXT_TO_TYPE: Record<string, FileType> = {
  // 3D / Unity
  prefab: "prefab",
  fbx: "mesh",
  obj: "mesh",
  blend: "mesh",
  dae: "mesh",
  mat: "material",
  unity: "scene",
  // Code
  cs: "script",
  js: "script",
  ts: "script",
  py: "script",
  shader: "shader",
  cginc: "shader",
  hlsl: "shader",
  compute: "shader",
  // Animation
  anim: "anim",
  controller: "anim",
  // Images
  png: "texture",
  jpg: "texture",
  jpeg: "texture",
  tga: "texture",
  psd: "texture",
  exr: "texture",
  tif: "texture",
  tiff: "texture",
  webp: "texture",
  gif: "texture",
  svg: "sprite",
  // Audio
  wav: "audio",
  mp3: "audio",
  ogg: "audio",
  aiff: "audio",
  flac: "audio",
  // Fonts
  ttf: "font",
  otf: "font",
  woff: "font",
  woff2: "font",
  // Docs
  pdf: "doc",
  md: "doc",
  txt: "doc",
  rtf: "doc",
  doc: "doc",
  docx: "doc",
  // Data
  json: "data",
  xml: "data",
  csv: "data",
  asset: "data",
  bytes: "data",
  dll: "data",
  zip: "data",
};

const IMAGE_MIME_PREFIX = "image/";

/** Lowercase extension including the dot (".png"), or "" when none. */
export function extOf(path: string): string {
  const base = path.slice(path.lastIndexOf("/") + 1);
  const dot = base.lastIndexOf(".");
  return dot > 0 ? base.slice(dot).toLowerCase() : "";
}

/** Classify a file by path, optionally refined by a detected MIME/type. */
export function fileTypeFromName(path: string, detectedType?: string): FileType {
  const ext = extOf(path).replace(".", "");
  const byExt = EXT_TO_TYPE[ext];
  if (byExt) return byExt;
  if (detectedType?.startsWith(IMAGE_MIME_PREFIX)) return "texture";
  return "file";
}

/** Whether a file type previews as an image (vs. a type placeholder). */
export function isImageType(ftype: FileType): boolean {
  return ftype === "texture" || ftype === "sprite";
}
