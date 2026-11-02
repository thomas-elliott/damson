import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@/design-system";

import { ServerAdminSection } from "./ServerAdminSection";

const fetchJsonMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return { ...actual, fetchJson: (...args: unknown[]) => fetchJsonMock(...args) };
});

const SETTINGS = { max_upload_bytes: 1024 };
const USER = { id: 1, email: "thomas@example.com", display_name: "Thomas", role: "sysadmin" };

// Each admin endpoint fails while its flag is set.
let failing: { settings: boolean; users: boolean };

beforeEach(() => {
  failing = { settings: false, users: false };
  fetchJsonMock.mockReset();
  fetchJsonMock.mockImplementation((url: string) => {
    if (url === "/api/admin/settings") {
      return failing.settings ? Promise.reject(new Error("boom")) : Promise.resolve(SETTINGS);
    }
    if (url === "/api/admin/users") {
      return failing.users ? Promise.reject(new Error("boom")) : Promise.resolve({ users: [USER] });
    }
    return Promise.reject(new Error(`unexpected ${url}`));
  });
});

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <TooltipProvider>
        <ServerAdminSection />
      </TooltipProvider>
    </QueryClientProvider>,
  );
}

describe("ServerAdminSection", () => {
  it("replaces the settings spinner with a Retry when settings fail, and recovers", async () => {
    failing.settings = true;
    const user = userEvent.setup();
    mount();

    expect(await screen.findByText("Couldn't load server settings")).toBeInTheDocument();
    // The users list still loads on its own.
    expect(await screen.findByText("Thomas")).toBeInTheDocument();
    expect(screen.queryByText("Loading…")).not.toBeInTheDocument();

    failing.settings = false;
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByDisplayValue("1024")).toBeInTheDocument();
  });

  it("offers a Retry rather than claiming there are no users when the list fails", async () => {
    failing.users = true;
    const user = userEvent.setup();
    mount();

    expect(await screen.findByText("Couldn't load users")).toBeInTheDocument();
    expect(screen.queryByText("No users yet")).not.toBeInTheDocument();
    expect(screen.queryByText(/Users \(0\)/)).not.toBeInTheDocument();

    failing.users = false;
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("Thomas")).toBeInTheDocument();
    expect(screen.getByText(/Users \(1\)/)).toBeInTheDocument();
  });
});
