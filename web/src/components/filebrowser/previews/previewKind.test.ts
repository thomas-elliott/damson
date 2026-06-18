import { describe, expect, it } from "vitest";

import type { FileNode, FileType } from "@/design-system";

import { audioMime, isFontFile, previewKind, shikiLang } from "./previewKind";

function node(ext: string, ftype: FileType): FileNode {
  return { type: "file", name: `file${ext}`, ext, ftype, size: 1, objectKey: "k" };
}

describe("shikiLang", () => {
  it("maps code and shader extensions to grammars", () => {
    expect(shikiLang(node(".cs", "script"))).toBe("csharp");
    expect(shikiLang(node(".ts", "script"))).toBe("typescript");
    expect(shikiLang(node(".py", "script"))).toBe("python");
    expect(shikiLang(node(".hlsl", "shader"))).toBe("hlsl");
    expect(shikiLang(node(".cginc", "shader"))).toBe("hlsl");
    expect(shikiLang(node(".shader", "shader"))).toBe("shaderlab");
  });

  it("treats Unity serialized assets as YAML", () => {
    expect(shikiLang(node(".prefab", "prefab"))).toBe("yaml");
    expect(shikiLang(node(".unity", "scene"))).toBe("yaml");
    expect(shikiLang(node(".mat", "material"))).toBe("yaml");
    expect(shikiLang(node(".anim", "anim"))).toBe("yaml");
  });

  it("maps data text formats and plaintext", () => {
    expect(shikiLang(node(".json", "data"))).toBe("json");
    expect(shikiLang(node(".xml", "data"))).toBe("xml");
    expect(shikiLang(node(".txt", "doc"))).toBe("text");
  });

  it("returns undefined for binary / non-text files", () => {
    expect(shikiLang(node(".png", "texture"))).toBeUndefined();
    expect(shikiLang(node(".fbx", "mesh"))).toBeUndefined();
    expect(shikiLang(null)).toBeUndefined();
  });
});

describe("audioMime", () => {
  it("maps native-codec audio to a MIME, excludes AIFF", () => {
    expect(audioMime(node(".wav", "audio"))).toBe("audio/wav");
    expect(audioMime(node(".mp3", "audio"))).toBe("audio/mpeg");
    expect(audioMime(node(".flac", "audio"))).toBe("audio/flac");
    expect(audioMime(node(".aiff", "audio"))).toBeUndefined();
    expect(audioMime(node(".aif", "audio"))).toBeUndefined();
  });
});

describe("isFontFile", () => {
  it("recognises sfnt/web font extensions", () => {
    expect(isFontFile(node(".ttf", "font"))).toBe(true);
    expect(isFontFile(node(".woff2", "font"))).toBe(true);
    expect(isFontFile(node(".png", "texture"))).toBe(false);
  });
});

describe("previewKind", () => {
  it("routes each type to its viewer", () => {
    expect(previewKind(node(".cs", "script"))).toBe("text");
    expect(previewKind(node(".yaml", "data"))).toBe("text");
    expect(previewKind(node(".mp3", "audio"))).toBe("audio");
    expect(previewKind(node(".otf", "font"))).toBe("font");
    expect(previewKind(node(".zip", "data"))).toBe("archive");
    expect(previewKind(node(".png", "texture"))).toBe("image");
    expect(previewKind(node(".fbx", "mesh"))).toBe("fallback");
    expect(previewKind(node(".svg", "sprite"))).toBe("fallback"); // SVG not inline yet
    expect(previewKind(node(".aiff", "audio"))).toBe("fallback"); // AIFF → download-only
    expect(previewKind(null)).toBe("fallback");
  });
});
