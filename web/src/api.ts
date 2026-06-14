import { getCachedAuthMode } from "@/lib/authMode";
import { clearLocalSession } from "@/lib/localAuth";
import { getUserManager } from "@/lib/oidc";
import { getAccessToken } from "@/lib/token";

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly body: unknown,
    message?: string,
  ) {
    super(message ?? `API error ${status}`);
    this.name = "ApiError";
  }
}

export class AuthRedirectError extends Error {
  constructor() {
    super("authentication required");
    this.name = "AuthRedirectError";
  }
}

export function isAuthRedirectError(err: unknown): err is AuthRedirectError {
  return err instanceof AuthRedirectError;
}

async function authHeader(): Promise<Record<string, string>> {
  const token = await getAccessToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

// OIDC supports silent renewal; local auth requires a fresh login when the
// JWT expires. renewOrReject returns true if the token was refreshed, false
// if we need to send the user back through the login flow.
let renewalInFlight: Promise<boolean> | null = null;
async function renewOrReject(): Promise<boolean> {
  if (renewalInFlight) return renewalInFlight;
  renewalInFlight = (async () => {
    const mode = getCachedAuthMode();
    if (mode === "oidc") {
      try {
        await getUserManager().signinSilent();
        return true;
      } catch {
        return false;
      }
    }
    // Local mode: expired tokens are unrecoverable without a password.
    clearLocalSession();
    return false;
  })();
  try {
    return await renewalInFlight;
  } finally {
    renewalInFlight = null;
  }
}

export async function fetchJson<T>(url: string, options: RequestInit = {}): Promise<T> {
  const hdrs: Record<string, string> = {
    "Content-Type": "application/json",
    ...(await authHeader()),
    ...((options.headers as Record<string, string>) ?? {}),
  };
  let res = await fetch(url, { ...options, headers: hdrs });
  if (res.status === 401) {
    if (await renewOrReject()) {
      res = await fetch(url, {
        ...options,
        headers: { ...hdrs, ...(await authHeader()) },
      });
    }
    if (res.status === 401) throw new AuthRedirectError();
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const msg = (body as { error?: string }).error;
    throw new ApiError(res.status, body, msg);
  }
  return (await res.json()) as T;
}

// fetchMaybeJson issues an authenticated request and tolerates responses
// with no body (204 No Content). Use it for PATCH/DELETE/POST mutations
// where the server replies with a status code only.
export async function fetchMaybeJson<T = unknown>(
  url: string,
  options: RequestInit = {},
): Promise<T | null> {
  const hdrs: Record<string, string> = {
    "Content-Type": "application/json",
    ...(await authHeader()),
    ...((options.headers as Record<string, string>) ?? {}),
  };
  let res = await fetch(url, { ...options, headers: hdrs });
  if (res.status === 401) {
    if (await renewOrReject()) {
      res = await fetch(url, {
        ...options,
        headers: { ...hdrs, ...(await authHeader()) },
      });
    }
    if (res.status === 401) throw new AuthRedirectError();
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const msg = (body as { error?: string }).error;
    throw new ApiError(res.status, body, msg);
  }
  if (res.status === 204) return null;
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
  }
}

// fetchBlob issues an authenticated GET and returns the body as a Blob. The
// file-serving endpoint forces `application/octet-stream` + a `nosniff`
// attachment disposition (served blobs are treated as hostile), so a plain
// `<img src>` can neither authenticate nor render. Fetching the bytes here and
// re-typing the Blob with a known image MIME lets the caller build a safe
// object URL for inline preview. `mimeType` overrides the Blob's type.
export async function fetchBlob(url: string, mimeType?: string): Promise<Blob> {
  const hdrs = await authHeader();
  let res = await fetch(url, { headers: hdrs });
  if (res.status === 401) {
    if (await renewOrReject()) {
      res = await fetch(url, { headers: { ...hdrs, ...(await authHeader()) } });
    }
    if (res.status === 401) throw new AuthRedirectError();
  }
  if (!res.ok) throw new ApiError(res.status, null);
  const blob = await res.blob();
  return mimeType ? blob.slice(0, blob.size, mimeType) : blob;
}

// uploadFile posts a FormData body (single file under the `file` field) and
// surfaces XHR upload progress via onProgress. Uses XMLHttpRequest because
// fetch has no upload-progress events yet.
export function uploadFile(
  url: string,
  file: File,
  onProgress?: (fraction: number) => void,
): Promise<unknown> {
  return new Promise((resolve, reject) => {
    void (async () => {
      const token = await getAccessToken();
      const form = new FormData();
      form.append("file", file);

      const xhr = new XMLHttpRequest();
      xhr.open("POST", url);
      if (token) xhr.setRequestHeader("Authorization", `Bearer ${token}`);

      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && onProgress) onProgress(e.loaded / e.total);
      };
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            resolve(JSON.parse(xhr.responseText || "{}"));
          } catch {
            resolve(null);
          }
        } else if (xhr.status === 401) {
          reject(new AuthRedirectError());
        } else {
          let msg = `upload failed ${xhr.status}`;
          try {
            const body = JSON.parse(xhr.responseText || "{}") as { error?: string };
            if (body.error) msg = `upload failed ${xhr.status}: ${body.error}`;
          } catch {
            // non-JSON body — keep the generic message
          }
          reject(new ApiError(xhr.status, xhr.responseText, msg));
        }
      };
      xhr.onerror = () => reject(new ApiError(0, null, "network error"));
      xhr.send(form);
    })();
  });
}
