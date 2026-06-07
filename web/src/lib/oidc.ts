import { UserManager, WebStorageStateStore } from "oidc-client-ts";

// Non-secret OIDC client config delivered by the server (GET /api/auth/mode →
// `oidc`), so a single prebuilt bundle can target any IdP without baking
// VITE_OIDC_* at build time. Redirect URIs are NOT configured — they're derived
// from window.location.origin, so they're always correct for whatever host the
// SPA is served from.
export interface OidcConfig {
  authority: string;
  client_id: string;
  scope?: string;
}

let manager: UserManager | null = null;

// configureOidc builds the UserManager singleton from server-provided config.
// Idempotent — the first call wins. The auth mode is fixed for a server's
// lifetime, so re-configuring would only churn oidc-client-ts state.
export function configureOidc(cfg: OidcConfig): UserManager {
  if (manager) return manager;
  const origin = window.location.origin;
  manager = new UserManager({
    authority: cfg.authority,
    client_id: cfg.client_id,
    // /auth/callback is both the interactive redirect target (App route ->
    // LoginCallbackPage) and the hidden-iframe silent-renew target; the page
    // dispatches on whether it's running inside an iframe.
    redirect_uri: `${origin}/auth/callback`,
    silent_redirect_uri: `${origin}/auth/callback`,
    // No post_logout_redirect_uri: logout is local-only (see lib/logout.ts) —
    // we clear the stored user rather than hitting the IdP's end_session, so an
    // end-session redirect target would never be used.
    scope: cfg.scope ?? "openid profile email",
    automaticSilentRenew: true,
    userStore: new WebStorageStateStore({ store: window.localStorage }),
  });
  return manager;
}

// getUserManager returns the configured singleton. Callers must ensure
// configureOidc has run first — fetchAuthMode() does this as a side effect on
// the initial /api/auth/mode probe, which every entry path awaits. Throwing
// surfaces an ordering bug loudly rather than building a half-configured client.
export function getUserManager(): UserManager {
  if (!manager) {
    throw new Error("OIDC not configured — fetchAuthMode() must resolve before getUserManager()");
  }
  return manager;
}

export async function getStoredUser() {
  return getUserManager().getUser();
}
