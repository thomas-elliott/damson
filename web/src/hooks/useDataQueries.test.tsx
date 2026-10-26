import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAssets, useLibrarySettings, useMe } from "@/hooks/useDataQueries";
import { useSwitchCurrentLibrary } from "@/hooks/useMutations";

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

  it("never shows one library's settings under another after a switch", async () => {
    // The server's current library, and each library's density.
    let current = 1;
    const density: Record<number, string> = { 1: "small", 2: "large" };
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, init?: RequestInit) => {
        const path = new URL(url, "http://x").pathname;
        if (path === "/api/me/current-library") {
          current = JSON.parse(String(init?.body)).library_id;
          return new Response(JSON.stringify({ library_id: current }), { status: 200 });
        }
        if (path === "/api/me") {
          // /api/me answers after the settings would, as it can in the app.
          const me = { id: 1, current_library: { id: current } };
          await new Promise((r) => setTimeout(r, 20));
          return new Response(JSON.stringify(me), { status: 200 });
        }
        const settings = { library_id: current, grid_density_default: density[current] };
        return new Response(JSON.stringify(settings), { status: 200 });
      }),
    );
    // Every render: the library me names, and the library the settings are for.
    const seen: [number | undefined, number | undefined][] = [];
    const { result } = renderHook(
      () => {
        const me = useMe();
        const settings = useLibrarySettings(me.data?.current_library?.id);
        seen.push([me.data?.current_library?.id, settings.data?.library_id]);
        return { settings, switchTo: useSwitchCurrentLibrary() };
      },
      { wrapper },
    );
    const settle = (library: number) =>
      waitFor(() => expect(result.current.settings.data?.library_id).toBe(library));

    await settle(1);
    await act(() => result.current.switchTo.mutateAsync(2));
    await settle(2);
    await act(() => result.current.switchTo.mutateAsync(1));
    await settle(1);
    expect(result.current.settings.data?.grid_density_default).toBe("small");

    const mismatched = seen.filter(([me, settings]) => settings != null && settings !== me);
    expect(mismatched).toEqual([]);
  });
});
