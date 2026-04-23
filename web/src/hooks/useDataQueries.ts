import { useQuery } from "@tanstack/react-query";

import { fetchJson } from "@/api";
import { fetchAuthMode, type AuthMode } from "@/lib/authMode";
import { HOUR, MINUTE } from "@/lib/cachePolicy";
import { queryKeys } from "@/lib/queryKeys";

// -- /api/auth/mode ---------------------------------------------------------

export function useAuthMode() {
  const q = useQuery<AuthMode>({
    queryKey: queryKeys.authMode,
    queryFn: fetchAuthMode,
    staleTime: Infinity, // doesn't change between page loads
    gcTime: Infinity,
  });
  return {
    mode: q.data ?? null,
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

export interface AssetRow {
  id: number;
  name: string;
  file_count: number;
  size_bytes: number;
  needs_review: boolean;
  created_at: string;
}

interface AssetListResponse {
  assets: AssetRow[];
  total: number;
}

export function useAssets(enabled: boolean = true) {
  const q = useQuery({
    queryKey: queryKeys.assets(),
    queryFn: () => fetchJson<AssetListResponse>("/api/assets"),
    staleTime: 30 * 1000, // assets change often during import; short stale
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
