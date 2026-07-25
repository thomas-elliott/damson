import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@/design-system/primitives/Tooltip";
import { UserMenu } from "./UserMenu";

const setTheme = vi.fn();
vi.mock("@/hooks/useTheme", () => ({
  useTheme: () => ({ mode: "dark", setTheme, toggle: vi.fn() }),
}));

const logout = vi.fn(() => Promise.resolve());
vi.mock("@/lib/logout", () => ({
  logout: () => logout(),
}));

function mount(displayName = "Thomas Elliott", email = "thomas@example.com") {
  return render(
    <MemoryRouter>
      <TooltipProvider>
        <UserMenu displayName={displayName} email={email} />
      </TooltipProvider>
    </MemoryRouter>,
  );
}

describe("UserMenu avatar initials", () => {
  const trigger = () => screen.getByRole("button", { name: "Account" });

  it("takes the first letter of the first two words", () => {
    mount("Thomas Elliott", "t@e.c");
    expect(trigger()).toHaveTextContent("TE");
  });

  it("falls back to two characters of a single-word name", () => {
    mount("Damson", "t@e.c");
    expect(trigger()).toHaveTextContent("DA");
  });

  it("falls back to the email when there is no display name", () => {
    mount("", "thomas@example.com");
    expect(trigger()).toHaveTextContent("TH");
  });

  it("never renders an empty avatar", () => {
    mount("", "");
    expect(trigger()).toHaveTextContent("?");
  });
});

describe("UserMenu", () => {
  it("keeps theme and sign-out behind the account trigger", async () => {
    mount();

    expect(screen.getByRole("button", { name: "Account" })).toHaveTextContent("TE");
    expect(screen.queryByRole("menuitem", { name: "Sign out" })).toBeNull();

    await userEvent.click(screen.getByRole("button", { name: "Account" }));

    expect(screen.getByText("thomas@example.com")).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Sign out" })).toBeInTheDocument();
    expect(screen.getByRole("menuitemradio", { name: "Dark" })).toBeChecked();
    expect(screen.getByRole("menuitemradio", { name: "Light" })).not.toBeChecked();
  });

  it("switches the theme from the menu", async () => {
    mount();
    await userEvent.click(screen.getByRole("button", { name: "Account" }));
    await userEvent.click(screen.getByRole("menuitemradio", { name: "Light" }));
    expect(setTheme).toHaveBeenCalledWith("light");
  });

  it("signs out from the menu", async () => {
    mount();
    await userEvent.click(screen.getByRole("button", { name: "Account" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Sign out" }));
    expect(logout).toHaveBeenCalled();
  });
});
