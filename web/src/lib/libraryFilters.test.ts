import { beforeEach, describe, expect, it } from "vitest";

import {
  FILTERS_OPEN_KEY,
  activeFilterCount,
  parseAssetFilters,
  readFiltersOpen,
  writeFiltersOpen,
} from "./libraryFilters";

const parse = (qs: string) => parseAssetFilters(new URLSearchParams(qs));

describe("parseAssetFilters", () => {
  it("reads every sidebar filter out of the query string", () => {
    expect(
      parse("q=castle&tag=env/props&licence=warn&spdx=MIT&kind=zip&needs_review=true"),
    ).toEqual({
      q: "castle",
      tag_path: "env/props",
      licence_status: "warn",
      licence_spdx: "MIT",
      package_kind: "zip",
      needs_review: true,
    });
  });

  it("carries project_id through as a number", () => {
    expect(parse("project_id=42").project_id).toBe(42);
  });

  it("drops a project_id that is not a positive integer", () => {
    for (const qs of ["project_id=0", "project_id=-3", "project_id=3.5", "project_id=abc"]) {
      expect(parse(qs).project_id, qs).toBeUndefined();
    }
  });

  it("omits absent and blank params rather than sending empty values", () => {
    expect(parse("")).toEqual({});
    expect(parse("q=&tag=&project_id=")).toEqual({});
  });

  it("treats needs_review as set only when explicitly true", () => {
    expect(parse("needs_review=false").needs_review).toBeUndefined();
  });
});

describe("activeFilterCount", () => {
  it("counts the sidebar filters", () => {
    expect(activeFilterCount(parse("tag=env&licence=err&project_id=7"))).toBe(3);
  });

  it("excludes the search term, which has its own visible input", () => {
    expect(activeFilterCount(parse("q=castle"))).toBe(0);
    expect(activeFilterCount(parse("q=castle&kind=zip"))).toBe(1);
  });

  it("is zero for an unfiltered library", () => {
    expect(activeFilterCount(parse(""))).toBe(0);
  });
});

describe("filter panel persistence", () => {
  beforeEach(() => localStorage.clear());

  it("defaults to closed when nothing is stored", () => {
    expect(readFiltersOpen()).toBe(false);
  });

  it("round-trips the open state", () => {
    writeFiltersOpen(true);
    expect(localStorage.getItem(FILTERS_OPEN_KEY)).toBe("true");
    expect(readFiltersOpen()).toBe(true);
    writeFiltersOpen(false);
    expect(readFiltersOpen()).toBe(false);
  });

  it("treats an unrecognised stored value as closed", () => {
    localStorage.setItem(FILTERS_OPEN_KEY, "1");
    expect(readFiltersOpen()).toBe(false);
  });
});

describe("parseAssetFilters — storefront metadata filters", () => {
  it("reads the publisher, store and released-after filters", () => {
    expect(parse("publisher=JustCreate&store=unity&released_after=2020-01-01")).toEqual({
      publisher: "JustCreate",
      store: "unity",
      released_after: "2020-01-01",
    });
  });

  it("accepts only known sort values", () => {
    expect(parse("sort=released_desc").sort).toBe("released_desc");
    expect(parse("sort=name_asc").sort).toBe("name_asc");
    // An unknown value is dropped rather than forwarded to the API, which
    // would have to reject it.
    for (const qs of ["sort=bogus", "sort=a.name;DROP TABLE assets", "sort="]) {
      expect(parse(qs).sort, qs).toBeUndefined();
    }
  });

  it("counts the new filters in the badge but not the sort", () => {
    expect(activeFilterCount(parse("publisher=JustCreate"))).toBe(1);
    expect(
      activeFilterCount(parse("publisher=JustCreate&store=unity&released_after=2020-01-01")),
    ).toBe(3);
    // The sort is a visible toolbar control, like the search box.
    expect(activeFilterCount(parse("sort=released_desc"))).toBe(0);
    expect(activeFilterCount(parse("sort=released_desc&store=unity"))).toBe(1);
  });
});
