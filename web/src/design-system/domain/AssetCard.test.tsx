import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";

import { AssetCard } from "./AssetCard";

describe("AssetCard thumbnail", () => {
  it("renders the real thumbnail image when imageUrl is set", () => {
    const { container } = render(<AssetCard name="pack" imageUrl="blob:thumb-123" />);
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img).toHaveAttribute("src", "blob:thumb-123");
  });

  it("falls back to the procedural placeholder (no <img>) when imageUrl is absent", () => {
    const { container } = render(<AssetCard name="pack" />);
    expect(container.querySelector("img")).toBeNull();
  });

  // Buttons shrink-to-fit in Chrome even at display:flex, so without w-full a
  // card sizes to its cover image's natural width and the grid loses its
  // equal columns.
  it("fills its grid cell rather than sizing to the image", () => {
    const { container } = render(<AssetCard name="pack" imageUrl="blob:thumb-123" />);
    const button = container.querySelector("button");
    expect(button?.className).toContain("w-full");
    expect(button?.className).toContain("min-w-0");
  });
});
