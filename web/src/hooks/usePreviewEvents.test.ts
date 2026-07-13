import { describe, expect, it } from "vitest";

import { foldPreviewEvent } from "./usePreviewEvents";
import type { PreviewRow } from "@/hooks/useDataQueries";

function row(overrides: Partial<PreviewRow>): PreviewRow {
  return {
    version_id: 1,
    asset_id: 1,
    asset_name: "pack",
    status: "processing",
    total: 3,
    done: 1,
    failed: 0,
    processing: 1,
    queued: 1,
    unsupported: 1,
    created_at: "2026-06-18T00:00:00Z",
    updated_at: "2026-06-18T00:00:00Z",
    ...overrides,
  };
}

describe("foldPreviewEvent", () => {
  it("inserts an in-flight rollup, newest first, replacing any prior for the same version", () => {
    const cached = { previews: [row({ version_id: 2 })], total: 1 };
    const ev = row({ version_id: 1, processing: 1, queued: 0 });
    const next = foldPreviewEvent(cached, ev)!;
    expect(next.total).toBe(2);
    expect(next.previews[0].version_id).toBe(1); // unshifted to front
  });

  it("updates an existing version in place rather than duplicating it", () => {
    const cached = {
      previews: [row({ version_id: 1, done: 1, processing: 2, queued: 0 })],
      total: 1,
    };
    const ev = row({ version_id: 1, done: 3, processing: 0, queued: 1 });
    const next = foldPreviewEvent(cached, ev)!;
    expect(next.total).toBe(1);
    expect(next.previews[0].done).toBe(3);
  });

  it("moves a terminal rollup (rendered something, nothing pending) to the finished tail", () => {
    const cached = { previews: [row({ version_id: 1 })], total: 1 };
    const ev = row({ version_id: 1, status: "done", done: 2, failed: 1, processing: 0, queued: 0 });
    const next = foldPreviewEvent(cached, ev)!;
    expect(next.total).toBe(0);
    expect(next.previews).toHaveLength(0);
    expect(next.finished).toHaveLength(1);
    expect(next.finished![0].version_id).toBe(1);
  });

  it("does not retain a purely-unsupported terminal rollup (nothing ever rendered)", () => {
    const cached = { previews: [row({ version_id: 1 })], total: 1 };
    const ev = row({
      version_id: 1,
      status: "done",
      done: 0,
      failed: 0,
      processing: 0,
      queued: 0,
      unsupported: 3,
    });
    const next = foldPreviewEvent(cached, ev)!;
    expect(next.previews).toHaveLength(0);
    expect(next.finished ?? []).toHaveLength(0);
  });

  it("leaves an undefined cache untouched", () => {
    expect(foldPreviewEvent(undefined, row({}))).toBeUndefined();
  });
});
