// Auth mode is a server-side deployment choice. The SPA fetches it once at
// boot and caches the result; mode itself never changes between loads for
// the same server. `needs_bootstrap` can flip from true → false during a
// session (once the first local admin is created) — callers that care
// should invalidate the cache via `invalidateAuthMode()` after bootstrap.

export type AuthMode = "oidc" | "local";

export interface AuthModeInfo {
  mode: AuthMode;
  needs_bootstrap: boolean;
}

let cached: AuthModeInfo | null = null;

export async function fetchAuthMode(): Promise<AuthModeInfo> {
  if (cached) return cached;
  const res = await fetch("/api/auth/mode");
  if (!res.ok) throw new Error(`auth mode lookup failed (${res.status})`);
  const body = (await res.json()) as { mode: AuthMode; needs_bootstrap?: boolean };
  cached = { mode: body.mode, needs_bootstrap: body.needs_bootstrap ?? false };
  return cached;
}

export function getCachedAuthMode(): AuthMode | null {
  return cached?.mode ?? null;
}

export function invalidateAuthMode(): void {
  cached = null;
}
