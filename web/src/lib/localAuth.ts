// Local-auth token state lives in localStorage. Short TTL tokens (1h) are
// extended in-place via /api/auth/refresh while the user is active — see
// `maybeRefreshLocalSession` below.

const TOKEN_KEY = "damson.local.token";
const EXP_KEY = "damson.local.expiresAt";

// Refresh proactively when the stored token is within this window of
// expiring. Picked to be comfortably larger than typical roundtrip + UI
// idle so a user mid-task doesn't hit a 401.
const REFRESH_WINDOW_MS = 5 * 60 * 1000;

export interface LocalSession {
  token: string;
  expiresAt: Date;
}

export function readLocalSession(): LocalSession | null {
  const token = localStorage.getItem(TOKEN_KEY);
  const expStr = localStorage.getItem(EXP_KEY);
  if (!token || !expStr) return null;
  const expiresAt = new Date(expStr);
  if (Number.isNaN(expiresAt.getTime())) return null;
  if (expiresAt.getTime() <= Date.now()) {
    clearLocalSession();
    return null;
  }
  return { token, expiresAt };
}

export function storeLocalSession(token: string, expiresAt: string | Date): void {
  const iso = typeof expiresAt === "string" ? expiresAt : expiresAt.toISOString();
  localStorage.setItem(TOKEN_KEY, token);
  localStorage.setItem(EXP_KEY, iso);
}

export function clearLocalSession(): void {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(EXP_KEY);
}

export async function localLogin(email: string, password: string): Promise<LocalSession> {
  const res = await fetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (res.status === 401) {
    throw new Error("invalid credentials");
  }
  if (!res.ok) {
    throw new Error(`login failed (${res.status})`);
  }
  const body = (await res.json()) as { token: string; expires_at: string };
  storeLocalSession(body.token, body.expires_at);
  return { token: body.token, expiresAt: new Date(body.expires_at) };
}

// localRefresh exchanges the current bearer for a fresh JWT. Server-side
// the existing token must still be valid (the auth middleware runs first),
// so this only works inside the active-session window.
export async function localRefresh(): Promise<LocalSession> {
  const sess = readLocalSession();
  if (!sess) throw new Error("no local session to refresh");
  const res = await fetch("/api/auth/refresh", {
    method: "POST",
    headers: { Authorization: `Bearer ${sess.token}` },
  });
  if (!res.ok) {
    throw new Error(`refresh failed (${res.status})`);
  }
  const body = (await res.json()) as { token: string; expires_at: string };
  storeLocalSession(body.token, body.expires_at);
  return { token: body.token, expiresAt: new Date(body.expires_at) };
}

// maybeRefreshLocalSession is a no-op unless the stored session is within
// REFRESH_WINDOW_MS of expiry, in which case it kicks off a refresh and
// awaits the result. Concurrent callers share the same in-flight refresh.
// Failures are swallowed — the next request will surface the 401 and the
// SPA will fall through to the login redirect via api.ts.
let refreshInFlight: Promise<LocalSession> | null = null;
export async function maybeRefreshLocalSession(): Promise<void> {
  const sess = readLocalSession();
  if (!sess) return;
  if (sess.expiresAt.getTime() - Date.now() > REFRESH_WINDOW_MS) return;
  if (refreshInFlight) {
    try {
      await refreshInFlight;
    } catch {
      // Concurrent refresh failed; the caller can retry.
    }
    return;
  }
  refreshInFlight = localRefresh();
  try {
    await refreshInFlight;
  } catch {
    // Leave the stored session as-is; downstream 401 handling will redirect.
  } finally {
    refreshInFlight = null;
  }
}

// localBootstrap creates the very first admin user. Safe to call only when
// /api/auth/mode reported `needs_bootstrap=true` — the server returns 409
// once any user exists.
export async function localBootstrap(email: string, password: string): Promise<LocalSession> {
  const res = await fetch("/api/auth/bootstrap", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (res.status === 409) {
    throw new Error("already bootstrapped — go to the login page");
  }
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? `bootstrap failed (${res.status})`);
  }
  const body = (await res.json()) as { token: string; expires_at: string };
  storeLocalSession(body.token, body.expires_at);
  return { token: body.token, expiresAt: new Date(body.expires_at) };
}
