// Auth mode is a server-side deployment choice. The SPA fetches it once at
// boot and caches the result; it never changes between loads for the same
// server.

export type AuthMode = "oidc" | "local";

let cached: AuthMode | null = null;

export async function fetchAuthMode(): Promise<AuthMode> {
  if (cached) return cached;
  const res = await fetch("/api/auth/mode");
  if (!res.ok) throw new Error(`auth mode lookup failed (${res.status})`);
  const body = (await res.json()) as { mode: AuthMode };
  cached = body.mode;
  return cached;
}

export function getCachedAuthMode(): AuthMode | null {
  return cached;
}
