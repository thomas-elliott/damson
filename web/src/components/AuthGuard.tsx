import { useEffect, useState } from "react";
import { Navigate, Outlet, useLocation } from "react-router-dom";

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
  const { mode, loading: modeLoading } = useAuthMode();
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
      });
    return () => {
      cancelled = true;
    };
  }, [mode, modeLoading]);

  if (modeLoading || state === "checking") {
    return (
      <div className="flex min-h-screen items-center justify-center text-sm text-muted">
        Authenticating…
      </div>
    );
  }
  if (state === "needs-login") {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }
  return <Outlet />;
}
