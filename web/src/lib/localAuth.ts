// Local-auth token state lives in localStorage. Short TTL tokens (1h) mean
// users re-log-in roughly once per session; refresh tokens are out of scope
// for the first pass.

const TOKEN_KEY = "damson.local.token";
const EXP_KEY = "damson.local.expiresAt";

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
