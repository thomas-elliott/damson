import { describe, expect, it } from "vitest";

import type { FileNode, FileType } from "@/design-system";

import { previewImageMime } from "./useFileObjectUrl";

function node(ext: string, ftype: FileType): FileNode {
  return { type: "file", name: `file${ext}`, ext, ftype, size: 1 };
}

describe("previewImageMime", () => {
  it("maps inline-previewable raster images to their MIME", () => {
    expect(previewImageMime(node(".png", "texture"))).toBe("image/png");
    expect(previewImageMime(node(".jpg", "texture"))).toBe("image/jpeg");
    expect(previewImageMime(node(".jpeg", "texture"))).toBe("image/jpeg");
    expect(previewImageMime(node(".gif", "texture"))).toBe("image/gif");
    expect(previewImageMime(node(".webp", "texture"))).toBe("image/webp");
  });

  it("returns undefined for non-image file types", () => {
    expect(previewImageMime(node(".cs", "script"))).toBeUndefined();
    expect(previewImageMime(node(".wav", "audio"))).toBeUndefined();
  });

  it("excludes SVG and pro/HDR images (handled elsewhere, not inline raster)", () => {
    // SVG is deliberately not fetched as an inline blob (active-content risk);
    // TGA/PSD/EXR are image-typed but only previewable via a backend derivative.
    expect(previewImageMime(node(".svg", "sprite"))).toBeUndefined();
    expect(previewImageMime(node(".tga", "texture"))).toBeUndefined();
    expect(previewImageMime(node(".psd", "texture"))).toBeUndefined();
  });

  it("returns undefined for null/undefined nodes", () => {
    expect(previewImageMime(null)).toBeUndefined();
    expect(previewImageMime(undefined)).toBeUndefined();
  });
});
