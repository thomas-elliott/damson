import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UploadAbortedError } from "@/api";
import { NotificationProvider, useNotifications } from "./NotificationProvider";

const uploadFileMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return { ...actual, uploadFile: (...args: unknown[]) => uploadFileMock(...args) };
});

const STORAGE_KEY = "damson.notifications";

function render() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderHook(() => useNotifications(), {
    wrapper: ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>
        <NotificationProvider>{children}</NotificationProvider>
      </QueryClientProvider>
    ),
  });
}

const file = () => new File(["x"], "a.zip", { type: "application/zip" });

describe("NotificationProvider", () => {
  beforeEach(() => {
    localStorage.clear();
    uploadFileMock.mockReset();
  });
  afterEach(() => vi.clearAllMocks());

  it("turns a finished upload into a success notification and clears the upload", async () => {
    uploadFileMock.mockResolvedValue({});
    const { result } = render();

    act(() => result.current.startUpload([file()]));
    await waitFor(() => expect(result.current.notifications).toHaveLength(1));

    expect(result.current.uploads).toHaveLength(0);
    expect(result.current.inFlightCount).toBe(0);
    expect(result.current.notifications[0]).toMatchObject({
      tone: "success",
      source: "upload",
      read: false,
    });
    expect(result.current.unreadCount).toBe(1);
  });

  it("removes a cancelled upload silently — no notification", async () => {
    uploadFileMock.mockRejectedValue(new UploadAbortedError());
    const { result } = render();

    act(() => result.current.startUpload([file()]));
    await waitFor(() => expect(result.current.uploads).toHaveLength(0));

    expect(result.current.notifications).toHaveLength(0);
  });

  it("turns a failed upload into an error notification", async () => {
    uploadFileMock.mockRejectedValue(new Error("boom"));
    const { result } = render();

    act(() => result.current.startUpload([file()]));
    await waitFor(() => expect(result.current.notifications).toHaveLength(1));

    expect(result.current.notifications[0]).toMatchObject({ tone: "error", source: "upload" });
    expect(result.current.notifications[0].message).toBe("boom");
  });

  it("markRead moves an item out of the unread count without deleting it", async () => {
    const { result } = render();
    act(() => result.current.notify({ tone: "info", title: "hi" }));
    const id = result.current.notifications[0].id;

    act(() => result.current.markRead(id));

    expect(result.current.unreadCount).toBe(0);
    expect(result.current.notifications).toHaveLength(1);
    expect(result.current.notifications[0].read).toBe(true);
  });

  it("dismiss deletes; clearRead removes only read items", () => {
    const { result } = render();
    act(() => {
      result.current.notify({ tone: "info", title: "one" });
      result.current.notify({ tone: "info", title: "two" });
    });
    const [second, first] = result.current.notifications;

    act(() => result.current.dismiss(first.id));
    expect(result.current.notifications).toHaveLength(1);

    act(() => result.current.markRead(second.id));
    act(() => result.current.clearRead());
    expect(result.current.notifications).toHaveLength(0);
  });

  it("prunes read notifications older than 7 days on load, keeping fresh ones", () => {
    const eightDaysAgo = Date.now() - 8 * 24 * 60 * 60 * 1000;
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify([
        {
          id: "old",
          tone: "info",
          title: "old",
          createdAt: 0,
          read: true,
          readAt: eightDaysAgo,
          source: "toast",
        },
        {
          id: "fresh",
          tone: "info",
          title: "fresh",
          createdAt: 0,
          read: true,
          readAt: Date.now(),
          source: "toast",
        },
        { id: "unread", tone: "info", title: "unread", createdAt: 0, read: false, source: "toast" },
      ]),
    );

    const { result } = render();

    const ids = result.current.notifications.map((n) => n.id);
    expect(ids).toContain("fresh");
    expect(ids).toContain("unread");
    expect(ids).not.toContain("old");
  });
});
