/// <reference lib="webworker" />
import { createSHA256 } from "hash-wasm";

export interface HashRequest {
  file: File;
  chunkBytes: number;
}

export type HashMessage =
  | { type: "progress"; read: number }
  | { type: "done"; sha256: string }
  | { type: "error"; message: string };

const ctx = self as unknown as DedicatedWorkerGlobalScope;

ctx.onmessage = (e: MessageEvent<HashRequest>) => {
  void run(e.data);
};

async function run({ file, chunkBytes }: HashRequest) {
  try {
    const hasher = await createSHA256();
    hasher.init();
    let read = 0;
    while (read < file.size) {
      const end = Math.min(read + chunkBytes, file.size);
      const slice = await file.slice(read, end).arrayBuffer();
      hasher.update(new Uint8Array(slice));
      read = end;
      ctx.postMessage({ type: "progress", read } satisfies HashMessage);
    }
    ctx.postMessage({ type: "done", sha256: hasher.digest("hex") } satisfies HashMessage);
  } catch (err) {
    ctx.postMessage({
      type: "error",
      message: err instanceof Error ? err.message : "hashing failed",
    } satisfies HashMessage);
  }
}
