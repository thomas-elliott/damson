// Next and previous on the package page step through the library results the
// asset was opened from. The results travel in the URL query (search, filters,
// sort, page), so a package page reloads and shares like the library page. At
// the edge of a page the step crosses into the neighbouring page, whose rows
// the caller fetches only while the asset sits on that edge.

export interface Neighbour {
  id: number;
  /** The library page the neighbour is on. */
  page: number;
}

export interface PackageNav {
  prev: Neighbour | null;
  next: Neighbour | null;
  /** One-based position in the whole result set; null when the asset isn't on the page. */
  position: number | null;
  total: number;
}

export interface NavInput {
  assetId: number;
  page: number;
  pageSize: number;
  total: number;
  /** Ids on the current page, or null while it loads. */
  current: number[] | null;
  /** Ids on the previous page, fetched only when the asset is first on its page. */
  before: number[] | null;
  /** Ids on the next page, fetched only when the asset is last on its page. */
  after: number[] | null;
}

/** Where the asset sits on its page, and so which neighbouring page is needed. */
export function edges(
  assetId: number,
  page: number,
  pageSize: number,
  total: number,
  current: number[] | null,
) {
  const i = current ? current.indexOf(assetId) : -1;
  return {
    index: i,
    needsBefore: i === 0 && page > 1,
    needsAfter: current != null && i >= 0 && i === current.length - 1 && page * pageSize < total,
  };
}

export function packageNav({
  assetId,
  page,
  pageSize,
  total,
  current,
  before,
  after,
}: NavInput): PackageNav {
  const { index, needsBefore, needsAfter } = edges(assetId, page, pageSize, total, current);
  if (!current || index < 0) return { prev: null, next: null, position: null, total };

  let prev: Neighbour | null = null;
  if (index > 0) prev = { id: current[index - 1], page };
  else if (needsBefore && before && before.length > 0)
    prev = { id: before[before.length - 1], page: page - 1 };

  let next: Neighbour | null = null;
  if (index < current.length - 1) next = { id: current[index + 1], page };
  else if (needsAfter && after && after.length > 0) next = { id: after[0], page: page + 1 };

  return { prev, next, position: (page - 1) * pageSize + index + 1, total };
}
