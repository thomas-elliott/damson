import { UserManager, WebStorageStateStore } from "oidc-client-ts";

let manager: UserManager | null = null;

export function getUserManager(): UserManager {
  if (manager) return manager;
  manager = new UserManager({
    authority: import.meta.env.VITE_OIDC_AUTHORITY,
    client_id: import.meta.env.VITE_OIDC_CLIENT_ID,
    redirect_uri: import.meta.env.VITE_OIDC_REDIRECT_URI,
    silent_redirect_uri: import.meta.env.VITE_OIDC_SILENT_REDIRECT_URI,
    post_logout_redirect_uri: import.meta.env.VITE_OIDC_POST_LOGOUT_REDIRECT_URI,
    scope: import.meta.env.VITE_OIDC_SCOPE ?? "openid profile email",
    automaticSilentRenew: true,
    userStore: new WebStorageStateStore({ store: window.localStorage }),
  });
  return manager;
}

export async function getStoredUser() {
  return getUserManager().getUser();
}
