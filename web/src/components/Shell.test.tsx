import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@/design-system/primitives/Tooltip";
import { Shell } from "./Shell";
import { NotificationProvider } from "./notifications/NotificationProvider";

vi.mock("@/hooks/useDataQueries", () => ({
  useMe: () => ({ data: { id: 1, email: "a@b.c", display_name: "Ada B" }, loading: false }),
  useTriage: () => ({ total: 0 }),
}));
vi.mock("@/hooks/useTheme", () => ({
  useTheme: () => ({ mode: "dark", setTheme: vi.fn(), toggle: vi.fn() }),
}));

const STORAGE_KEY = "damson.notifications";

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <TooltipProvider>
          <NotificationProvider>
            <Shell main={<div>page</div>} />
          </NotificationProvider>
        </TooltipProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("Shell notification bell", () => {
  beforeEach(() => localStorage.clear());

  it("shows an unread-count badge and toggles the centre open", async () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify([
        {
          id: "n1",
          tone: "info",
          title: "one",
          createdAt: Date.now(),
          read: false,
          source: "toast",
        },
        {
          id: "n2",
          tone: "error",
          title: "two",
          createdAt: Date.now(),
          read: false,
          source: "toast",
        },
      ]),
    );
    mount();

    const bell = screen.getByRole("button", { name: "Notifications" });
    expect(bell).toHaveTextContent("2");

    expect(screen.queryByRole("region", { name: "Notification centre" })).toBeNull();
    await userEvent.click(bell);
    expect(screen.getByRole("region", { name: "Notification centre" })).toBeInTheDocument();
  });

  it("renders no badge when there is nothing unread or in flight", async () => {
    mount();
    const bell = screen.getByRole("button", { name: "Notifications" });
    await waitFor(() => expect(bell).not.toHaveTextContent("0"));
    expect(bell).toHaveTextContent("");
  });
});

describe("Shell rail account controls", () => {
  beforeEach(() => localStorage.clear());

  it("carries an account menu and no standalone theme toggle", () => {
    mount();
    expect(screen.getByRole("button", { name: "Account" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Switch to light theme" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Switch to dark theme" })).toBeNull();
  });
});
