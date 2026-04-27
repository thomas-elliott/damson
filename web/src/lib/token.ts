// Unified access-token accessor. The api.ts fetch wrapper asks this module
// for the current bearer token regardless of which auth mode is active.

import { getCachedAuthMode } from "@/lib/authMode";
import { maybeRefreshLocalSession, readLocalSession } from "@/lib/localAuth";
import { getStoredUser } from "@/lib/oidc";

export async function getAccessToken(): Promise<string | null> {
  const mode = getCachedAuthMode();
  if (mode === "local") {
    // Proactively refresh near expiry so a long-running request mid-session
    // doesn't bounce into /login. The helper short-circuits when there's
    // plenty of time left on the token.
    await maybeRefreshLocalSession();
    return readLocalSession()?.token ?? null;
  }
  if (mode === "oidc") {
    const user = await getStoredUser();
    if (!user || user.expired || !user.access_token) return null;
    return user.access_token;
  }
  return null;
}
