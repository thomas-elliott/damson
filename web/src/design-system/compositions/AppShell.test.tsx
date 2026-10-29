import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";

import { AppShell } from "./AppShell";

const SIDEBAR_W_KEY = "damson.sidebar.w";

function mount(sidebar?: string) {
  return render(
    <AppShell
      rail={<div>rail</div>}
      sidebar={sidebar ? <div>{sidebar}</div> : undefined}
      main={<div>main</div>}
      resizableSidebar={!!sidebar}
    />,
  );
}

// The sidebar column's width; the grid template reads it as --sidebar-col.
function columns(): string {
  const shell = screen.getByText("rail").parentElement!.parentElement as HTMLElement;
  return `var(--rail-w) ${shell.style.getPropertyValue("--sidebar-col")} 1fr`;
}

describe("AppShell sidebar column", () => {
  beforeEach(() => localStorage.clear());

  it("collapses the sidebar column when no sidebar is passed", () => {
    mount();
    expect(columns()).toBe("var(--rail-w) 0px 1fr");
    expect(screen.queryByLabelText("Resize sidebar")).toBeNull();
  });

  it("opens the column at the persisted width", () => {
    localStorage.setItem(SIDEBAR_W_KEY, "312");
    mount("filters");
    expect(columns()).toBe("var(--rail-w) 312px 1fr");
    expect(screen.getByLabelText("Resize sidebar")).toBeInTheDocument();
  });

  it("restores the persisted width when a collapsed sidebar is opened in place", () => {
    localStorage.setItem(SIDEBAR_W_KEY, "312");
    const { rerender } = render(
      <AppShell
        rail={<div>rail</div>}
        sidebar={undefined}
        main={<div>main</div>}
        resizableSidebar={false}
      />,
    );
    expect(columns()).toBe("var(--rail-w) 0px 1fr");

    rerender(
      <AppShell
        rail={<div>rail</div>}
        sidebar={<div>filters</div>}
        main={<div>main</div>}
        resizableSidebar
      />,
    );
    expect(columns()).toBe("var(--rail-w) 312px 1fr");
    expect(localStorage.getItem(SIDEBAR_W_KEY)).toBe("312");
  });

  it("ignores a stored width outside the clamp bounds", () => {
    localStorage.setItem(SIDEBAR_W_KEY, "9000");
    mount("filters");
    expect(columns()).toBe("var(--rail-w) 248px 1fr");
  });

  it("lets an explicit sidebarWidth win over the stored value", () => {
    localStorage.setItem(SIDEBAR_W_KEY, "312");
    render(
      <AppShell
        rail={<div>rail</div>}
        sidebar={<div>filters</div>}
        main={<div>main</div>}
        sidebarWidth={200}
      />,
    );
    expect(columns()).toBe("var(--rail-w) 200px 1fr");
  });
});

describe("AppShell landmarks", () => {
  it("renders the main column as the main landmark", () => {
    mount();
    expect(screen.getByRole("main")).toHaveTextContent("main");
  });

  it("offers a skip link first, which moves focus to main", async () => {
    mount();
    await userEvent.tab();
    const skip = screen.getByRole("link", { name: "Skip to content" });
    expect(skip).toHaveFocus();
    await userEvent.click(skip);
    expect(screen.getByRole("main")).toHaveFocus();
  });
});
