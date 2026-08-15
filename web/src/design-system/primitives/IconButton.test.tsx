import { render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it } from "vitest";

import { IconButton } from "./IconButton";

describe("IconButton", () => {
  it("forwards a ref to the underlying button", () => {
    const ref = createRef<HTMLButtonElement>();
    render(
      <IconButton ref={ref} aria-label="Close">
        <svg />
      </IconButton>,
    );

    expect(ref.current).toBeInstanceOf(HTMLButtonElement);
    expect(ref.current).toBe(screen.getByRole("button", { name: "Close" }));
  });

  it("defaults to type=button so it never submits a surrounding form", () => {
    render(<IconButton aria-label="Close" />);
    expect(screen.getByRole("button", { name: "Close" })).toHaveAttribute("type", "button");
  });

  it("sizes sm/md/lg as squares on Button's 28/32/36 scale", () => {
    render(
      <>
        <IconButton aria-label="s" size="sm" />
        <IconButton aria-label="m" size="md" />
        <IconButton aria-label="l" size="lg" />
      </>,
    );

    expect(screen.getByRole("button", { name: "s" })).toHaveClass("size-7");
    expect(screen.getByRole("button", { name: "m" })).toHaveClass("size-8");
    expect(screen.getByRole("button", { name: "l" })).toHaveClass("size-9");
  });

  it("marks pressed via data-pressed, and omits the attribute otherwise", () => {
    render(
      <>
        <IconButton aria-label="on" pressed />
        <IconButton aria-label="off" />
      </>,
    );

    expect(screen.getByRole("button", { name: "on" })).toHaveAttribute("data-pressed", "true");
    expect(screen.getByRole("button", { name: "off" })).not.toHaveAttribute("data-pressed");
  });

  it("defaults to ghost, not to Button's primary", () => {
    render(<IconButton aria-label="Close" />);
    const button = screen.getByRole("button", { name: "Close" });

    expect(button).toHaveClass("bg-transparent");
    expect(button).not.toHaveClass("bg-primary");
  });
});
