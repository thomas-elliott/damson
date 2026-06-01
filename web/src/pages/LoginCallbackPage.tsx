import { useEffect } from "react";
import { useNavigate } from "react-router";

import { fetchAuthMode } from "@/lib/authMode";
import { getUserManager } from "@/lib/oidc";

export function LoginCallbackPage() {
  const navigate = useNavigate();
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      // This route is a cold load (the IdP redirected the whole browser back
      // here, or oidc-client-ts loaded it in a hidden silent-renew iframe), so
      // the UserManager isn't configured yet — fetch the server's OIDC config
      // first (fetchAuthMode configures it as a side effect).
      try {
        await fetchAuthMode();
      } catch (err) {
        console.error("auth mode lookup failed in OIDC callback", err);
      }
      // Inside an iframe ⇒ silent renew; top-level ⇒ interactive sign-in.
      const inIframe = window.self !== window.top;
      try {
        if (inIframe) {
          await getUserManager().signinSilentCallback();
          return; // parent frame owns navigation; nothing to do here.
        }
        await getUserManager().signinRedirectCallback();
      } catch (err) {
        // Even on failure, fall through to / — AuthGuard re-triggers the login
        // redirect on the next render.
        console.error("OIDC callback failed", err);
      }
      if (!cancelled && !inIframe) navigate("/", { replace: true });
    })();
    return () => {
      cancelled = true;
    };
  }, [navigate]);
  return (
    <div className="flex min-h-screen items-center justify-center text-ui text-text-3">
      Completing sign-in…
    </div>
  );
}
