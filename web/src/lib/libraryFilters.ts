import type { AssetFilters, AssetSort, LicenceStatus } from "@/hooks/useDataQueries";

const ASSET_SORTS: AssetSort[] = [
  "created_desc",
  "created_asc",
  "released_desc",
  "released_asc",
  "name_asc",
];

export const FILTERS_OPEN_KEY = "damson.library.filters";

export function parseAssetFilters(search: URLSearchParams): AssetFilters {
  const f: AssetFilters = {};
  const q = search.get("q");
  if (q) f.q = q;
  const tag = search.get("tag");
  if (tag) f.tag_path = tag;
  const lic = search.get("licence");
  if (lic) f.licence_status = lic as LicenceStatus;
  const spdx = search.get("spdx");
  if (spdx) f.licence_spdx = spdx;
  const kind = search.get("kind");
  if (kind) f.package_kind = kind;
  const project = Number(search.get("project_id"));
  if (Number.isInteger(project) && project > 0) f.project_id = project;
  const needsReview = search.get("needs_review");
  if (needsReview === "true") f.needs_review = true;
  const publisher = search.get("publisher");
  if (publisher) f.publisher = publisher;
  const store = search.get("store");
  if (store) f.store = store;
  const releasedAfter = search.get("released_after");
  if (releasedAfter) f.released_after = releasedAfter;
  const updates = search.get("updates_available");
  if (updates === "true") f.updates_available = true;
  if (search.get("not_downloaded") === "true") f.not_downloaded = true;
  const sort = search.get("sort");
  if (sort && (ASSET_SORTS as string[]).includes(sort)) f.sort = sort as AssetSort;
  return f;
}

// The page sizes the pager offers, all inside the API's 1-200 limit.
export const PAGE_SIZES = [25, 50, 100, 200] as const;
export const DEFAULT_PAGE_SIZE = 50;

export interface LibraryPaging {
  /** 1-based. Not clamped to the last page: only the caller knows the total. */
  page: number;
  pageSize: number;
}

// Reads `page` and `page_size`; anything that isn't a positive integer, or a
// page size the pager doesn't offer, falls back to the default.
export function parsePaging(search: URLSearchParams): LibraryPaging {
  const page = Number(search.get("page"));
  const size = Number(search.get("page_size"));
  return {
    page: Number.isInteger(page) && page > 0 ? page : 1,
    pageSize: (PAGE_SIZES as readonly number[]).includes(size) ? size : DEFAULT_PAGE_SIZE,
  };
}

// Returns a copy of `search` showing `page`, leaving the param off for page 1
// so the default view keeps a clean URL.
export function withPage(search: URLSearchParams, page: number): URLSearchParams {
  const next = new URLSearchParams(search);
  if (page > 1) next.set("page", String(page));
  else next.delete("page");
  return next;
}

// Counts the sidebar filters only: the search term has its own visible input,
// so badging it would flag something already on screen.
export function activeFilterCount(filters: AssetFilters): number {
  return Object.entries(filters).filter(
    ([key, value]) => key !== "q" && key !== "sort" && value !== undefined,
  ).length;
}

export function readFiltersOpen(): boolean {
  if (typeof window === "undefined") return false;
  return window.localStorage.getItem(FILTERS_OPEN_KEY) === "true";
}

export function writeFiltersOpen(open: boolean): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(FILTERS_OPEN_KEY, String(open));
}
