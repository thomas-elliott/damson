import { useEffect } from "react";
import { useNavigate } from "react-router-dom";

import { getUserManager } from "@/lib/oidc";

export function LoginCallbackPage() {
  const navigate = useNavigate();
  useEffect(() => {
    getUserManager()
      .signinRedirectCallback()
      .then(() => navigate("/", { replace: true }))
      .catch((err: unknown) => {
        // Even on failure, push back to / — AuthGuard will re-trigger the
        // login redirect on the next render.
        console.error("OIDC callback failed", err);
        navigate("/", { replace: true });
      });
  }, [navigate]);
  return (
    <div className="flex min-h-screen items-center justify-center text-ui text-text-3">
      Completing sign-in…
    </div>
  );
}
