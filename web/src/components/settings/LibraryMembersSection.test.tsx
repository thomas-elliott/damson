import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@/design-system";

import { LibraryMembersSection } from "./LibraryMembersSection";

const fetchJsonMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return { ...actual, fetchJson: (...args: unknown[]) => fetchJsonMock(...args) };
});

const MEMBER = {
  user_id: 1,
  email: "thomas@example.com",
  display_name: "Thomas",
  role: "admin",
  global_role: "user",
};

// The members list is the only request that goes through fetchJson.
function serve(members: () => Promise<unknown>) {
  fetchJsonMock.mockImplementation(members);
}

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <TooltipProvider>
        <LibraryMembersSection libraryId={7} isAdmin />
      </TooltipProvider>
    </QueryClientProvider>,
  );
}

describe("LibraryMembersSection", () => {
  beforeEach(() => {
    fetchJsonMock.mockReset();
  });

  it("says it is loading rather than that there are no members", async () => {
    serve(() => new Promise(() => {}));
    mount();

    expect(await screen.findByText("Loading…")).toBeInTheDocument();
    expect(screen.queryByText("No members yet")).not.toBeInTheDocument();
    expect(screen.queryByText(/Members \(0\)/)).not.toBeInTheDocument();
  });

  it("offers a Retry when the members fail to load, and recovers", async () => {
    let fail = true;
    serve(() =>
      fail ? Promise.reject(new Error("boom")) : Promise.resolve({ members: [MEMBER] }),
    );
    const user = userEvent.setup();
    mount();

    expect(await screen.findByText("Couldn't load members")).toBeInTheDocument();
    expect(screen.queryByText("No members yet")).not.toBeInTheDocument();

    fail = false;
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("Thomas")).toBeInTheDocument();
    expect(screen.getByText(/Members \(1\)/)).toBeInTheDocument();
  });

  it("says there are no members only once the list has loaded empty", async () => {
    serve(() => Promise.resolve({ members: [] }));
    mount();

    expect(await screen.findByText("No members yet")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load members")).not.toBeInTheDocument();
  });
});
