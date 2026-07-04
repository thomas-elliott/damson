import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError, isUploadAbortedError, uploadFile } from "@/api";

vi.mock("@/lib/token", () => ({
  getAccessToken: vi.fn().mockResolvedValue(null),
}));

class FakeXHR {
  static instances: FakeXHR[] = [];
  upload: { onprogress: ((e: ProgressEvent) => void) | null } = { onprogress: null };
  status = 0;
  responseText = "";
  aborted = false;
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onabort: (() => void) | null = null;

  constructor() {
    FakeXHR.instances.push(this);
  }
  open() {}
  setRequestHeader() {}
  send() {}
  abort() {
    this.aborted = true;
    this.onabort?.();
  }
}

async function latestXHR(): Promise<FakeXHR> {
  return vi.waitFor(() => {
    const xhr = FakeXHR.instances.at(-1);
    if (!xhr) throw new Error("no XHR yet");
    return xhr;
  });
}

const file = new File(["hello"], "hello.txt", { type: "text/plain" });

describe("uploadFile", () => {
  beforeEach(() => {
    FakeXHR.instances = [];
    vi.stubGlobal("XMLHttpRequest", FakeXHR);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("resolves with the parsed body on a 2xx response", async () => {
    const promise = uploadFile("/api/imports", file);
    const xhr = await latestXHR();
    xhr.status = 201;
    xhr.responseText = JSON.stringify({ id: 7 });
    xhr.onload?.();
    await expect(promise).resolves.toEqual({ id: 7 });
  });

  it("rejects with UploadAbortedError when the signal aborts mid-flight", async () => {
    const controller = new AbortController();
    const promise = uploadFile("/api/imports", file, undefined, controller.signal);
    const xhr = await latestXHR();
    controller.abort();
    expect(xhr.aborted).toBe(true);
    await expect(promise).rejects.toSatisfy(isUploadAbortedError);
  });

  it("rejects immediately with UploadAbortedError when the signal is already aborted", async () => {
    const controller = new AbortController();
    controller.abort();
    const promise = uploadFile("/api/imports", file, undefined, controller.signal);
    await expect(promise).rejects.toSatisfy(isUploadAbortedError);
    expect(FakeXHR.instances).toHaveLength(0);
  });

  it("rejects with ApiError — not UploadAbortedError — on a non-2xx response", async () => {
    const promise = uploadFile("/api/imports", file);
    const xhr = await latestXHR();
    xhr.status = 500;
    xhr.responseText = JSON.stringify({ error: "boom" });
    xhr.onload?.();
    const err = await promise.catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(isUploadAbortedError(err)).toBe(false);
  });
});
