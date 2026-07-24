import { unzipSync } from "fflate";
import { describe, expect, it } from "vitest";

import {
  bundleFilesToZip,
  bundleName,
  hasFolderStructure,
  isArchiveFile,
  topFolderName,
} from "./bundleUpload";

function fileWithPath(path: string, data: string): File {
  const name = path.includes("/") ? path.slice(path.lastIndexOf("/") + 1) : path;
  return Object.assign(new File([data], name), { path });
}

describe("upload intent detection", () => {
  it("flags folder structure from a file path", () => {
    expect(hasFolderStructure([fileWithPath("kit/a.png", "a")])).toBe(true);
    expect(hasFolderStructure([fileWithPath("a.png", "a")])).toBe(false);
  });

  it("recognises archive extensions case-insensitively", () => {
    expect(isArchiveFile(new File([], "pack.zip"))).toBe(true);
    expect(isArchiveFile(new File([], "pack.UnityPackage"))).toBe(true);
    expect(isArchiveFile(new File([], "logo.png"))).toBe(false);
  });

  it("derives the bundle name from the top folder", () => {
    const files = [fileWithPath("MyKit/tex/a.png", "a"), fileWithPath("MyKit/b.fbx", "b")];
    expect(topFolderName(files)).toBe("MyKit");
    expect(bundleName(files)).toBe("MyKit.zip");
  });

  it("falls back to bundle.zip with no folder", () => {
    expect(bundleName([new File([], "a.png"), new File([], "b.png")])).toBe("bundle.zip");
  });
});

describe("bundleFilesToZip", () => {
  it("packs files into one zip preserving relative paths and bytes", async () => {
    const files = [
      fileWithPath("kit/readme.txt", "hello"),
      fileWithPath("kit/tex/a.png", "image-bytes"),
    ];
    const zipFile = await bundleFilesToZip(files, "kit.zip");
    expect(zipFile.name).toBe("kit.zip");

    const bytes = new Uint8Array(await zipFile.arrayBuffer());
    const entries = unzipSync(bytes);
    const names = Object.keys(entries).sort();
    expect(names).toEqual(["kit/readme.txt", "kit/tex/a.png"]);
    expect(new TextDecoder().decode(entries["kit/readme.txt"])).toBe("hello");
    expect(new TextDecoder().decode(entries["kit/tex/a.png"])).toBe("image-bytes");
  });
});
