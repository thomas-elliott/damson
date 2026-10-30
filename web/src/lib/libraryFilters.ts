import type { AssetFilters, AssetSort, LicenceStatus } from "@/hooks/useDataQueries";

export const LICENCE_STATUSES: { value: LicenceStatus; label: string }[] = [
  { value: "ok", label: "Compliant" },
  { value: "warn", label: "Review" },
  { value: "err", label: "Violation" },
  { value: "unk", label: "Unknown" },
];

export const PACKAGE_KINDS: { value: string; label: string }[] = [
  { value: "unitypackage", label: "Unity package" },
  { value: "loose", label: "Loose file" },
  { value: "zip", label: "Zip archive" },
];

// Boundaries resolve when clicked, so a long-lived session never pins a stale
// date.
export const RELEASED_AFTER_PRESETS: { years: number; label: string }[] = [
  { years: 1, label: "Last year" },
  { years: 2, label: "Last 2 years" },
  { years: 5, label: "Last 5 years" },
];

export function yearsAgoISODate(years: number): string {
  const d = new Date();
  d.setFullYear(d.getFullYear() - years);
  return d.toISOString().slice(0, 10);
}

const ASSET_SORTS: AssetSort[] = [
  "created_desc",
  "created_asc",
  "released_desc",
  "released_asc",
  "name_asc",
];

export function parseAssetFilters(search: URLSearchParams): AssetFilters {
  const f: AssetFilters = {};
  const q = search.get("q");
  if (q) f.q = q;
  const tags = [...new Set(search.getAll("tag").filter(Boolean))];
  if (tags.length > 0) f.tag_path = tags;
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

// The search without its page and page size: what a list shows, whichever
// page of it is on screen.
export function withoutPaging(search: URLSearchParams): string {
  const next = new URLSearchParams(search);
  next.delete("page");
  next.delete("page_size");
  return next.toString();
}

// Counts the Filters popover's filters. The search term has its own visible
// input and the tags their own button, so badging them would flag something
// already on screen.
export function activeFilterCount(filters: AssetFilters): number {
  return Object.entries(filters).filter(
    ([key, value]) => key !== "q" && key !== "sort" && key !== "tag_path" && value !== undefined,
  ).length;
}

// The URL params that filter the library, tags included; the search, the
// sort and the paging are not filters.
const NOT_FILTERS = new Set(["q", "sort", "page", "page_size"]);

// Returns a copy of `search` with every filter and tag dropped, back on the
// first page. The search term, sort and page size stay.
export function withoutFilters(search: URLSearchParams): URLSearchParams {
  const next = new URLSearchParams();
  for (const [key, value] of search) {
    if (NOT_FILTERS.has(key) && key !== "page") next.append(key, value);
  }
  return next;
}
