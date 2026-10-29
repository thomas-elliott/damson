import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { DetailPanel } from "./DetailPanel";

// Opened from a card, as the library grid does, and closed from the panel.
function Opener() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Castle Kit card
      </button>
      <button type="button" onClick={() => setOpen(false)}>
        Library
      </button>
      {open && (
        <DetailPanel title="Castle Kit" onClose={() => setOpen(false)}>
          body
        </DetailPanel>
      )}
    </>
  );
}

describe("DetailPanel overlay", () => {
  it("is a region named by its title heading", async () => {
    render(<Opener />);
    await userEvent.click(screen.getByRole("button", { name: "Castle Kit card" }));
    const panel = screen.getByRole("complementary", { name: "Castle Kit" });
    expect(panel).toContainElement(screen.getByRole("heading", { level: 2, name: "Castle Kit" }));
  });

  it("moves focus to its heading on open and back to the opener on close", async () => {
    render(<Opener />);
    const card = screen.getByRole("button", { name: "Castle Kit card" });
    await userEvent.click(card);
    expect(screen.getByRole("heading", { name: "Castle Kit" })).toHaveFocus();
    await userEvent.click(screen.getByRole("button", { name: "Close details" }));
    expect(card).toHaveFocus();
  });

  it("leaves focus where the user put it when the panel closes from outside", async () => {
    render(<Opener />);
    await userEvent.click(screen.getByRole("button", { name: "Castle Kit card" }));
    const outside = screen.getByRole("button", { name: "Library" });
    await userEvent.click(outside);
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(outside).toHaveFocus();
  });
});
