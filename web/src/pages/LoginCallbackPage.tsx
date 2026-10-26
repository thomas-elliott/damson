import { useEffect, useState } from "react";
import { useNavigate } from "react-router";

import { Button } from "@/design-system";
import { fetchAuthMode } from "@/lib/authMode";
import { getUserManager } from "@/lib/oidc";
import { safeReturnTo } from "@/lib/returnTo";

// The signin/silent-renew callbacks are single-use: oidc-client-ts removes the
// stored SigninState on the first run, so a second pass throws "No matching
// state found in storage". React StrictMode double-invokes effects in dev (and
// a stray remount would do the same), so the callback work is memoised at
// module scope — every invocation awaits the SAME promise, but each live mount
// still drives its own navigation. One document = one callback; the
// silent-renew iframe is its own document with fresh module state.
type CallbackResult = { inIframe: boolean; returnTo?: string; error?: unknown };

let callbackInFlight: Promise<CallbackResult> | null = null;

function runCallbackOnce(): Promise<CallbackResult> {
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
        return { inIframe };
      }
      // AuthGuard put the requested page in the sign-in state.
      const user = await getUserManager().signinRedirectCallback();
      const state = user.state as { returnTo?: unknown } | undefined;
      return { inIframe, returnTo: safeReturnTo(state?.returnTo) };
    } catch (err) {
      console.error("OIDC callback failed", err);
      return { inIframe, error: err };
    }
  })();
  return callbackInFlight;
}

// Only the IdP's description is shown: other errors are oidc-client-ts
// internals such as "No matching state found in storage". ErrorResponse is
// matched by the name it sets, not instanceof, which a second copy of the
// library would break.
function failureMessage(error: unknown): string {
  if (error instanceof Error && error.name === "ErrorResponse") {
    const description = (error as { error_description?: unknown }).error_description;
    if (typeof description === "string" && description) return description;
  }
  return "Something went wrong while signing you in. Try signing in again.";
}

export function LoginCallbackPage() {
  const navigate = useNavigate();
  const [failure, setFailure] = useState<string | null>(null);
  useEffect(() => {
    let cancelled = false;
    void runCallbackOnce().then(({ inIframe, returnTo, error }) => {
      // The iframe (silent renew) has no SPA route to drive; the parent frame
      // owns navigation there.
      if (cancelled || inIframe) return;
      // A failed sign-in stays here: going to / would have AuthGuard send the
      // browser straight back to the IdP.
      if (error) setFailure(failureMessage(error));
      else navigate(returnTo ?? "/", { replace: true });
    });
    return () => {
      cancelled = true;
    };
  }, [navigate]);
  if (failure) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-3 text-center">
        <p className="text-subhead font-medium text-foreground">Sign-in didn't complete</p>
        <p className="max-w-[42ch] text-body text-muted-foreground">{failure}</p>
        <Button variant="primary" onClick={() => navigate("/", { replace: true })}>
          Sign in again
        </Button>
      </div>
    );
  }
  return (
    <div className="flex min-h-screen items-center justify-center text-ui text-subtle-foreground">
      Completing sign-in…
    </div>
  );
}
