import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ErrorState } from "@/design-system/compositions/ErrorState";

describe("ErrorState", () => {
  it("announces the failure as a status and retries on click", async () => {
    const onRetry = vi.fn();
    render(<ErrorState title="Couldn't load assets" onRetry={onRetry} />);
    expect(screen.getByRole("status")).toHaveTextContent("Couldn't load assets");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("disables Retry until the retry it started settles", async () => {
    let settle!: () => void;
    const onRetry = vi.fn(() => new Promise<void>((resolve) => (settle = resolve)));
    render(<ErrorState title="Couldn't load assets" onRetry={onRetry} />);
    const retry = screen.getByRole("button", { name: "Retry" });
    await userEvent.click(retry);
    expect(retry).toBeDisabled();
    settle();
    await vi.waitFor(() => expect(retry).toBeEnabled());
  });
});
