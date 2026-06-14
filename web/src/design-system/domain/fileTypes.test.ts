import { describe, expect, it } from "vitest";

import { extOf, fileTypeFromName } from "./fileTypes";

describe("extOf", () => {
  it("returns the lowercase extension including the dot", () => {
    expect(extOf("Assets/Foo.PNG")).toBe(".png");
    expect(extOf("a/b/Script.cs")).toBe(".cs");
  });
  it("returns empty string when there is no extension", () => {
    expect(extOf("README")).toBe("");
    expect(extOf("path/to/LICENSE")).toBe("");
  });
  it("ignores dots in folder names", () => {
    expect(extOf("a.b/c/file")).toBe("");
  });
  it("does not treat a leading-dot dotfile as an extension", () => {
    expect(extOf(".gitignore")).toBe("");
  });
});

describe("fileTypeFromName", () => {
  it("classifies common Unity/3D extensions", () => {
    expect(fileTypeFromName("a/SM_Castle.prefab")).toBe("prefab");
    expect(fileTypeFromName("Meshes/x.fbx")).toBe("mesh");
    expect(fileTypeFromName("T_Flame.png")).toBe("texture");
    expect(fileTypeFromName("M_Fire.mat")).toBe("material");
    expect(fileTypeFromName("Demo.unity")).toBe("scene");
    expect(fileTypeFromName("DayNight.cs")).toBe("script");
    expect(fileTypeFromName("Unlit.shader")).toBe("shader");
    expect(fileTypeFromName("Pulse.anim")).toBe("anim");
    expect(fileTypeFromName("hit.wav")).toBe("audio");
    expect(fileTypeFromName("Orbitron.ttf")).toBe("font");
  });
  it("falls back to a generic file for unknown extensions", () => {
    expect(fileTypeFromName("weird.zzz")).toBe("file");
    expect(fileTypeFromName("README")).toBe("file");
  });
  it("uses detected MIME only when the extension is unknown", () => {
    expect(fileTypeFromName("blob.bin", "image/png")).toBe("texture");
    // A known extension wins over the MIME hint.
    expect(fileTypeFromName("DayNight.cs", "image/png")).toBe("script");
  });
});
