// Unified access-token accessor. The api.ts fetch wrapper asks this module
// for the current bearer token regardless of which auth mode is active.

import { getCachedAuthMode } from "@/lib/authMode";
import { readLocalSession } from "@/lib/localAuth";
import { getStoredUser } from "@/lib/oidc";

export async function getAccessToken(): Promise<string | null> {
  const mode = getCachedAuthMode();
  if (mode === "local") {
    return readLocalSession()?.token ?? null;
  }
  if (mode === "oidc") {
    const user = await getStoredUser();
    if (!user || user.expired || !user.access_token) return null;
    return user.access_token;
  }
  return null;
}
