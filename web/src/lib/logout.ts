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
    // Local-only logout: clear the stored user so the next AuthGuard pass
    // bounces to /login. We deliberately do NOT call signoutRedirect() — the
    // IdP's end_session needs a registered post_logout_redirect_uri and would
    // end the whole SSO session across every app sharing the IdP.
    await getUserManager()
      .removeUser()
      .catch(() => {});
  } else {
    clearLocalSession();
  }
  // Full navigation (not React Router) so the query cache is wiped too.
  window.location.href = "/login";
}
