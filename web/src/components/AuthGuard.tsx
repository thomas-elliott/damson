import { useEffect, useState } from "react";
import { Navigate, Outlet, useLocation } from "react-router";

import { useAuthMode } from "@/hooks/useDataQueries";
import { readLocalSession } from "@/lib/localAuth";
import { getUserManager } from "@/lib/oidc";

type AuthState = "checking" | "ready" | "needs-login";

// AuthGuard runs before any library-scoped route. It blocks on the
// /api/auth/mode lookup, then dispatches to the right flow:
//
//   - OIDC: getUser() from oidc-client-ts; if missing/expired, trigger
//           signinRedirect (browser leaves the SPA entirely).
//   - Local: read the stored JWT; if missing/expired, bounce to /login.
export function AuthGuard() {
  const { mode, needsBootstrap, loading: modeLoading } = useAuthMode();
  const location = useLocation();
  const [state, setState] = useState<AuthState>("checking");

  useEffect(() => {
    if (modeLoading || !mode) return;
    let cancelled = false;

    if (mode === "local") {
      const sess = readLocalSession();
      setState(sess ? "ready" : "needs-login");
      return;
    }

    // OIDC path.
    getUserManager()
      .getUser()
      .then((user) => {
        if (cancelled) return;
        if (!user || user.expired) {
          void getUserManager().signinRedirect();
        } else {
          setState("ready");
        }
      })
      .catch((err) => {
        // Corrupted userStore or oidc-client-ts state machine error —
        // bounce into the login flow rather than stalling on the spinner.
        if (cancelled) return;
        console.error("OIDC getUser failed", err);
        void getUserManager().signinRedirect();
      });
    return () => {
      cancelled = true;
    };
  }, [mode, modeLoading]);

  if (modeLoading || state === "checking") {
    return (
      <div className="flex min-h-screen items-center justify-center text-ui text-text-3">
        Authenticating…
      </div>
    );
  }
  if (state === "needs-login") {
    // `needsBootstrap` only matters for unauthenticated users — an authed
    // session means the server has at least one user, regardless of what a
    // stale cached /api/auth/mode says. Checking it inside "needs-login"
    // also avoids a race right after bootstrap where the flag hasn't
    // refetched yet.
    if (mode === "local" && needsBootstrap) {
      return <Navigate to="/bootstrap" replace />;
    }
    return <Navigate to="/login" replace state={{ from: location }} />;
  }
  return <Outlet />;
}
