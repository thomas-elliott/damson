import { describe, expect, it } from "vitest";

import type { FileNode, FileType } from "@/design-system";

import { previewKind, shikiLang } from "./previewKind";

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

describe("previewKind", () => {
  it("prefers text, then image, then fallback", () => {
    expect(previewKind(node(".cs", "script"))).toBe("text");
    expect(previewKind(node(".yaml", "data"))).toBe("text");
    expect(previewKind(node(".png", "texture"))).toBe("image");
    expect(previewKind(node(".fbx", "mesh"))).toBe("fallback");
    expect(previewKind(node(".svg", "sprite"))).toBe("fallback"); // SVG not inline yet
    expect(previewKind(null)).toBe("fallback");
  });
});
