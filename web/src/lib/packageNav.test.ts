import { describe, expect, it } from "vitest";

import { edges, packageNav } from "./packageNav";

const base = { pageSize: 3, total: 8, before: null, after: null };

describe("packageNav", () => {
  it("steps within a page", () => {
    expect(packageNav({ ...base, assetId: 5, page: 2, current: [4, 5, 6] })).toEqual({
      prev: { id: 4, page: 2 },
      next: { id: 6, page: 2 },
      position: 5,
      total: 8,
    });
  });

  it("crosses into the previous page from the first row", () => {
    const nav = packageNav({ ...base, assetId: 4, page: 2, current: [4, 5, 6], before: [1, 2, 3] });
    expect(nav.prev).toEqual({ id: 3, page: 1 });
    expect(nav.position).toBe(4);
  });

  it("crosses into the next page from the last row", () => {
    const nav = packageNav({ ...base, assetId: 6, page: 2, current: [4, 5, 6], after: [7, 8] });
    expect(nav.next).toEqual({ id: 7, page: 3 });
  });

  it("stops at the ends of the results", () => {
    const first = packageNav({ ...base, assetId: 1, page: 1, current: [1, 2, 3] });
    expect(first.prev).toBeNull();
    const last = packageNav({ ...base, assetId: 8, page: 3, current: [7, 8] });
    expect(last.next).toBeNull();
  });

  it("has no neighbours for an asset that isn't in the results", () => {
    expect(packageNav({ ...base, assetId: 99, page: 1, current: [1, 2, 3] })).toEqual({
      prev: null,
      next: null,
      position: null,
      total: 8,
    });
  });

  it("waits for the neighbouring page before offering a step across", () => {
    const nav = packageNav({ ...base, assetId: 6, page: 2, current: [4, 5, 6] });
    expect(nav.next).toBeNull();
  });
});

describe("edges", () => {
  it("asks for a neighbouring page only on its edge", () => {
    expect(edges(4, 2, 3, 8, [4, 5, 6])).toMatchObject({ needsBefore: true, needsAfter: false });
    expect(edges(6, 2, 3, 8, [4, 5, 6])).toMatchObject({ needsBefore: false, needsAfter: true });
    expect(edges(5, 2, 3, 8, [4, 5, 6])).toMatchObject({ needsBefore: false, needsAfter: false });
    expect(edges(8, 3, 3, 8, [7, 8])).toMatchObject({ needsAfter: false });
  });
});
