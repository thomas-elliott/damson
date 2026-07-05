import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { StorageCleanupSection } from "./StorageCleanupSection";

const fetchJsonMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return { ...actual, fetchJson: (...args: unknown[]) => fetchJsonMock(...args) };
});

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <StorageCleanupSection />
    </QueryClientProvider>,
  );
}

describe("StorageCleanupSection", () => {
  beforeEach(() => {
    fetchJsonMock.mockReset();
    fetchJsonMock.mockImplementation((url: string) => {
      if (url.endsWith("/scan")) {
        return Promise.resolve({
          min_age_seconds: 604800,
          total_count: 1,
          total_bytes: 6,
          candidates: [
            {
              prefix: "blobs",
              key: "abc123deadbeef",
              size: 6,
              last_modified: "2026-01-01T00:00:00Z",
              filename: "lost.bin",
            },
          ],
        });
      }
      if (url.endsWith("/delete")) {
        return Promise.resolve({ deleted_count: 1, freed_bytes: 6, skipped: [] });
      }
      return Promise.reject(new Error(`unexpected url ${url}`));
    });
  });
  afterEach(() => vi.clearAllMocks());

  it("scans, then deletes the selected orphan and prunes it from the list", async () => {
    const user = userEvent.setup();
    mount();

    // The section is collapsed by default — expand it, then scan.
    await user.click(screen.getByRole("button", { name: /Storage cleanup/ }));
    await user.click(screen.getByRole("button", { name: "Scan for orphans" }));

    // The orphan (with metadata) is listed and selected by default.
    await waitFor(() => expect(screen.getByText("abc123deadbeef")).toBeInTheDocument());
    expect(screen.getByText("lost.bin")).toBeInTheDocument();
    expect(screen.getByText(/1 orphan/)).toBeInTheDocument();

    // Delete → confirm.
    await user.click(screen.getByRole("button", { name: "Delete selected" }));
    await user.click(screen.getByRole("button", { name: "Delete" }));

    // The delete call carried exactly the selected key.
    await waitFor(() => {
      const deleteCall = fetchJsonMock.mock.calls.find((c) => String(c[0]).endsWith("/delete"));
      expect(deleteCall).toBeTruthy();
      expect(String((deleteCall![1] as RequestInit).body)).toContain("abc123deadbeef");
    });

    // The deleted orphan is pruned and a result notice is shown.
    await waitFor(() => expect(screen.queryByText("abc123deadbeef")).not.toBeInTheDocument());
    expect(screen.getByText(/Deleted 1/)).toBeInTheDocument();
  });
});
