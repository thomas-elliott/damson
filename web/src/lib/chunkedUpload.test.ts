import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/api";

import { CHUNKED_UPLOAD_THRESHOLD, cancelChunkedUpload, uploadFileChunked } from "./chunkedUpload";

vi.mock("@/lib/token", () => ({ getAccessToken: () => Promise.resolve("test-token") }));

const DIGEST = "a".repeat(64);

// A worker stub: the real one loads WASM, which jsdom has no way to run. What
// matters here is the chunk loop, not the digest, and the server re-verifies
// the hash anyway.
class FakeWorker {
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: ((e: ErrorEvent) => void) | null = null;
  postMessage() {
    queueMicrotask(() => {
      this.onmessage?.({ data: { type: "progress", read: 1 } } as MessageEvent);
      this.onmessage?.({ data: { type: "done", sha256: DIGEST } } as MessageEvent);
    });
  }
  terminate() {}
}

interface Call {
  url: string;
  method: string;
  bodySize: number;
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function sessionBody(over: Record<string, unknown> = {}) {
  return {
    upload_id: 7,
    status: "open",
    chunk_size: 8,
    size_bytes: 20,
    received_bytes: 0,
    next_chunk: 0,
    total_chunks: 3,
    deduped: false,
    ...over,
  };
}

let calls: Call[];

beforeEach(() => {
  calls = [];
  vi.stubGlobal("Worker", FakeWorker);
  window.localStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

// installFetch records every request and answers from the handler, defaulting
// to the happy chunked path.
function installFetch(handler?: (call: Call, attempt: number) => Response | undefined) {
  const attempts = new Map<string, number>();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      const body = init?.body;
      const bodySize = body instanceof Blob ? body.size : 0;
      const call = { url, method, bodySize };
      calls.push(call);

      const n = (attempts.get(url + method) ?? 0) + 1;
      attempts.set(url + method, n);
      const override = handler?.(call, n);
      if (override) return override;

      if (url.endsWith("/api/imports/uploads") && method === "POST") {
        return jsonResponse(201, sessionBody());
      }
      if (url.includes("/chunks/")) {
        const index = Number(url.slice(url.lastIndexOf("/") + 1));
        const received = Math.min((index + 1) * 8, 20);
        return jsonResponse(200, sessionBody({ received_bytes: received, next_chunk: index + 1 }));
      }
      if (url.endsWith("/complete")) {
        return jsonResponse(202, sessionBody({ status: "completed", asset_id: 3, import_id: 4 }));
      }
      return jsonResponse(404, { error: "unexpected request" });
    }),
  );
}

function bigFile(size = 20): File {
  const f = new File(["x".repeat(size)], "Big Pack.unitypackage");
  Object.defineProperty(f, "size", { value: size });
  return f;
}

function chunkCalls(): Call[] {
  return calls.filter((c) => c.url.includes("/chunks/"));
}

describe("chunked upload", () => {
  it("sends the file one chunk at a time and completes", async () => {
    installFetch();
    const phases: string[] = [];
    const progress: number[] = [];

    const resp = await uploadFileChunked({
      file: bigFile(),
      onPhase: (p) => phases.push(p),
      onProgress: (p) => progress.push(p),
    });

    expect(phases).toEqual(["hashing", "uploading"]);
    expect(chunkCalls().map((c) => c.url.slice(c.url.lastIndexOf("/") + 1))).toEqual([
      "0",
      "1",
      "2",
    ]);
    // 8 + 8 + 4: the last chunk is the remainder, not a padded full chunk.
    expect(chunkCalls().map((c) => c.bodySize)).toEqual([8, 8, 4]);
    expect(calls.at(-1)?.url).toContain("/complete");
    expect(resp).toEqual({ asset_id: 3, import_id: 4, suggested_match: undefined });
    expect(progress.at(-1)).toBe(1);
  });

  it("transfers nothing when the server already holds the content", async () => {
    installFetch((call) => {
      if (call.url.endsWith("/api/imports/uploads") && call.method === "POST") {
        return jsonResponse(
          202,
          sessionBody({ upload_id: 0, deduped: true, asset_id: 9, import_id: 11 }),
        );
      }
      return undefined;
    });

    const resp = await uploadFileChunked({ file: bigFile() });

    expect(resp.asset_id).toBe(9);
    expect(chunkCalls()).toHaveLength(0);
    expect(calls.some((c) => c.url.includes("/complete"))).toBe(false);
  });

  it("resumes a stored session instead of re-sending accepted chunks", async () => {
    installFetch((call) => {
      if (call.url.endsWith("/api/imports/uploads/7") && call.method === "GET") {
        return jsonResponse(200, sessionBody({ received_bytes: 16, next_chunk: 2 }));
      }
      if (call.url.endsWith("/api/imports/uploads") && call.method === "POST") {
        throw new Error("a resumable session must not open a second one");
      }
      return undefined;
    });
    window.localStorage.setItem(
      "damson.uploads.resumable",
      JSON.stringify({ "Big Pack.unitypackage:20:0": 7 }),
    );
    const file = bigFile();
    Object.defineProperty(file, "lastModified", { value: 0 });

    await uploadFileChunked({ file });

    expect(chunkCalls().map((c) => c.url.slice(c.url.lastIndexOf("/") + 1))).toEqual(["2"]);
    expect(chunkCalls()[0].bodySize).toBe(4);
  });

  it("starts fresh when the stored session is no longer open", async () => {
    installFetch((call) => {
      if (call.url.endsWith("/api/imports/uploads/7") && call.method === "GET") {
        return jsonResponse(200, sessionBody({ status: "expired" }));
      }
      return undefined;
    });
    window.localStorage.setItem(
      "damson.uploads.resumable",
      JSON.stringify({ "Big Pack.unitypackage:20:0": 7 }),
    );
    const file = bigFile();
    Object.defineProperty(file, "lastModified", { value: 0 });

    await uploadFileChunked({ file });

    expect(calls.some((c) => c.url.endsWith("/api/imports/uploads") && c.method === "POST")).toBe(
      true,
    );
    expect(chunkCalls()).toHaveLength(3);
    expect(window.localStorage.getItem("damson.uploads.resumable")).toBe("{}");
  });

  it("retries a chunk the server failed to accept", async () => {
    installFetch((call, attempt) => {
      if (call.url.endsWith("/chunks/1") && attempt === 1) {
        return jsonResponse(503, { error: "backend unavailable" });
      }
      return undefined;
    });

    await uploadFileChunked({ file: bigFile() });

    const ones = chunkCalls().filter((c) => c.url.endsWith("/chunks/1"));
    expect(ones).toHaveLength(2);
    expect(calls.at(-1)?.url).toContain("/complete");
  });

  it("gives up immediately on a chunk the server rejects outright", async () => {
    installFetch((call) => {
      if (call.url.endsWith("/chunks/0")) {
        return jsonResponse(400, { error: "chunk 0 must be 8 bytes, got 4" });
      }
      return undefined;
    });

    await expect(uploadFileChunked({ file: bigFile() })).rejects.toBeInstanceOf(ApiError);
    // One attempt only: resending a body the server called malformed is futile.
    expect(chunkCalls()).toHaveLength(1);
  });

  it("stops on abort and does not complete the upload", async () => {
    const controller = new AbortController();
    installFetch((call) => {
      if (call.url.endsWith("/chunks/0")) controller.abort();
      return undefined;
    });

    await expect(
      uploadFileChunked({ file: bigFile(), signal: controller.signal }),
    ).rejects.toThrow();
    expect(calls.some((c) => c.url.includes("/complete"))).toBe(false);
  });

  it("releases the server session when a cancelled upload is cleaned up", async () => {
    installFetch();
    window.localStorage.setItem(
      "damson.uploads.resumable",
      JSON.stringify({ "Big Pack.unitypackage:20:0": 7 }),
    );
    const file = bigFile();
    Object.defineProperty(file, "lastModified", { value: 0 });

    await cancelChunkedUpload(file);

    const del = calls.find((c) => c.method === "DELETE");
    expect(del?.url).toContain("/api/imports/uploads/7");
    expect(window.localStorage.getItem("damson.uploads.resumable")).toBe("{}");
  });

  it("puts the threshold above what a single request handles comfortably", () => {
    expect(CHUNKED_UPLOAD_THRESHOLD).toBeGreaterThanOrEqual(64 * 1024 * 1024);
  });
});
