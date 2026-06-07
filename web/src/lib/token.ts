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
    // Send the id_token, not the access_token. The backend is a self-contained
    // resource server that JWT-verifies the bearer with aud == client_id (an
    // ID-token verifier); some IdPs issue OPAQUE access tokens that can't be
    // JWT-verified at all. The id_token is always a signed JWT with
    // aud=<client_id>, exactly what the verifier expects, and carries the
    // sub/email/name claims the backend maps to a user. Using it uniformly
    // keeps the SPA IdP-agnostic. expires_at tracks the token lifetime; the
    // id_token's own exp is also enforced server-side.
    if (!user || user.expired || !user.id_token) return null;
    return user.id_token;
  }
  return null;
}
