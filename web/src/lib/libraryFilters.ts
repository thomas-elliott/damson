import type { AssetFilters, LicenceStatus } from "@/hooks/useDataQueries";

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
  return f;
}

export function activeFilterCount(filters: AssetFilters): number {
  return Object.entries(filters).filter(([key, value]) => key !== "q" && value !== undefined)
    .length;
}

export function readFiltersOpen(): boolean {
  if (typeof window === "undefined") return false;
  return window.localStorage.getItem(FILTERS_OPEN_KEY) === "true";
}

export function writeFiltersOpen(open: boolean): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(FILTERS_OPEN_KEY, String(open));
}
