import { useMutation, useQueryClient } from "@tanstack/react-query";

import { fetchJson, fetchMaybeJson } from "@/api";
import { queryKeys } from "@/lib/queryKeys";

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
      return fetchJson<{ asset_id: number; tag_id: number }>(
        `/api/assets/${input.assetId}/tags`,
        { method: "POST", body: JSON.stringify({ path: input.path }) },
      );
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
    mutationFn: async (input: {
      projectId: number;
      asset_id: number;
      version_id?: number;
    }) => {
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
      await fetchMaybeJson(
        `/api/collections/${input.collectionId}/assets/${input.assetId}`,
        { method: "DELETE" },
      );
      return input;
    },
    onSuccess: (v) => {
      qc.invalidateQueries({ queryKey: queryKeys.collection(v.collectionId) });
      qc.invalidateQueries({ queryKey: queryKeys.asset(v.assetId) });
    },
  });
}
