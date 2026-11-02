import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAssets, useLibrarySettings } from "@/hooks/useDataQueries";

vi.mock("@/lib/token", () => ({
  getAccessToken: vi.fn().mockResolvedValue(null),
}));

// The server's answer per offset: a status, and the assets on a 200.
let answers: Record<number, number>;

function respond(url: string): Response {
  const offset = Number(new URL(url, "http://x").searchParams.get("offset") ?? 0);
  const status = answers[offset] ?? 200;
  const body =
    status === 200
      ? { assets: [{ id: offset + 1, name: `asset ${offset + 1}` }], total: 100 }
      : { error: "internal error" };
  return new Response(JSON.stringify(body), { status });
}

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  answers = {};
  vi.stubGlobal(
    "fetch",
    vi.fn((url: string) => Promise.resolve(respond(url))),
  );
});
afterEach(() => vi.unstubAllGlobals());

describe("useAssets failed", () => {
  it("is set when the first load fails", async () => {
    answers[0] = 500;
    const { result } = renderHook(() => useAssets({}, true, { limit: 10, offset: 0 }), {
      wrapper,
    });
    await waitFor(() => expect(result.current.failed).toBe(true));
    expect(result.current.data).toEqual([]);
  });

  it("stays clear when a refetch fails over data already shown", async () => {
    const { result } = renderHook(() => useAssets({}, true, { limit: 10, offset: 0 }), {
      wrapper,
    });
    await waitFor(() => expect(result.current.data).toHaveLength(1));
    answers[0] = 500;
    await act(() => result.current.reload());
    await waitFor(() => expect(result.current.error).not.toBeNull());
    expect(result.current.failed).toBe(false);
    expect(result.current.data).toHaveLength(1);
  });

  it("is set when a page change fails, though the previous page was kept", async () => {
    answers[10] = 500;
    const { result, rerender } = renderHook(
      ({ offset }) => useAssets({}, true, { limit: 10, offset }),
      { wrapper, initialProps: { offset: 0 } },
    );
    await waitFor(() => expect(result.current.data).toHaveLength(1));
    rerender({ offset: 10 });
    await waitFor(() => expect(result.current.failed).toBe(true));
  });
});

describe("useLibrarySettings", () => {
  it("does not retry a failed load, so the grid falls back at once", async () => {
    const fetchSettings = vi.fn(() =>
      Promise.resolve(new Response(JSON.stringify({ error: "internal error" }), { status: 500 })),
    );
    vi.stubGlobal("fetch", fetchSettings);
    // The app's own default: one retry.
    const client = new QueryClient({ defaultOptions: { queries: { retry: 1, retryDelay: 0 } } });
    const { result } = renderHook(() => useLibrarySettings(1), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).not.toBeNull();
    expect(fetchSettings).toHaveBeenCalledTimes(1);
  });
});
