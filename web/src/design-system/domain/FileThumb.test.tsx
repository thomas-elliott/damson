import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";

import { FileThumb } from "./FileThumb";
import type { FileNode } from "./fileTree";
import type { FileType } from "./fileTypes";

function node(ftype: FileType, ext = ".bin"): FileNode {
  return { type: "file", name: `file${ext}`, ext, ftype, size: 1 };
}

describe("FileThumb", () => {
  it("renders a resolved imageUrl directly, even for a non-image file type", () => {
    // A backend derivative resolves to an object URL the caller already deemed
    // renderable, so the isImageType gate must not block it.
    const { container } = render(<FileThumb node={node("audio")} imageUrl="blob:thumb-9" />);
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img).toHaveAttribute("src", "blob:thumb-9");
  });

  it("falls back to a placeholder (no <img>) when no imageUrl is given", () => {
    const { container } = render(<FileThumb node={node("script", ".cs")} />);
    expect(container.querySelector("img")).toBeNull();
  });
});
