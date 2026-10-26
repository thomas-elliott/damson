// Where to send someone once they have signed in: the page they asked for,
// or "/" when there was none or it is not a page of this app. Honouring any
// URL would make sign-in an open redirect.

const SIGN_IN_PATHS = new Set(["/login", "/auth/callback", "/bootstrap"]);

type PathLike = { pathname: string; search?: string; hash?: string };

export function safeReturnTo(candidate: unknown): string {
  const raw = toPath(candidate);
  if (!raw?.startsWith("/")) return "/";
  let url: URL;
  try {
    url = new URL(raw, window.location.origin);
  } catch {
    return "/";
  }
  // "//host" and "/\host" resolve to another origin.
  if (url.origin !== window.location.origin) return "/";
  // Going back to a sign-in page would loop.
  if (SIGN_IN_PATHS.has(url.pathname.replace(/\/+$/, ""))) return "/";
  return url.pathname + url.search + url.hash;
}

function toPath(candidate: unknown): string | null {
  if (typeof candidate === "string") return candidate;
  if (candidate && typeof candidate === "object" && "pathname" in candidate) {
    const { pathname, search = "", hash = "" } = candidate as PathLike;
    if (typeof pathname === "string") return pathname + search + hash;
  }
  return null;
}
