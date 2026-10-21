import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "@/design-system/primitives/Button";

const btn = (name: string) => screen.getByRole("button", { name });
const classes = (name: string) => btn(name).className.split(/\s+/);

describe("Button", () => {
  it("defaults to the primary variant", () => {
    render(<Button>Go</Button>);
    expect(classes("Go")).toContain("bg-primary");
  });

  it("renders destructive as a soft red tint, not a fill", () => {
    render(<Button variant="destructive">Delete</Button>);
    const c = classes("Delete");

    expect(c).toContain("bg-destructive-soft");
    expect(c).toContain("text-destructive");
    expect(c).not.toContain("text-destructive-foreground");
    expect(c).not.toContain("bg-destructive");
  });

  it("renders the ghost variant", () => {
    render(<Button variant="ghost">Ghost</Button>);
    expect(classes("Ghost")).toContain("bg-transparent");
  });

  it("merges a caller className into the destructive variant", () => {
    render(
      <Button variant="destructive" className="w-full">
        Wide
      </Button>,
    );
    expect(classes("Wide")).toContain("w-full");
  });

  it("forwards native button props", () => {
    render(
      <Button variant="secondary" disabled>
        Off
      </Button>,
    );
    expect(btn("Off")).toBeDisabled();
  });
});
