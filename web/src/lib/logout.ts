// Single logout entry point, shared by every "Sign out" control. Handles both
// auth modes and always lands the user back on /login — including from the
// authenticated-but-no-library state, where the only way out used to be a
// dead `<a href="#">`.

import { getCachedAuthMode } from "@/lib/authMode";
import { clearLocalSession } from "@/lib/localAuth";
import { getUserManager } from "@/lib/oidc";

export async function logout(): Promise<void> {
  const mode = getCachedAuthMode();
  if (mode === "oidc") {
    try {
      // Leaves the SPA entirely; the IdP ends the session and redirects back
      // to post_logout_redirect_uri (/login).
      await getUserManager().signoutRedirect();
      return;
    } catch {
      // IdP advertises no end-session endpoint (or it failed): clear the
      // stored user locally so the next AuthGuard pass bounces to login.
      await getUserManager()
        .removeUser()
        .catch(() => {});
    }
  } else {
    clearLocalSession();
  }
  // Full navigation (not React Router) so the query cache is wiped too.
  window.location.href = "/login";
}
