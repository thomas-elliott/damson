import { describe, expect, it } from "vitest";

import { isRelevantListing, relevantListings } from "./listingRelevance";

describe("isRelevantListing", () => {
  it("drops listings that share none of the asset's words", () => {
    expect(isRelevantListing("Sample Terrain Textures", "Gaia Pro 2023")).toBe(false);
    expect(isRelevantListing("Sample Terrain Textures", "Behavior Designer")).toBe(false);
  });

  it("drops a listing that shares one word of several", () => {
    expect(
      isRelevantListing("Sample Terrain Textures", "Gaia Pro - Terrain & Scene Generator"),
    ).toBe(false);
  });

  it("keeps a listing that carries most of the name", () => {
    expect(isRelevantListing("POLYGON_Fantasy_Kingdom", "POLYGON - Fantasy Kingdom")).toBe(true);
    expect(isRelevantListing("Terrain Textures v2", "Terrain Textures Pack")).toBe(true);
  });

  it("drops a listing that is mostly other words", () => {
    expect(isRelevantListing("Sample Props", "Tavern and Inn Props 16px: FREE Sample")).toBe(false);
    expect(isRelevantListing("Fantasy Kingdom", "Fantasy Kingdom Castle Siege Battle Pack")).toBe(
      false,
    );
  });

  it("ignores filler words and version numbers", () => {
    expect(isRelevantListing("The Sample Pack 2", "Sample Pack")).toBe(false);
  });
});

describe("relevantListings", () => {
  it("keeps the store's order", () => {
    const got = relevantListings("Fantasy Kingdom", [
      { title: "Fantasy Kingdom Lite" },
      { title: "Behavior Designer" },
      { title: "Kingdom of Fantasy" },
    ]);
    expect(got.map((c) => c.title)).toEqual(["Fantasy Kingdom Lite", "Kingdom of Fantasy"]);
  });
});
