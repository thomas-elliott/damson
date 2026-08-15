import { render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it } from "vitest";

import { IconButton } from "@/design-system/primitives/IconButton";

const btn = (name: string) => screen.getByRole("button", { name });
const classes = (name: string) => btn(name).className.split(/\s+/);

describe("IconButton", () => {
  it("forwards a ref to the underlying button", () => {
    const ref = createRef<HTMLButtonElement>();
    render(
      <IconButton ref={ref} aria-label="Close">
        <svg />
      </IconButton>,
    );

    expect(ref.current).toBeInstanceOf(HTMLButtonElement);
    expect(ref.current).toBe(btn("Close"));
  });

  it("defaults to type=button so it never submits a surrounding form", () => {
    render(<IconButton aria-label="Close" />);
    expect(btn("Close")).toHaveAttribute("type", "button");
  });

  it("sizes sm/md/lg as squares on Button's 28/32/36 scale", () => {
    render(
      <>
        <IconButton aria-label="s" size="sm" />
        <IconButton aria-label="m" size="md" />
        <IconButton aria-label="l" size="lg" />
      </>,
    );

    expect(classes("s")).toContain("size-7");
    expect(classes("m")).toContain("size-8");
    expect(classes("l")).toContain("size-9");
  });

  it("marks pressed via data-pressed, and omits the attribute otherwise", () => {
    render(
      <>
        <IconButton aria-label="on" pressed />
        <IconButton aria-label="off" />
      </>,
    );

    expect(btn("on")).toHaveAttribute("data-pressed", "true");
    expect(btn("off")).not.toHaveAttribute("data-pressed");
  });

  it("defaults to ghost, not to Button's primary", () => {
    render(<IconButton aria-label="Close" />);

    expect(classes("Close")).toContain("bg-transparent");
    expect(classes("Close")).not.toContain("bg-primary");
  });
});
