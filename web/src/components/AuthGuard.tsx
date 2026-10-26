import { useEffect, useState } from "react";
import { Navigate, Outlet, useLocation } from "react-router";

import { useAuthMode, useMe } from "@/hooks/useDataQueries";
import { readLocalSession } from "@/lib/localAuth";
import { getUserManager } from "@/lib/oidc";
import { ProfileSetupPage } from "@/pages/ProfileSetupPage";

type AuthState = "checking" | "ready" | "needs-login";

// AuthGuard runs before any library-scoped route. It blocks on the
// /api/auth/mode lookup, then dispatches to the right flow:
//
//   - OIDC: getUser() from oidc-client-ts; if missing/expired, trigger
//           signinRedirect (browser leaves the SPA entirely), carrying the
//           requested page for LoginCallbackPage to return to.
//   - Local: read the stored JWT; if missing/expired, bounce to /login with
//           the requested page in router state.
export function AuthGuard() {
  const { mode, needsBootstrap, loading: modeLoading } = useAuthMode();
  const location = useLocation();
  const [state, setState] = useState<AuthState>("checking");

  useEffect(() => {
    if (modeLoading || !mode) return;
    let cancelled = false;

    if (mode === "local") {
      const next: AuthState = readLocalSession() ? "ready" : "needs-login";
      // Defer off the effect's synchronous body so we don't trigger a
      // cascading render mid-commit (react-hooks/set-state-in-effect); the
      // "Authenticating…" spinner covers this microtask. The OIDC branch below
      // already updates state from async callbacks, which the rule allows.
      queueMicrotask(() => {
        if (!cancelled) setState(next);
      });
      return () => {
        cancelled = true;
      };
    }

    // OIDC path. The requested page rides along in the sign-in state, which
    // oidc-client-ts keeps in local storage and hands back to the callback.
    const { pathname, search, hash } = window.location;
    const signIn = () =>
      void getUserManager().signinRedirect({ state: { returnTo: pathname + search + hash } });
    getUserManager()
      .getUser()
      .then((user) => {
        if (cancelled) return;
        if (!user || user.expired) {
          signIn();
        } else {
          setState("ready");
        }
      })
      .catch((err) => {
        // Corrupted userStore or oidc-client-ts state machine error —
        // bounce into the login flow rather than stalling on the spinner.
        if (cancelled) return;
        console.error("OIDC getUser failed", err);
        signIn();
      });
    return () => {
      cancelled = true;
    };
  }, [mode, modeLoading]);

  if (modeLoading || state === "checking") {
    return (
      <div className="flex min-h-screen items-center justify-center text-ui text-subtle-foreground">
        Authenticating…
      </div>
    );
  }
  if (state === "needs-login") {
    // `needsBootstrap` only matters for unauthenticated users — an authed
    // session means the server has at least one user. Bootstrap success
    // now seeds the cached value synchronously (BootstrapPage uses
    // queryClient.setQueryData), so by the time AuthGuard re-runs the
    // flag already reflects reality.
    if (mode === "local" && needsBootstrap) {
      return <Navigate to="/bootstrap" replace />;
    }
    return <Navigate to="/login" replace state={{ from: location }} />;
  }
  return <ProfileGate />;
}

// ProfileGate sits between a valid session and the app. It holds the user on
// the first-login profile step until they've set an email (needs_profile),
// then falls through to the routed app. Separate component so useMe only runs
// once we know the session is ready.
function ProfileGate() {
  const { data: me, loading } = useMe();

  if (loading || !me) {
    return (
      <div className="flex min-h-screen items-center justify-center text-ui text-subtle-foreground">
        Authenticating…
      </div>
    );
  }
  if (me.needs_profile) {
    return <ProfileSetupPage me={me} />;
  }
  return <Outlet />;
}
