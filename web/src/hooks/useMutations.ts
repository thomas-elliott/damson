import { useMutation, useQueryClient } from "@tanstack/react-query";

import { fetchJson, fetchMaybeJson } from "@/api";
import type { Me } from "@/hooks/useDataQueries";
import { queryKeys } from "@/lib/queryKeys";

// usePatchMe writes the current user's own profile (email + optional display
// name). Used by the first-login profile step. The fresh Me is seeded straight
// into the query cache so the profile gate falls through immediately.
export function usePatchMe() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { email: string; display_name?: string }) =>
      fetchJson<Me>("/api/me", {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: (me) => {
      qc.setQueryData(queryKeys.me, me);
    },
  });
}

export function usePatchAsset() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: {
      id: number;
      name?: string;
      description?: string;
      source_url?: string;
    }) => {
      const { id, ...body } = input;
      return fetchJson<{ id: number; name: string }>(`/api/assets/${id}`, {
        method: "PATCH",
        body: JSON.stringify(body),
      });
    },
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: queryKeys.asset(v.id) });
      qc.invalidateQueries({ queryKey: queryKeys.assets() });
    },
  });
}

export function useDeleteAsset() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: number) => {
      await fetchMaybeJson(`/api/assets/${id}`, { method: "DELETE" });
      return id;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.assets() });
    },
  });
}

export function useReviewAsset() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: number) => {
      await fetchMaybeJson(`/api/assets/${id}/review`, { method: "POST" });
      return id;
    },
    onSuccess: (id) => {
      qc.invalidateQueries({ queryKey: queryKeys.asset(id) });
      qc.invalidateQueries({ queryKey: queryKeys.assets() });
    },
  });
}

export function useAddAssetTag() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { assetId: number; path: string }) => {
      return fetchJson<{ asset_id: number; tag_id: number }>(`/api/assets/${input.assetId}/tags`, {
        method: "POST",
        body: JSON.stringify({ path: input.path }),
      });
    },
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: queryKeys.asset(v.assetId) });
      qc.invalidateQueries({ queryKey: queryKeys.tags });
    },
  });
}

export function useRemoveAssetTag() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { assetId: number; tagId: number }) => {
      await fetchMaybeJson(`/api/assets/${input.assetId}/tags/${input.tagId}`, {
        method: "DELETE",
      });
      return input;
    },
    onSuccess: (v) => {
      qc.invalidateQueries({ queryKey: queryKeys.asset(v.assetId) });
      qc.invalidateQueries({ queryKey: queryKeys.tags });
    },
  });
}

export function usePutLicence() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: {
      assetId: number;
      original_text: string;
      use_detected?: boolean;
      detected_type?: string;
      attributes?: Record<string, unknown>;
    }) => {
      const { assetId, ...body } = input;
      return fetchJson<unknown>(`/api/assets/${assetId}/licence`, {
        method: "PUT",
        body: JSON.stringify(body),
      });
    },
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: queryKeys.asset(v.assetId) });
      qc.invalidateQueries({ queryKey: queryKeys.assets() });
    },
  });
}

export function useReviewLicence() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (assetId: number) => {
      await fetchMaybeJson(`/api/assets/${assetId}/licence/review`, {
        method: "POST",
      });
      return assetId;
    },
    onSuccess: (id) => {
      qc.invalidateQueries({ queryKey: queryKeys.asset(id) });
    },
  });
}

export function useCreateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: {
      name: string;
      description?: string;
      target_distribution?: string;
      glyph?: string;
      glyph_color?: string;
    }) => {
      return fetchJson<{ id: number }>(`/api/projects`, {
        method: "POST",
        body: JSON.stringify(input),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.projects });
    },
  });
}

export function useDeleteProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: number) => {
      await fetchMaybeJson(`/api/projects/${id}`, { method: "DELETE" });
      return id;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.projects });
    },
  });
}

export function usePinAssetToProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { projectId: number; asset_id: number; version_id?: number }) => {
      const { projectId, ...body } = input;
      return fetchJson<unknown>(`/api/projects/${projectId}/pins`, {
        method: "POST",
        body: JSON.stringify(body),
      });
    },
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: queryKeys.project(v.projectId) });
      qc.invalidateQueries({ queryKey: queryKeys.projects });
      qc.invalidateQueries({ queryKey: queryKeys.asset(v.asset_id) });
    },
  });
}

export function useUnpinAssetFromProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { projectId: number; pinId: number }) => {
      await fetchMaybeJson(`/api/projects/${input.projectId}/pins/${input.pinId}`, {
        method: "DELETE",
      });
      return input;
    },
    onSuccess: (v) => {
      qc.invalidateQueries({ queryKey: queryKeys.project(v.projectId) });
      qc.invalidateQueries({ queryKey: queryKeys.projects });
    },
  });
}

export function useCreateCollection() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { name: string; description?: string }) => {
      return fetchJson<{ id: number }>(`/api/collections`, {
        method: "POST",
        body: JSON.stringify(input),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.collections });
    },
  });
}

export function useDeleteCollection() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: number) => {
      await fetchMaybeJson(`/api/collections/${id}`, { method: "DELETE" });
      return id;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.collections });
    },
  });
}

export function useAddAssetToCollection() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { collectionId: number; asset_id: number }) => {
      await fetchMaybeJson(`/api/collections/${input.collectionId}/assets`, {
        method: "POST",
        body: JSON.stringify({ asset_id: input.asset_id }),
      });
      return input;
    },
    onSuccess: (v) => {
      qc.invalidateQueries({ queryKey: queryKeys.collection(v.collectionId) });
      qc.invalidateQueries({ queryKey: queryKeys.asset(v.asset_id) });
    },
  });
}

export function useRemoveAssetFromCollection() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { collectionId: number; assetId: number }) => {
      await fetchMaybeJson(`/api/collections/${input.collectionId}/assets/${input.assetId}`, {
        method: "DELETE",
      });
      return input;
    },
    onSuccess: (v) => {
      qc.invalidateQueries({ queryKey: queryKeys.collection(v.collectionId) });
      qc.invalidateQueries({ queryKey: queryKeys.asset(v.assetId) });
    },
  });
}

export interface LibrarySettingsPatch {
  theme_default?: "dark" | "light";
  grid_density_default?: "small" | "medium" | "large";
  retain_original_packages?: boolean;
  dedupe_enabled?: boolean;
  thumbnails_raster_enabled?: boolean;
  thumbnails_audio_enabled?: boolean;
  thumbnails_font_enabled?: boolean;
  thumbnails_unity_preview?: boolean;
  audit_retention_days?: number | null;
}

export function usePatchLibrarySettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: LibrarySettingsPatch) => {
      return fetchJson<unknown>(`/api/settings`, {
        method: "PATCH",
        body: JSON.stringify(input),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.librarySettings });
    },
  });
}

export function useRetryImport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: number) => {
      return fetchJson<{ asset_id: number; import_id: number }>(`/api/imports/${id}/retry`, {
        method: "POST",
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.imports });
      qc.invalidateQueries({ queryKey: queryKeys.assets() });
    },
  });
}

// useRequeueAssetPreviews regenerates every supported file's preview for an
// asset's current version. The asset detail (file statuses) and the preview
// queue are refreshed so the new "queued" rows appear immediately.
export function useRequeueAssetPreviews() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (assetId: number) =>
      fetchJson<{ asset_id: number; version_id: number; queued: number }>(
        `/api/assets/${assetId}/previews/requeue`,
        { method: "POST" },
      ),
    onSuccess: (_d, assetId) => {
      qc.invalidateQueries({ queryKey: queryKeys.asset(assetId) });
      qc.invalidateQueries({ queryKey: queryKeys.previews });
    },
  });
}

// useRescanAsset re-runs the housekeeping passes over an asset's current
// version: licence re-detect, thumbnail regenerate, preview requeue. Refreshes
// the asset detail (thumbnail, file statuses, licence) and the preview queue.
export function useRescanAsset() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (assetId: number) =>
      fetchJson<{
        asset_id: number;
        version_id: number;
        licence_updated: boolean;
        detected_type?: string;
        thumbnail_regenerated: boolean;
        previews_queued: number;
      }>(`/api/assets/${assetId}/rescan`, { method: "POST" }),
    onSuccess: (_d, assetId) => {
      qc.invalidateQueries({ queryKey: queryKeys.asset(assetId) });
      qc.invalidateQueries({ queryKey: queryKeys.assets() });
      qc.invalidateQueries({ queryKey: queryKeys.previews });
    },
  });
}

// useRequeueFilePreview regenerates the preview for a single file of an asset's
// current version.
export function useRequeueFilePreview() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { assetId: number; fileId: number }) =>
      fetchJson<{ asset_id: number; version_id: number; file_id: number }>(
        `/api/assets/${input.assetId}/files/${input.fileId}/previews/requeue`,
        { method: "POST" },
      ),
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: queryKeys.asset(v.assetId) });
      qc.invalidateQueries({ queryKey: queryKeys.previews });
    },
  });
}

export function useInviteMember() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: {
      libraryId: number;
      email: string;
      display_name?: string;
      role: "admin" | "member";
      password?: string;
    }) => {
      const { libraryId, ...body } = input;
      return fetchJson<{ user_id: number }>(`/api/libraries/${libraryId}/members`, {
        method: "POST",
        body: JSON.stringify(body),
      });
    },
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: queryKeys.libraryMembers(v.libraryId) });
    },
  });
}

export function usePatchMember() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { libraryId: number; userId: number; role: "admin" | "member" }) => {
      const { libraryId, userId, role } = input;
      return fetchJson<unknown>(`/api/libraries/${libraryId}/members/${userId}`, {
        method: "PATCH",
        body: JSON.stringify({ role }),
      });
    },
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: queryKeys.libraryMembers(v.libraryId) });
    },
  });
}

export function useRemoveMember() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { libraryId: number; userId: number }) => {
      await fetchMaybeJson(`/api/libraries/${input.libraryId}/members/${input.userId}`, {
        method: "DELETE",
      });
      return input;
    },
    onSuccess: (v) => {
      qc.invalidateQueries({ queryKey: queryKeys.libraryMembers(v.libraryId) });
    },
  });
}

export function useCreateSubsetVersion() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: {
      assetId: number;
      versionId: number;
      file_ids: number[];
      version_label?: string;
    }) => {
      const { assetId, versionId, ...body } = input;
      return fetchJson<{ asset_version_id: number }>(
        `/api/assets/${assetId}/versions/${versionId}/subset`,
        { method: "POST", body: JSON.stringify(body) },
      );
    },
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: queryKeys.asset(v.assetId) });
    },
  });
}

export function usePatchUserRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { userId: number; role: "user" | "sysadmin" }) => {
      return fetchJson<unknown>(`/api/admin/users/${input.userId}`, {
        method: "PATCH",
        body: JSON.stringify({ role: input.role }),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.adminUsers });
    },
  });
}

export function useResetUserPassword() {
  return useMutation({
    mutationFn: async (input: { userId: number; password: string }) => {
      return fetchJson<unknown>(`/api/admin/users/${input.userId}`, {
        method: "PATCH",
        body: JSON.stringify({ password: input.password }),
      });
    },
  });
}

export function usePatchServerSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { max_upload_bytes?: number; licence_kb_auto_update?: boolean }) => {
      return fetchJson<unknown>(`/api/admin/settings`, {
        method: "PATCH",
        body: JSON.stringify(input),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.adminSettings });
    },
  });
}

export function useCreateLibrary() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { name: string; description?: string }) => {
      return fetchJson<{ id: number; name: string; description: string }>(`/api/libraries`, {
        method: "POST",
        body: JSON.stringify(input),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.libraries });
    },
  });
}

// Switching library flips the server-side current_library_id; the client
// caches keyed on the previous scope (assets, tags, projects, collections,
// settings) must be dropped or the SPA will momentarily render the old
// library's data under the new library's name.
export function useSwitchCurrentLibrary() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (libraryId: number) => {
      return fetchJson<{ library_id: number }>(`/api/me/current-library`, {
        method: "POST",
        body: JSON.stringify({ library_id: libraryId }),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.me });
      qc.invalidateQueries({ queryKey: queryKeys.libraries });
      qc.invalidateQueries({ queryKey: queryKeys.assets() });
      qc.invalidateQueries({ queryKey: queryKeys.tags });
      qc.invalidateQueries({ queryKey: queryKeys.projects });
      qc.invalidateQueries({ queryKey: queryKeys.collections });
      qc.invalidateQueries({ queryKey: queryKeys.librarySettings });
    },
  });
}

export function usePatchLibrary() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { libraryId: number; name?: string; description?: string }) => {
      const { libraryId, ...body } = input;
      return fetchJson<unknown>(`/api/libraries/${libraryId}`, {
        method: "PATCH",
        body: JSON.stringify(body),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.me });
    },
  });
}

// Library deletion is irreversible — the danger-zone confirm dialog gates
// the call. Server bumps every former member's tokens_valid_after, so the
// next request from any open tab fails 401 and bounces to login.
export function useDeleteLibrary() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (libraryId: number) => {
      await fetchMaybeJson(`/api/libraries/${libraryId}`, { method: "DELETE" });
      return libraryId;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.me });
      qc.invalidateQueries({ queryKey: queryKeys.assets() });
      qc.invalidateQueries({ queryKey: queryKeys.projects });
      qc.invalidateQueries({ queryKey: queryKeys.collections });
    },
  });
}

export interface StorageCleanupCandidate {
  prefix: string;
  key: string;
  size: number;
  last_modified: string;
  filename?: string;
  library_id?: number;
  asset_id?: number;
  version_id?: number;
  kind?: string;
}

export interface StorageCleanupScan {
  min_age_seconds: number;
  total_count: number;
  total_bytes: number;
  candidates: StorageCleanupCandidate[];
}

export interface StorageCleanupDeleteResult {
  deleted_count: number;
  freed_bytes: number;
  skipped: { prefix: string; key: string; reason: string }[];
}

export function useStorageCleanupScan() {
  return useMutation({
    mutationFn: async () =>
      fetchJson<StorageCleanupScan>(`/api/admin/storage/cleanup/scan`, { method: "POST" }),
  });
}

export function useStorageCleanupDelete() {
  return useMutation({
    mutationFn: async (keys: { prefix: string; key: string }[]) =>
      fetchJson<StorageCleanupDeleteResult>(`/api/admin/storage/cleanup/delete`, {
        method: "POST",
        body: JSON.stringify({ keys }),
      }),
  });
}
