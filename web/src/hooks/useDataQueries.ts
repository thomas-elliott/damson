import { useQuery } from "@tanstack/react-query";

import { fetchJson } from "@/api";
import { fetchAuthMode, type AuthModeInfo } from "@/lib/authMode";
import { HOUR, MINUTE } from "@/lib/cachePolicy";
import { queryKeys } from "@/lib/queryKeys";

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
  };
}

// -- /api/me ----------------------------------------------------------------

export interface MeLibrary {
  id: number;
  name: string;
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

// -- /api/libraries ---------------------------------------------------------

export interface LibrarySummary {
  library_id: number;
  name: string;
  description?: string;
  role: "admin" | "member";
  is_current: boolean;
}

export function useLibraries(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.libraries,
    queryFn: () => fetchJson<{ libraries: LibrarySummary[] }>("/api/libraries"),
    staleTime: 5 * MINUTE,
    gcTime: 1 * HOUR,
    enabled,
  });
  return {
    data: q.data?.libraries ?? [],
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
}

interface AssetListResponse {
  assets: AssetRow[];
  total: number;
}

export interface AssetFilters {
  q?: string;
  tag_id?: number;
  tag_path?: string;
  licence_status?: LicenceStatus;
  licence_spdx?: string;
  project_id?: number;
  collection_id?: number;
  source_host?: string;
  needs_review?: boolean;
  package_kind?: string;
}

function filtersToQuery(f: AssetFilters | undefined): string {
  if (!f) return "";
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(f)) {
    if (v === undefined || v === "" || v === null) continue;
    params.set(k, String(v));
  }
  const s = params.toString();
  return s ? `?${s}` : "";
}

export function useAssets(filters?: AssetFilters, enabled: boolean = true) {
  const keyed =
    filters && Object.keys(filters).length > 0
      ? (Object.fromEntries(
          Object.entries(filters).filter(([, v]) => v !== undefined && v !== ""),
        ) as Record<string, string | number | boolean>)
      : undefined;
  const q = useQuery({
    queryKey: queryKeys.assets(keyed),
    queryFn: () => fetchJson<AssetListResponse>(`/api/assets${filtersToQuery(filters)}`),
    staleTime: 30 * 1000,
    gcTime: 10 * MINUTE,
    enabled,
  });
  return {
    data: q.data?.assets ?? [],
    total: q.data?.total ?? 0,
    loading: q.isPending,
    error: q.error,
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
  pinned_version_id: number;
  version_label: string;
}

export interface CollectionRef {
  collection_id: number;
  collection_name: string;
}

export interface MetadataView {
  title?: string;
  description?: string;
  publisher?: string;
  price?: string;
  currency?: string;
  licence_hint?: string;
  cover_url?: string;
  image_urls?: string[];
  store?: string;
  listing_url?: string;
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
  current_version_id?: number;
  thumbnail_object_key?: string;
  thumbnail_content_type?: string;
  created_at: string;
  updated_at: string;
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
    reload: q.refetch,
  };
}

// -- /api/tags --------------------------------------------------------------

export interface TagView {
  id: number;
  path: string;
  parent_id?: number;
  asset_count: number;
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
  created_at: string;
  updated_at: string;
}

export interface ProjectPinDetail {
  pin_id: number;
  asset_id: number;
  asset_name: string;
  pinned_version_id: number;
  version_label: string;
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
    reload: q.refetch,
  };
}

// -- /api/settings ----------------------------------------------------------

export interface LibrarySettings {
  library_id: number;
  theme_default: "dark" | "light";
  grid_density_default: "small" | "medium" | "large";
  retain_original_packages: boolean;
  dedupe_enabled: boolean;
  thumbnails_raster_enabled: boolean;
  thumbnails_audio_enabled: boolean;
  thumbnails_font_enabled: boolean;
  thumbnails_unity_preview: boolean;
  auto_enrich_enabled: boolean;
  preferred_store: string;
  audit_retention_days: number | null;
}

export function useLibrarySettings(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.librarySettings,
    queryFn: () => fetchJson<LibrarySettings>("/api/settings"),
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
    params.set(k, String(v));
  }
  const s = params.toString();
  return s ? `?${s}` : "";
}

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
  });
  return {
    data: q.data?.entries ?? [],
    total: q.data?.total ?? 0,
    loading: q.isPending,
    error: q.error,
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
  });
  return {
    data: q.data?.entries ?? [],
    total: q.data?.total ?? 0,
    loading: q.isPending,
    error: q.error,
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
    reload: q.refetch,
  };
}

// -- /api/imports -----------------------------------------------------------

export interface ImportRow {
  id: number;
  asset_id: number;
  asset_name: string;
  stage: "uploaded" | "extracting" | "detecting" | "thumbnailing" | "done" | "failed";
  error?: string;
  updated_at: string;
  created_at: string;
  needs_review: boolean;
}

interface ImportListResponse {
  imports: ImportRow[];
  finished?: ImportRow[];
  total: number;
}

export function useImports(
  stage: "in_flight" | "all" | "done" | "failed" = "in_flight",
  enabled: boolean = true,
) {
  const search = stage === "in_flight" ? "" : `?stage=${stage}`;
  const q = useQuery({
    queryKey: [...queryKeys.imports, stage],
    queryFn: () => fetchJson<ImportListResponse>(`/api/imports${search}`),
    staleTime: 15 * 1000,
    gcTime: 5 * MINUTE,
    enabled,
  });
  return {
    data: q.data?.imports ?? [],
    finished: q.data?.finished ?? [],
    total: q.data?.total ?? 0,
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
    reload: q.refetch,
  };
}

// -- /api/admin/settings ----------------------------------------------------

export interface ServerSettings {
  max_upload_bytes: number;
  licence_kb_auto_update: boolean;
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
    reload: q.refetch,
  };
}
