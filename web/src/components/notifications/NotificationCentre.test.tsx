import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UploadAbortedError } from "@/api";
import { NotificationProvider, useNotifications } from "./NotificationProvider";

const uploadFileMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return { ...actual, uploadFile: (...args: unknown[]) => uploadFileMock(...args) };
});

function Harness() {
  const { setOpen, startUpload } = useNotifications();
  return (
    <div>
      <button onClick={() => setOpen(true)}>open-centre</button>
      <button onClick={() => startUpload([new File(["x"], "big.zip")])}>do-upload</button>
    </div>
  );
}

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <NotificationProvider>
        <Harness />
      </NotificationProvider>
    </QueryClientProvider>,
  );
}

const STORAGE_KEY = "damson.notifications";

describe("NotificationCentre", () => {
  beforeEach(() => {
    localStorage.clear();
    uploadFileMock.mockReset();
  });
  afterEach(() => vi.clearAllMocks());

  it("is hidden until opened", async () => {
    mount();
    expect(screen.queryByRole("region", { name: "Notification centre" })).toBeNull();
    await userEvent.click(screen.getByText("open-centre"));
    expect(screen.getByRole("region", { name: "Notification centre" })).toBeInTheDocument();
  });

  it("marking a notification read removes it from the Unread tab", async () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify([
        {
          id: "n1",
          tone: "info",
          title: "Hello there",
          createdAt: Date.now(),
          read: false,
          source: "toast",
        },
      ]),
    );
    mount();
    await userEvent.click(screen.getByText("open-centre"));

    expect(screen.getByText("Hello there")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Mark read" }));

    await waitFor(() => expect(screen.queryByText("Hello there")).toBeNull());
    expect(screen.getByText("No new notifications")).toBeInTheDocument();
  });

  // The panel's heading already says Notifications; the tabs split it.
  it("names its tabs Unread and Read, and announces an empty tab", async () => {
    mount();
    await userEvent.click(screen.getByText("open-centre"));
    expect(screen.getByRole("tab", { name: "Unread" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Read" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: /Notifications/ })).toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent("No new notifications");
  });

  it("shows the Uploads tab only while an upload is in flight and cancels it", async () => {
    uploadFileMock.mockImplementation(
      (_url: string, _file: File, _p: unknown, signal: AbortSignal) =>
        new Promise((_resolve, reject) => {
          signal.addEventListener("abort", () => reject(new UploadAbortedError()));
        }),
    );
    mount();
    await userEvent.click(screen.getByText("open-centre"));
    expect(screen.queryByRole("tab", { name: /Uploads/ })).toBeNull();

    await userEvent.click(screen.getByText("do-upload"));

    const uploadsTab = await screen.findByRole("tab", { name: /Uploads/ });
    expect(uploadsTab).toBeInTheDocument();
    await userEvent.click(uploadsTab);
    const region = screen.getByRole("region", { name: "Notification centre" });
    await userEvent.click(within(region).getByRole("button", { name: "Cancel" }));

    await waitFor(() => expect(screen.queryByRole("tab", { name: /Uploads/ })).toBeNull());
  });
});
