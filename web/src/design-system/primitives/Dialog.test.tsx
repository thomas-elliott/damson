import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { Button } from "./Button";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "./Dialog";

// Damson opens dialogs from state rather than through a DialogTrigger, as
// AddByLocationButton and New project do; the dialog may unmount on close.
function Opener({ unmount }: { unmount?: boolean }) {
  const [open, setOpen] = useState(false);
  const dialog = (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent aria-describedby={undefined}>
        <DialogTitle>New project</DialogTitle>
        <DialogFooter>
          <Button variant="ghost" onClick={() => setOpen(false)}>
            Cancel
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
  return (
    <>
      <Button onClick={() => setOpen(true)}>New project</Button>
      {unmount ? open && dialog : dialog}
    </>
  );
}

describe("Dialog", () => {
  it("returns focus to the opener on Escape", async () => {
    render(<Opener />);
    await userEvent.click(screen.getByRole("button", { name: "New project" }));
    await screen.findByRole("dialog");
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(screen.getByRole("button", { name: "New project" })).toHaveFocus());
  });

  it("returns focus to the opener on Cancel", async () => {
    render(<Opener />);
    await userEvent.click(screen.getByRole("button", { name: "New project" }));
    await userEvent.click(await screen.findByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "New project" })).toHaveFocus());
  });

  it("returns focus to the opener when the dialog unmounts on close", async () => {
    render(<Opener unmount />);
    await userEvent.click(screen.getByRole("button", { name: "New project" }));
    await screen.findByRole("dialog");
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(screen.getByRole("button", { name: "New project" })).toHaveFocus());
  });
});
