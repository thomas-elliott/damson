import { QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { UploadResponse } from "@/api";

const uploadFileMock =
  vi.fn<
    (url: string, file: File, onProgress?: unknown, signal?: AbortSignal) => Promise<UploadResponse>
  >();

vi.mock("@/api", async () => {
  const actual = await vi.importActual<typeof import("@/api")>("@/api");
  return {
    ...actual,
    uploadFile: (...args: Parameters<typeof uploadFileMock>) => uploadFileMock(...args),
  };
});

import { queryClient } from "@/lib/queryClient";
import { NotificationProvider, useNotifications } from "./NotificationProvider";

function Harness({ files }: { files: File[] }) {
  const { startUpload, cancelAllUploads, uploads } = useNotifications();
  const active = uploads.filter((u) => !u.queued).length;
  return (
    <div>
      <button onClick={() => startUpload(files)}>go</button>
      <button onClick={() => cancelAllUploads()}>cancelAll</button>
      <output>active:{active}</output>
      <output>total:{uploads.length}</output>
    </div>
  );
}

function mount(files: File[]) {
  return render(
    <QueryClientProvider client={queryClient}>
      <NotificationProvider>
        <Harness files={files} />
      </NotificationProvider>
    </QueryClientProvider>,
  );
}

function makeFiles(n: number): File[] {
  return Array.from({ length: n }, (_, i) => new File([`data-${i}`], `f${i}.png`));
}

describe("upload concurrency queue", () => {
  let resolvers: Array<(v: UploadResponse) => void>;
  let signals: AbortSignal[];

  beforeEach(() => {
    localStorage.clear();
    resolvers = [];
    signals = [];
    uploadFileMock.mockReset();
    uploadFileMock.mockImplementation((_url, _file, _onProgress, signal) => {
      if (signal) signals.push(signal);
      return new Promise<UploadResponse>((resolve) => resolvers.push(resolve));
    });
  });

  it("runs at most 3 uploads at once and starts a queued one as a slot frees", async () => {
    mount(makeFiles(5));
    act(() => {
      fireEvent.click(screen.getByText("go"));
    });

    // Five dropped, but only three may be in flight — the rest queue.
    expect(uploadFileMock).toHaveBeenCalledTimes(3);
    expect(screen.getByText("active:3")).toBeInTheDocument();
    expect(screen.getByText("total:5")).toBeInTheDocument();

    // Completing one frees a slot → the fourth starts.
    await act(async () => {
      resolvers[0]({});
      await Promise.resolve();
    });
    expect(uploadFileMock).toHaveBeenCalledTimes(4);
    expect(screen.getByText("total:4")).toBeInTheDocument();
  });

  it("cancelAllUploads aborts every upload and clears the list", async () => {
    mount(makeFiles(2));
    act(() => {
      fireEvent.click(screen.getByText("go"));
    });
    expect(signals.every((s) => !s.aborted)).toBe(true);

    act(() => {
      fireEvent.click(screen.getByText("cancelAll"));
    });
    expect(signals.every((s) => s.aborted)).toBe(true);
    expect(screen.getByText("total:0")).toBeInTheDocument();
  });
});
