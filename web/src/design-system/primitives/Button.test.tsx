import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "@/design-system/primitives/Button";

const btn = (name: string) => screen.getByRole("button", { name });
const classes = (name: string) => btn(name).className.split(/\s+/);

describe("Button", () => {
  it("defaults to the registry's primary variant", () => {
    render(<Button>Go</Button>);
    expect(classes("Go")).toContain("bg-primary");
  });

  it("keeps damson's soft-red destructive instead of the registry's filled one", () => {
    render(<Button variant="destructive">Delete</Button>);
    const c = classes("Delete");

    expect(c).toContain("bg-destructive-soft");
    expect(c).toContain("text-destructive");
    // The registry's destructive is filled: bg-destructive with an inverted
    // foreground. Neither may survive the wrapper.
    expect(c).not.toContain("text-destructive-foreground");
    expect(c).not.toContain("bg-destructive");
  });

  it("passes other variants straight through to the registry", () => {
    render(<Button variant="ghost">Ghost</Button>);
    expect(classes("Ghost")).toContain("bg-transparent");
  });

  it("merges a caller className over the destructive override", () => {
    render(
      <Button variant="destructive" className="w-full">
        Wide
      </Button>,
    );
    expect(classes("Wide")).toContain("w-full");
  });

  it("forwards registry props", () => {
    render(
      <Button variant="secondary" disabled>
        Off
      </Button>,
    );
    expect(btn("Off")).toBeDisabled();
  });
});
