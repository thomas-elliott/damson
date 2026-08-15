import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "@/design-system/primitives/Button";

const cls = (name: string) => screen.getByRole("button", { name }).className;

describe("Button", () => {
  it("defaults to the registry's primary variant", () => {
    render(<Button>Go</Button>);
    expect(cls("Go")).toContain("bg-primary");
  });

  it("keeps damson's soft-red destructive instead of the registry's filled one", () => {
    render(<Button variant="destructive">Delete</Button>);
    const c = cls("Delete");

    expect(c).toContain("bg-destructive-soft");
    expect(c).toContain("text-destructive");
    // The registry's destructive is filled: bg-destructive with an inverted
    // foreground. Neither may survive the wrapper.
    expect(c).not.toContain("text-destructive-foreground");
    expect(c.split(/\s+/)).not.toContain("bg-destructive");
  });

  it("passes other variants straight through to the registry", () => {
    render(<Button variant="ghost">Ghost</Button>);
    expect(cls("Ghost")).toContain("bg-transparent");
  });

  it("merges a caller className over the destructive override", () => {
    render(
      <Button variant="destructive" className="w-full">
        Wide
      </Button>,
    );
    expect(cls("Wide")).toContain("w-full");
  });

  it("forwards registry props", () => {
    render(
      <Button variant="secondary" disabled>
        Off
      </Button>,
    );
    expect(screen.getByRole("button", { name: "Off" })).toBeDisabled();
  });
});
