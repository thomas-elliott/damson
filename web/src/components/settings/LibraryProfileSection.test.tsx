import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LibraryProfileSection } from "./LibraryProfileSection";

const fetchJsonMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return { ...actual, fetchJson: (...args: unknown[]) => fetchJsonMock(...args) };
});

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <LibraryProfileSection
          library={{ id: 7, name: "Main", description: "Shared kit", role: "admin" }}
          isAdmin
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("LibraryProfileSection", () => {
  afterEach(() => vi.clearAllMocks());

  it("keeps the saved description when only the name changes", async () => {
    fetchJsonMock.mockResolvedValue({});
    const user = userEvent.setup();
    mount();

    expect(screen.getByDisplayValue("Shared kit")).toBeInTheDocument();
    const name = screen.getByDisplayValue("Main");
    await user.clear(name);
    await user.type(name, "Studio");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(fetchJsonMock).toHaveBeenCalled());
    const [url, init] = fetchJsonMock.mock.calls[0];
    expect(url).toBe("/api/libraries/7");
    expect(JSON.parse(init.body)).toEqual({ name: "Studio", description: "Shared kit" });
  });
});
