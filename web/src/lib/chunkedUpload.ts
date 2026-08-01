import {
  ApiError,
  UploadAbortedError,
  fetchJson,
  fetchMaybeJson,
  type UploadResponse,
} from "@/api";
import { getAccessToken } from "@/lib/token";

import type { HashMessage } from "./sha256Worker";

export const CHUNKED_UPLOAD_THRESHOLD = 256 * 1024 * 1024;

const HASH_READ_BYTES = 8 * 1024 * 1024;
const MAX_CHUNK_ATTEMPTS = 5;
const RETRY_BASE_MS = 500;
const RESUME_KEY = "damson.uploads.resumable";

export type UploadPhase = "hashing" | "uploading";

export interface ChunkedUploadOptions {
  file: File;
  assetId?: number;
  signal?: AbortSignal;
  onPhase?: (phase: UploadPhase) => void;
  onProgress?: (fraction: number) => void;
}

interface SessionState {
  upload_id: number;
  status: string;
  chunk_size: number;
  size_bytes: number;
  received_bytes: number;
  next_chunk: number;
  total_chunks: number;
  deduped: boolean;
  asset_id?: number;
  import_id?: number;
  suggested_match?: { asset_id: number; name: string };
}

// fileKey identifies a file across reloads, where no File handle survives.
function fileKey(file: File): string {
  return `${file.name}:${file.size}:${file.lastModified}`;
}

function readResumeMap(): Record<string, number> {
  try {
    const raw = window.localStorage.getItem(RESUME_KEY);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    return parsed && typeof parsed === "object" ? (parsed as Record<string, number>) : {};
  } catch {
    return {};
  }
}

function rememberSession(file: File, uploadId: number) {
  try {
    const map = readResumeMap();
    map[fileKey(file)] = uploadId;
    window.localStorage.setItem(RESUME_KEY, JSON.stringify(map));
  } catch {
    // storage unavailable — resume across reloads is simply not offered
  }
}

function forgetSession(file: File) {
  try {
    const map = readResumeMap();
    delete map[fileKey(file)];
    window.localStorage.setItem(RESUME_KEY, JSON.stringify(map));
  } catch {
    // storage unavailable — nothing to clean up
  }
}

// resumableSession returns a still-open session for this exact file, or null.
async function resumableSession(file: File): Promise<SessionState | null> {
  const uploadId = readResumeMap()[fileKey(file)];
  if (!uploadId) return null;
  try {
    const state = await fetchJson<SessionState>(`/api/imports/uploads/${uploadId}`);
    if (state.status === "open" && state.size_bytes === file.size) return state;
  } catch {
    // fall through and start fresh
  }
  forgetSession(file);
  return null;
}

export function hashFile(
  file: File,
  signal?: AbortSignal,
  onProgress?: (fraction: number) => void,
): Promise<string> {
  return new Promise((resolve, reject) => {
    const worker = new Worker(new URL("./sha256Worker.ts", import.meta.url), { type: "module" });
    const done = (fn: () => void) => {
      worker.terminate();
      signal?.removeEventListener("abort", onAbort);
      fn();
    };
    const onAbort = () => done(() => reject(new UploadAbortedError()));
    signal?.addEventListener("abort", onAbort);

    worker.onmessage = (e: MessageEvent<HashMessage>) => {
      const msg = e.data;
      if (msg.type === "progress") {
        onProgress?.(file.size ? msg.read / file.size : 1);
      } else if (msg.type === "done") {
        done(() => resolve(msg.sha256));
      } else {
        done(() => reject(new Error(msg.message)));
      }
    };
    worker.onerror = (e) => done(() => reject(new Error(e.message || "hash worker failed")));
    worker.postMessage({ file, chunkBytes: HASH_READ_BYTES });
  });
}

// uploadFileChunked hashes the file, opens (or resumes) a session, and sends
// it a chunk at a time.
export async function uploadFileChunked({
  file,
  assetId,
  signal,
  onPhase,
  onProgress,
}: ChunkedUploadOptions): Promise<UploadResponse> {
  throwIfAborted(signal);
  onPhase?.("hashing");
  const sha256 = await hashFile(file, signal, onProgress);

  throwIfAborted(signal);
  onPhase?.("uploading");
  onProgress?.(0);

  let session = await resumableSession(file);
  if (!session) {
    session = await fetchJson<SessionState>("/api/imports/uploads", {
      method: "POST",
      body: JSON.stringify({
        filename: file.name,
        size_bytes: file.size,
        sha256,
        content_type: file.type,
        ...(assetId ? { asset_id: assetId } : {}),
      }),
    });
    if (session.deduped) {
      onProgress?.(1);
      return {
        asset_id: session.asset_id,
        import_id: session.import_id,
        suggested_match: session.suggested_match,
      };
    }
    rememberSession(file, session.upload_id);
  }

  const { upload_id: uploadId, chunk_size: chunkSize } = session;
  onProgress?.(file.size ? session.received_bytes / file.size : 0);

  let index = session.next_chunk;
  while (index * chunkSize < file.size) {
    throwIfAborted(signal);
    const start = index * chunkSize;
    const end = Math.min(start + chunkSize, file.size);
    const state = await sendChunk(uploadId, index, file.slice(start, end), signal);
    onProgress?.(file.size ? state.received_bytes / file.size : 1);
    index = state.next_chunk;
  }

  const done = await fetchJson<SessionState>(`/api/imports/uploads/${uploadId}/complete`, {
    method: "POST",
  });
  forgetSession(file);
  onProgress?.(1);
  return {
    asset_id: done.asset_id,
    import_id: done.import_id,
    suggested_match: done.suggested_match,
  };
}

// cancelChunkedUpload releases the server-side session.
export async function cancelChunkedUpload(file: File): Promise<void> {
  const uploadId = readResumeMap()[fileKey(file)];
  forgetSession(file);
  if (!uploadId) return;
  try {
    await fetchMaybeJson(`/api/imports/uploads/${uploadId}`, { method: "DELETE" });
  } catch {
    // the janitor is the backstop
  }
}

async function sendChunk(
  uploadId: number,
  index: number,
  blob: Blob,
  signal?: AbortSignal,
): Promise<SessionState> {
  let lastError: unknown;
  for (let attempt = 1; attempt <= MAX_CHUNK_ATTEMPTS; attempt++) {
    throwIfAborted(signal);
    try {
      const token = await getAccessToken();
      const res = await fetch(`/api/imports/uploads/${uploadId}/chunks/${index}`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/octet-stream",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: blob,
        signal,
      });
      if (res.ok) return (await res.json()) as SessionState;

      const body = await res.text();
      const err = new ApiError(res.status, body, chunkErrorMessage(res.status, body, index));
      if (!isRetryableStatus(res.status)) throw err;
      lastError = err;
    } catch (err) {
      if (err instanceof ApiError && !isRetryableStatus(err.status)) throw err;
      if (signal?.aborted) throw new UploadAbortedError();
      lastError = err;
    }
    await delay(RETRY_BASE_MS * 2 ** (attempt - 1), signal);
  }
  throw lastError instanceof Error ? lastError : new Error(`chunk ${index} failed`);
}

function isRetryableStatus(status: number): boolean {
  return status === 0 || status === 408 || status === 429 || status >= 500;
}

function chunkErrorMessage(status: number, body: string, index: number): string {
  try {
    const parsed = JSON.parse(body || "{}") as { error?: string };
    if (parsed.error) return `chunk ${index} failed (${status}): ${parsed.error}`;
  } catch {
    // non-JSON body — keep the generic message
  }
  return `chunk ${index} failed (${status})`;
}

function delay(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    function onAbort() {
      clearTimeout(timer);
      reject(new UploadAbortedError());
    }
    signal?.addEventListener("abort", onAbort);
  });
}

function throwIfAborted(signal?: AbortSignal) {
  if (signal?.aborted) throw new UploadAbortedError();
}
