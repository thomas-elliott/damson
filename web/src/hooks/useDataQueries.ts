import { useEffect } from "react";
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query";

import { fetchJson, isAuthRedirectError } from "@/api";
import { fetchAuthMode, type AuthModeInfo } from "@/lib/authMode";
import { HOUR, MINUTE } from "@/lib/cachePolicy";
import { queryKeys } from "@/lib/queryKeys";

// A query that failed with nothing to show: a failed first load or page
// change. A failed refetch over data on screen is not one, and neither is an
// expired session, which the login redirect deals with.
function failedToLoad(q: UseQueryResult<unknown>): boolean {
  return q.isError && q.data === undefined && !isAuthRedirectError(q.error);
}

// -- /api/auth/mode ---------------------------------------------------------

export function useAuthMode() {
  const q = useQuery<AuthModeInfo>({
    queryKey: queryKeys.authMode,
    queryFn: fetchAuthMode,
    staleTime: Infinity, // doesn't change between page loads
    gcTime: Infinity,
  });
  return {
    mode: q.data?.mode ?? null,
    needsBootstrap: q.data?.needs_bootstrap ?? false,
    loading: q.isPending,
    error: q.error,
    reload: q.refetch,
  };
}

// -- /api/me ----------------------------------------------------------------

export interface MeLibrary {
  id: number;
  name: string;
  description: string;
  role: "admin" | "member";
}

export interface Me {
  id: number;
  email: string;
  display_name: string;
  role: "user" | "sysadmin";
  // True when the user hasn't set an email yet (the IdP sent no email claim at
  // first sign-in). The SPA shows the first-login profile step while this holds.
  needs_profile: boolean;
  current_library?: MeLibrary;
}

export function useMe(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.me,
    queryFn: () => fetchJson<Me>("/api/me"),
    staleTime: 5 * MINUTE,
    gcTime: 1 * HOUR,
    enabled,
  });
  return {
    data: q.data ?? null,
    loading: q.isPending,
    error: q.error,
    reload: q.refetch,
  };
}

// -- /api/assets ------------------------------------------------------------

export type LicenceStatus = "ok" | "warn" | "err" | "unk";

export interface AssetRow {
  id: number;
  name: string;
  file_count: number;
  size_bytes: number;
  needs_review: boolean;
  created_at: string;
  thumbnail_object_key?: string;
  licence_spdx?: string;
  licence_status?: LicenceStatus;
  package_kind?: string;
  publisher?: string;
  released_at?: string;
  update_available?: boolean;
  /** Recorded by its location with no files; size and file count mean nothing. */
  is_reference?: boolean;
}

interface AssetListResponse {
  assets: AssetRow[];
  total: number;
}

export interface AssetFilters {
  q?: string;
  tag_id?: number;
  /** Any of these tags, or anything under one. */
  tag_path?: string[];
  licence_status?: LicenceStatus;
  licence_spdx?: string;
  project_id?: number;
  collection_id?: number;
  source_host?: string;
  needs_review?: boolean;
  package_kind?: string;
  publisher?: string;
  store?: string;
  released_after?: string;
  updates_available?: boolean;
  // Purchased, not downloaded: a reference asset with no path.
  not_downloaded?: boolean;
  sort?: AssetSort;
}

export type AssetSort =
  | "created_desc"
  | "created_asc"
  | "released_desc"
  | "released_asc"
  | "name_asc"
  // Name matches first; only meaningful with q. The pickers use it, not the grid.
  | "relevance";

export interface AssetFacet {
  value: string;
  count: number;
}

export interface AssetFacetsView {
  publishers: AssetFacet[];
  stores: AssetFacet[];
  engines: AssetFacet[];
}

function filtersToQuery(f: AssetFilters | undefined): string {
  if (!f) return "";
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(f)) {
    if (v === undefined || v === "" || v === null) continue;
    if (Array.isArray(v)) for (const item of v) params.append(k, String(item));
    else params.set(k, String(v));
  }
  const s = params.toString();
  return s ? `?${s}` : "";
}

export interface AssetPage {
  limit: number;
  offset: number;
}

// Without `page` the API's default page applies. With it, the previous page
// stays on screen while the next one loads; `placeholder` says when that is
// what `data` and `total` hold.
export function useAssets(filters?: AssetFilters, enabled: boolean = true, page?: AssetPage) {
  const params = page ? { ...filters, ...page } : filters;
  const keyed =
    params && Object.keys(params).length > 0
      ? (Object.fromEntries(
          Object.entries(params).filter(([, v]) => v !== undefined && v !== ""),
        ) as Record<string, string | number | boolean | string[]>)
      : undefined;
  const q = useQuery({
    queryKey: queryKeys.assets(keyed),
    queryFn: () => fetchJson<AssetListResponse>(`/api/assets${filtersToQuery(params)}`),
    staleTime: 30 * 1000,
    gcTime: 10 * MINUTE,
    enabled,
    placeholderData: page ? keepPreviousData : undefined,
  });
  return {
    data: q.data?.assets ?? [],
    total: q.data?.total ?? 0,
    placeholder: q.isPlaceholderData,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/assets/:id --------------------------------------------------------

export interface AssetVersionView {
  id: number;
  version_label: string;
  store_version: string;
  notes: string;
  import_date: string;
  package_kind: string;
  file_count: number;
  size_bytes: number;
  has_original: boolean;
  /** Storage key of the retained original archive; present only when has_original. */
  original_package_object_key?: string;
  is_current: boolean;
}

export interface AssetFileView {
  id: number;
  relative_path: string;
  size_bytes: number;
  content_hash: string;
  object_key: string;
  detected_type?: string;
  /** Backend-rendered preview derivative, served from GET /api/previews/:key. */
  preview_object_key?: string;
  /** First-class preview state: unsupported | queued | processing | done | failed. */
  preview_status?: PreviewStatus;
}

export interface AssetTagView {
  id: number;
  path: string;
  parent_id?: number;
}

export interface LicenceAttributes {
  commercial_use: boolean | null;
  modification: boolean | null;
  redistribution: boolean | null;
  attribution_required: boolean | null;
  share_alike: boolean | null;
  open_source_required: boolean | null;
  custom_terms?: string | null;
}

export interface LicenceView {
  id: number;
  asset_id: number;
  detected_type: string;
  detection_confidence: number;
  canonical_diff?: string;
  original_text: string;
  attributes: LicenceAttributes;
  status: LicenceStatus;
  reviewed_at?: string;
}

export interface ProjectPinRef {
  project_id: number;
  project_name: string;
  /** Null for a reference asset, which has no versions. */
  pinned_version_id: number | null;
  version_label: string;
}

export interface CollectionRef {
  collection_id: number;
  collection_name: string;
}

export interface MetadataAttribute {
  key: string;
  label: string;
  value: string;
}

export interface MetadataView {
  title?: string;
  description?: string;
  publisher?: string;
  publisher_url?: string;
  price?: string;
  currency?: string;
  licence_hint?: string;
  cover_url?: string;
  image_urls?: string[];
  tags?: string[];
  category_path?: string;
  store?: string;
  listing_url?: string;
  released_at?: string;
  store_updated_at?: string;
  latest_store_version?: string;
  engine?: string;
  min_engine_version?: string;
  rating?: number;
  rating_count?: number;
  attributes?: MetadataAttribute[];
}

export interface MetadataCandidate {
  title: string;
  publisher?: string;
  price?: string;
  currency?: string;
  cover_url?: string;
  listing_url: string;
  store: string;
}

export interface SourceMetadataView {
  store?: string;
  listing_url?: string;
  fetched_at?: string;
  matched: boolean;
  match_confidence?: number;
  metadata?: MetadataView;
  candidates?: MetadataCandidate[];
}

export interface MetadataConflict {
  field: string;
  current: string;
  proposed: string;
}

export interface MetadataFetchResponse {
  store: string;
  listing_url: string;
  fetched: MetadataView | null;
  applied: string[];
  conflicts: MetadataConflict[];
}

export interface MetadataSearchResponse {
  query: string;
  candidates: MetadataCandidate[];
}

export interface AssetDetailView {
  id: number;
  name: string;
  description: string;
  source_url: string;
  needs_review: boolean;
  /** Recorded by its location (path and/or store link) with no versions or files. */
  is_reference: boolean;
  /** Where a reference asset lives on the NFS store. Damson never reads it. */
  location_path?: string;
  current_version_id?: number;
  thumbnail_object_key?: string;
  thumbnail_content_type?: string;
  created_at: string;
  updated_at: string;
  publisher?: string;
  publisher_url?: string;
  store?: string;
  store_product_id?: string;
  released_at?: string;
  store_updated_at?: string;
  latest_store_version?: string;
  engine?: string;
  min_engine_version?: string;
  price?: string;
  price_currency?: string;
  rating?: number;
  rating_count?: number;
  store_deprecated?: boolean;
  update_available?: boolean;
  licence?: LicenceView;
  source_metadata?: SourceMetadataView;
  tags: AssetTagView[];
  versions: AssetVersionView[];
  files: AssetFileView[];
  images: AssetImageView[];
  projects: ProjectPinRef[];
  collections: CollectionRef[];
}

export interface AssetImageView {
  id: number;
  object_key: string;
  content_type?: string;
}

export function useAssetDetail(id: number | null) {
  const q = useQuery({
    queryKey: id ? queryKeys.asset(id) : ["asset", "none"],
    queryFn: () => fetchJson<AssetDetailView>(`/api/assets/${id}`),
    staleTime: 30 * 1000,
    gcTime: 10 * MINUTE,
    enabled: id != null,
  });
  return {
    data: q.data ?? null,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/tags --------------------------------------------------------------

export interface TagView {
  id: number;
  path: string;
  parent_id?: number;
  /** Assets carrying this tag itself. */
  asset_count: number;
  /** Assets under the tag or any tag below it, once each: what filtering by it matches. */
  subtree_count: number;
}

export function useAssetFacets(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.assetFacets,
    queryFn: () => fetchJson<AssetFacetsView>("/api/assets/facets"),
    staleTime: 5 * MINUTE,
    gcTime: 1 * HOUR,
    enabled,
  });
  return {
    data: q.data ?? { publishers: [], stores: [], engines: [] },
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

export function useTags(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.tags,
    queryFn: () => fetchJson<{ tags: TagView[] }>("/api/tags"),
    staleTime: 5 * MINUTE,
    gcTime: 1 * HOUR,
    enabled,
  });
  return {
    data: q.data?.tags ?? [],
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

export interface TagIndexCoverView {
  id: number;
  name: string;
  publisher?: string;
  thumbnail_object_key?: string;
}

export interface TagIndexEntryView {
  /** A second-level tag, or a top-level one with nothing under it. */
  path: string;
  count: number;
  top_children: { name: string; count: number }[];
  covers: TagIndexCoverView[];
}

export function useTagIndex(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.tagIndex,
    queryFn: () => fetchJson<{ tags: TagIndexEntryView[] }>("/api/tags/index"),
    staleTime: 5 * MINUTE,
    gcTime: 1 * HOUR,
    enabled,
  });
  return {
    data: q.data?.tags ?? [],
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/projects ----------------------------------------------------------

export interface ProjectView {
  id: number;
  name: string;
  description: string;
  target_distribution: string;
  glyph: string;
  glyph_color: string;
  asset_count: number;
  updates_available: number;
  /** The pins' licence statuses, counted. */
  licence_counts: Record<LicenceStatus, number>;
  created_at: string;
  updated_at: string;
}

export interface ProjectPinDetail {
  pin_id: number;
  asset_id: number;
  asset_name: string;
  /** Null for a reference asset, which has no versions. */
  pinned_version_id: number | null;
  version_label: string;
  is_reference?: boolean;
  latest_version_id?: number;
  latest_label?: string;
  has_update: boolean;
  licence_status: LicenceStatus;
  licence_spdx?: string;
  size_bytes: number;
  pinned_at: string;
}

export interface ProjectDetailView extends ProjectView {
  pins: ProjectPinDetail[];
}

export function useProjects(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.projects,
    queryFn: () => fetchJson<{ projects: ProjectView[] }>("/api/projects"),
    staleTime: 30 * 1000,
    gcTime: 10 * MINUTE,
    enabled,
  });
  return {
    data: q.data?.projects ?? [],
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

export function useProject(id: number | null) {
  const q = useQuery({
    queryKey: id ? queryKeys.project(id) : ["project", "none"],
    queryFn: () => fetchJson<ProjectDetailView>(`/api/projects/${id}`),
    staleTime: 30 * 1000,
    gcTime: 10 * MINUTE,
    enabled: id != null,
  });
  return {
    data: q.data ?? null,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/collections -------------------------------------------------------

export interface CollectionView {
  id: number;
  name: string;
  description: string;
  asset_count: number;
  created_at: string;
  updated_at: string;
}

export function useCollections(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.collections,
    queryFn: () => fetchJson<{ collections: CollectionView[] }>("/api/collections"),
    staleTime: 30 * 1000,
    gcTime: 10 * MINUTE,
    enabled,
  });
  return {
    data: q.data?.collections ?? [],
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

export function useCollection(id: number | null) {
  const q = useQuery({
    queryKey: id ? queryKeys.collection(id) : ["collection", "none"],
    queryFn: () => fetchJson<CollectionView>(`/api/collections/${id}`),
    staleTime: 30 * 1000,
    gcTime: 10 * MINUTE,
    enabled: id != null,
  });
  return {
    data: q.data ?? null,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/settings ----------------------------------------------------------

export interface LibrarySettings {
  library_id: number;
  grid_density_default: "small" | "medium" | "large";
  retain_original_packages: boolean;
  thumbnails_raster_enabled: boolean;
  thumbnails_audio_enabled: boolean;
  thumbnails_font_enabled: boolean;
  thumbnails_unity_preview: boolean;
  auto_enrich_enabled: boolean;
  preferred_store: string;
  audit_retention_days: number | null;
}

export function useLibrarySettings(libraryId: number | undefined) {
  const q = useQuery({
    queryKey: queryKeys.librarySettings(libraryId),
    queryFn: () => fetchJson<LibrarySettings>("/api/settings"),
    staleTime: 5 * MINUTE,
    gcTime: 1 * HOUR,
    enabled: libraryId != null,
    // Grids wait for these and draw at medium without them, so a failure
    // settles at once. The settings page shows it with a Retry.
    retry: false,
  });
  return {
    data: q.data ?? null,
    loading: q.isPending,
    error: q.error,
    reload: q.refetch,
  };
}

// -- /api/audit -------------------------------------------------------------

export interface AuditEntry {
  id: number;
  library_id?: number;
  library_name?: string;
  user_id?: number;
  user_display_name?: string;
  user_email?: string;
  action_type: string;
  target_type: string;
  target_id?: number;
  /** The target's current name; absent when it is gone or has none. */
  target_name?: string;
  diff?: Record<string, unknown>;
  created_at: string;
}

export interface AuditFilters {
  user_id?: number;
  action_type?: string;
  target_type?: string;
  target_id?: number;
  since?: string;
  until?: string;
  /** Matches the user's name or email, or the action type, as a substring. */
  q?: string;
  limit?: number;
  offset?: number;
}

// Sysadmin audit feed scope. `server` is the original behaviour (events
// with library_id IS NULL); `all` merges every library-scoped event into
// the same view so a sysadmin can spot cross-library activity bursts.
export type AdminAuditScope = "server" | "all";

interface AuditListResponse {
  entries: AuditEntry[];
  total: number;
}

function auditQuery(f?: object): string {
  if (!f) return "";
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(f)) {
    if (v === undefined || v === "" || v === null) continue;
    if (Array.isArray(v)) for (const item of v) params.append(k, String(item));
    else params.set(k, String(v));
  }
  const s = params.toString();
  return s ? `?${s}` : "";
}

// The audit pages page through these two feeds: the previous page stays on
// screen while the next one loads, and `placeholder` says when that is what
// `data` and `total` hold.
export function useLibraryAudit(filters?: AuditFilters, enabled: boolean = true) {
  const keyed =
    filters && Object.keys(filters).length > 0
      ? (Object.fromEntries(
          Object.entries(filters).filter(([, v]) => v !== undefined && v !== ""),
        ) as Record<string, string | number | boolean>)
      : undefined;
  const q = useQuery({
    queryKey: queryKeys.audit(keyed),
    queryFn: () => fetchJson<AuditListResponse>(`/api/audit${auditQuery(filters)}`),
    staleTime: 30 * 1000,
    gcTime: 5 * MINUTE,
    enabled,
    placeholderData: keepPreviousData,
  });
  return {
    data: q.data?.entries ?? [],
    total: q.data?.total ?? 0,
    placeholder: q.isPlaceholderData,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

export function useAdminAudit(
  filters?: AuditFilters & { scope?: AdminAuditScope },
  enabled: boolean = true,
) {
  const keyed =
    filters && Object.keys(filters).length > 0
      ? (Object.fromEntries(
          Object.entries(filters).filter(([, v]) => v !== undefined && v !== ""),
        ) as Record<string, string | number | boolean>)
      : undefined;
  const q = useQuery({
    queryKey: queryKeys.adminAudit(keyed),
    queryFn: () => fetchJson<AuditListResponse>(`/api/admin/audit${auditQuery(filters)}`),
    staleTime: 30 * 1000,
    gcTime: 5 * MINUTE,
    enabled,
    placeholderData: keepPreviousData,
  });
  return {
    data: q.data?.entries ?? [],
    total: q.data?.total ?? 0,
    placeholder: q.isPlaceholderData,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

export function useAssetAudit(assetId: number | null) {
  const q = useQuery({
    queryKey: assetId ? queryKeys.assetAudit(assetId) : ["asset", "none", "audit"],
    queryFn: () => fetchJson<AuditListResponse>(`/api/assets/${assetId}/audit`),
    staleTime: 30 * 1000,
    gcTime: 5 * MINUTE,
    enabled: assetId != null,
  });
  return {
    data: q.data?.entries ?? [],
    total: q.data?.total ?? 0,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

export function useProjectAudit(projectId: number | null) {
  const q = useQuery({
    queryKey: projectId ? queryKeys.projectAudit(projectId) : ["project", "none", "audit"],
    queryFn: () => fetchJson<AuditListResponse>(`/api/projects/${projectId}/audit`),
    staleTime: 30 * 1000,
    gcTime: 5 * MINUTE,
    enabled: projectId != null,
  });
  return {
    data: q.data?.entries ?? [],
    total: q.data?.total ?? 0,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/triage ------------------------------------------------------------

export type ImportStage =
  "uploaded" | "extracting" | "detecting" | "thumbnailing" | "done" | "failed";

// TriageRow is one asset in the triage queue. `import` is set only while the
// asset's latest import is in flight, or failed and not dismissed.
export interface TriageRow {
  asset_id: number;
  asset_name: string;
  needs_review: boolean;
  is_reference: boolean;
  description?: string;
  thumbnail_object_key?: string;
  matched: boolean;
  import?: { id: number; stage: ImportStage; error?: string };
  updated_at: string;
}

interface TriageResponse {
  assets: TriageRow[];
  reviewed: TriageRow[];
  total: number;
}

export const TRIAGE_PAGE = 50;

// useTriage reads one page of the triage queue (1-based). The rail's badge
// reads page 1 for its total, so it shares the page's cache.
export function useTriage(page: number = 1) {
  const offset = (page - 1) * TRIAGE_PAGE;
  const q = useQuery({
    queryKey: queryKeys.triage(page),
    queryFn: () => fetchJson<TriageResponse>(`/api/triage?limit=${TRIAGE_PAGE}&offset=${offset}`),
    staleTime: 15 * 1000,
    gcTime: 5 * MINUTE,
    placeholderData: keepPreviousData,
  });
  return {
    data: q.data?.assets ?? [],
    reviewed: q.data?.reviewed ?? [],
    total: q.data?.total ?? 0,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/store-lookups/batches/:id ---------------------------------------

export interface StoreLookupBatchView {
  batch_id: number;
  total: number;
  queued: number;
  done: number;
  failed: number;
}

// How often a batch with lookups still queued is asked about. The server
// spaces lookups to one store a couple of seconds apart, so a record's lookup
// lands within a few polls.
const LOOKUP_POLL_MS = 2000;

// useStoreLookupBatch follows a batch of background store lookups until none
// is queued, then refetches the assets, so what the lookups filled in shows.
export function useStoreLookupBatch(batchId: number | null) {
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: batchId ? queryKeys.storeLookupBatch(batchId) : ["storeLookupBatch", "none"],
    queryFn: () => fetchJson<StoreLookupBatchView>(`/api/store-lookups/batches/${batchId}`),
    enabled: batchId != null,
    refetchInterval: (query) => (query.state.data?.queued === 0 ? false : LOOKUP_POLL_MS),
    gcTime: 5 * MINUTE,
  });
  const finishedAt = q.data?.queued === 0 ? q.dataUpdatedAt : 0;
  useEffect(() => {
    if (!finishedAt) return;
    qc.invalidateQueries({ queryKey: queryKeys.assets() });
    qc.invalidateQueries({ queryKey: queryKeys.anyAsset });
    qc.invalidateQueries({ queryKey: queryKeys.assetFacets });
    qc.invalidateQueries({ queryKey: queryKeys.tags });
  }, [finishedAt, qc]);
  return {
    data: q.data ?? null,
    loading: q.isPending,
    error: q.error,
    reload: q.refetch,
  };
}

// -- /api/previews (processing queue) --------------------------------------

export type PreviewStatus = "unsupported" | "queued" | "processing" | "done" | "failed";

/** Per-version rollup of its files' preview statuses — one processing-queue row. */
export interface PreviewRow {
  version_id: number;
  asset_id: number;
  asset_name: string;
  status: "processing" | "queued" | "done";
  total: number;
  done: number;
  failed: number;
  processing: number;
  queued: number;
  unsupported: number;
  created_at: string;
  updated_at: string;
}

interface PreviewListResponse {
  previews: PreviewRow[];
  finished?: PreviewRow[];
  total: number;
}

export function usePreviewQueue(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.previews,
    queryFn: () => fetchJson<PreviewListResponse>("/api/previews"),
    staleTime: 15 * 1000,
    gcTime: 5 * MINUTE,
    enabled,
  });
  return {
    data: q.data?.previews ?? [],
    finished: q.data?.finished ?? [],
    total: q.data?.total ?? 0,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/libraries/:id/members --------------------------------------------

export interface MemberView {
  user_id: number;
  email: string;
  display_name: string;
  role: "admin" | "member";
  global_role: "user" | "sysadmin";
}

export function useLibraryMembers(libraryId: number | null) {
  const q = useQuery({
    queryKey: libraryId ? queryKeys.libraryMembers(libraryId) : ["libraries", "none", "members"],
    queryFn: () => fetchJson<{ members: MemberView[] }>(`/api/libraries/${libraryId}/members`),
    staleTime: 30 * 1000,
    gcTime: 5 * MINUTE,
    enabled: libraryId != null,
  });
  return {
    data: q.data?.members ?? [],
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/admin/users -------------------------------------------------------

export interface AdminUser {
  id: number;
  email: string;
  display_name: string;
  role: "user" | "sysadmin";
}

export function useAdminUsers(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.adminUsers,
    queryFn: () => fetchJson<{ users: AdminUser[] }>(`/api/admin/users`),
    staleTime: 30 * 1000,
    gcTime: 5 * MINUTE,
    enabled,
  });
  return {
    data: q.data?.users ?? [],
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/admin/settings ----------------------------------------------------

export interface ServerSettings {
  max_upload_bytes: number;
}

export function useServerSettings(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.adminSettings,
    queryFn: () => fetchJson<ServerSettings>(`/api/admin/settings`),
    staleTime: 5 * MINUTE,
    gcTime: 1 * HOUR,
    enabled,
  });
  return {
    data: q.data ?? null,
    loading: q.isPending,
    error: q.error,
    failed: failedToLoad(q),
    reload: q.refetch,
  };
}

// -- /api/me/tokens ---------------------------------------------------------

export interface ApiToken {
  id: number;
  label: string;
  prefix: string;
  scope: "read" | "write";
  library_id: number;
  library_name?: string;
  created_at: string;
  last_used_at?: string | null;
  expires_at?: string | null;
  revoked_at?: string | null;
}

export function useApiTokens(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.apiTokens,
    queryFn: () => fetchJson<{ tokens: ApiToken[] }>(`/api/me/tokens`),
    staleTime: 30 * 1000,
    gcTime: 5 * MINUTE,
    enabled,
  });
  return {
    data: q.data?.tokens ?? [],
    loading: q.isPending,
    error: q.error,
    reload: q.refetch,
  };
}
