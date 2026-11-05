import { describe, expect, it } from "vitest";

import {
  DEFAULT_PAGE_SIZE,
  activeFilterCount,
  parseAssetFilters,
  parsePaging,
  withPage,
  withoutFilters,
} from "./libraryFilters";

const parse = (qs: string) => parseAssetFilters(new URLSearchParams(qs));

describe("parseAssetFilters", () => {
  it("reads every filter out of the query string", () => {
    expect(
      parse("q=castle&tag=env/props&licence=warn&spdx=MIT&kind=zip&needs_review=true"),
    ).toEqual({
      q: "castle",
      tag_path: ["env/props"],
      licence_status: "warn",
      licence_spdx: "MIT",
      package_kind: "zip",
      needs_review: true,
    });
  });

  it("reads every ticked tag, once each", () => {
    expect(parse("tag=3d/props&tag=2d/ui&tag=3d/props").tag_path).toEqual(["3d/props", "2d/ui"]);
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
  it("counts the Filters popover's filters", () => {
    expect(activeFilterCount(parse("licence=err&project_id=7"))).toBe(2);
  });

  it("excludes the search term and the tags, which have their own controls", () => {
    expect(activeFilterCount(parse("q=castle"))).toBe(0);
    expect(activeFilterCount(parse("tag=3d/props&tag=2d/ui"))).toBe(0);
    expect(activeFilterCount(parse("q=castle&tag=3d/props&kind=zip"))).toBe(1);
  });

  it("is zero for an unfiltered library", () => {
    expect(activeFilterCount(parse(""))).toBe(0);
  });
});

describe("withoutFilters", () => {
  it("drops the filters and tags and goes back to the first page", () => {
    const next = withoutFilters(
      new URLSearchParams(
        "q=castle&tag=3d/props&tag=2d/ui&licence=err&sort=name_asc&page=3&page_size=100",
      ),
    );
    expect(next.toString()).toBe("q=castle&sort=name_asc&page_size=100");
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

  it("reads the updates-available filter only when explicitly true", () => {
    expect(parse("updates_available=true").updates_available).toBe(true);
    expect(parse("updates_available=false").updates_available).toBeUndefined();
    expect(parse("").updates_available).toBeUndefined();
  });

  it("reads the not-downloaded filter only when explicitly true", () => {
    expect(parse("not_downloaded=true").not_downloaded).toBe(true);
    expect(parse("not_downloaded=false").not_downloaded).toBeUndefined();
    expect(activeFilterCount(parse("not_downloaded=true"))).toBe(1);
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

describe("parsePaging", () => {
  const paging = (qs: string) => parsePaging(new URLSearchParams(qs));

  it("defaults to the first page at the default size", () => {
    expect(paging("")).toEqual({ page: 1, pageSize: DEFAULT_PAGE_SIZE });
  });

  it("reads the page and an offered page size", () => {
    expect(paging("page=3&page_size=100")).toEqual({ page: 3, pageSize: 100 });
  });

  it("falls back to the defaults for invalid values", () => {
    for (const qs of ["page=0", "page=-2", "page=1.5", "page=abc", "page="]) {
      expect(paging(qs).page, qs).toBe(1);
    }
    // 30 is inside the API's limit but not a size the pager offers.
    for (const qs of ["page_size=0", "page_size=500", "page_size=30", "page_size=x"]) {
      expect(paging(qs).pageSize, qs).toBe(DEFAULT_PAGE_SIZE);
    }
  });

  it("is not counted as a filter", () => {
    expect(activeFilterCount(parse("page=2&page_size=25"))).toBe(0);
  });
});

describe("withPage", () => {
  it("sets the page and keeps the other params", () => {
    expect(withPage(new URLSearchParams("q=castle"), 4).toString()).toBe("q=castle&page=4");
  });

  it("drops the param for page 1", () => {
    expect(withPage(new URLSearchParams("q=castle&page=4"), 1).toString()).toBe("q=castle");
  });
});
