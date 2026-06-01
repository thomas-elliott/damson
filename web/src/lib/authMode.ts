// Auth mode is a server-side deployment choice — local vs OIDC. The mode
// itself never changes for the lifetime of a server; `needs_bootstrap`
// can flip from true → false the moment the first user is created.
//
// All callers go through TanStack Query (see `useAuthMode` in
// hooks/useDataQueries.ts). The `getCachedAuthMode` helper exists for the
// few places that need the mode synchronously without a hook (api.ts's
// 401 renewal, lib/token.ts's bearer attach) — it reads directly out of
// the same query cache rather than maintaining a parallel singleton, so
// invalidations are a single-source operation.

import { configureOidc } from "@/lib/oidc";
import { queryClient } from "@/lib/queryClient";
import { queryKeys } from "@/lib/queryKeys";

export type AuthMode = "oidc" | "local";

export interface AuthModeInfo {
  mode: AuthMode;
  needs_bootstrap: boolean;
}

interface AuthModeBody {
  mode: AuthMode;
  needs_bootstrap?: boolean;
  // Present only in OIDC mode — the issuer + client_id the SPA needs to
  // configure oidc-client-ts at runtime (see lib/oidc.ts).
  oidc?: { authority: string; client_id: string; scope?: string };
}

export async function fetchAuthMode(): Promise<AuthModeInfo> {
  const res = await fetch("/api/auth/mode");
  if (!res.ok) throw new Error(`auth mode lookup failed (${res.status})`);
  const body = (await res.json()) as AuthModeBody;
  // Configure the OIDC client immediately from the server-supplied config, so
  // every downstream caller (AuthGuard, token.ts, the api.ts 401 silent-renew
  // path) can rely on getUserManager() being ready. Idempotent.
  if (body.mode === "oidc" && body.oidc) {
    configureOidc(body.oidc);
  }
  return { mode: body.mode, needs_bootstrap: body.needs_bootstrap ?? false };
}

// Synchronous accessor backed by TanStack's query cache. Returns null
// until `useAuthMode()` (or another caller) has populated it.
export function getCachedAuthMode(): AuthMode | null {
  const data = queryClient.getQueryData<AuthModeInfo>(queryKeys.authMode);
  return data?.mode ?? null;
}
