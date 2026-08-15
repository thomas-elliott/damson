import { useEffect } from "react";
import { useNavigate } from "react-router";

import { fetchAuthMode } from "@/lib/authMode";
import { getUserManager } from "@/lib/oidc";

// The signin/silent-renew callbacks are single-use: oidc-client-ts removes the
// stored SigninState on the first run, so a second pass throws "No matching
// state found in storage". React StrictMode double-invokes effects in dev (and
// a stray remount would do the same), so the callback work is memoised at
// module scope — every invocation awaits the SAME promise, but each live mount
// still drives its own navigation. One document = one callback; the
// silent-renew iframe is its own document with fresh module state.
let callbackInFlight: Promise<{ inIframe: boolean }> | null = null;

function runCallbackOnce(): Promise<{ inIframe: boolean }> {
  if (callbackInFlight) return callbackInFlight;
  callbackInFlight = (async () => {
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
      } else {
        await getUserManager().signinRedirectCallback();
      }
    } catch (err) {
      // Even on failure, fall through to / — AuthGuard re-triggers the login
      // redirect on the next render.
      console.error("OIDC callback failed", err);
    }
    return { inIframe };
  })();
  return callbackInFlight;
}

export function LoginCallbackPage() {
  const navigate = useNavigate();
  useEffect(() => {
    let cancelled = false;
    void runCallbackOnce().then(({ inIframe }) => {
      // The iframe (silent renew) has no SPA route to drive; the parent frame
      // owns navigation there.
      if (!cancelled && !inIframe) navigate("/", { replace: true });
    });
    return () => {
      cancelled = true;
    };
  }, [navigate]);
  return (
    <div className="flex min-h-screen items-center justify-center text-ui text-subtle-foreground">
      Completing sign-in…
    </div>
  );
}
