import { render } from "@testing-library/react";
import { Button, cn as packageCn } from "@telliott/shared-ui";
import { describe, expect, it } from "vitest";

import { cn } from "@/design-system/utils";

// A component from @telliott/shared-ui merges className with the `cn` bundled in
// the package, which knows the contract's 8 role sizes and none of damson's 19.
// These assertions pin what that costs. They will fail when shared-design changes
// the boundary — which is the point; the failure is the signal to delete them.
const ROLE = "text-mono";
const CONTRACT_ROLE = "text-ui";

describe("damson font-size roles across the package boundary", () => {
  it("does not resolve against the size a package component sets itself", () => {
    const { container } = render(<Button className={ROLE}>x</Button>);
    const classes = container.querySelector("button")!.className.split(/\s+/);

    // Both survive: the package's `cn` reads `text-mono` as a colour, so it never
    // conflicts with `text-ui`. Which one applies is stylesheet order.
    expect(classes).toContain(ROLE);
    expect(classes).toContain(CONTRACT_ROLE);
    expect(cn(CONTRACT_ROLE, ROLE)).toBe(ROLE);
  });

  it("is dropped when the same className also carries a colour", () => {
    expect(packageCn(ROLE, "text-muted-foreground")).toBe("text-muted-foreground");
    expect(cn(ROLE, "text-muted-foreground")).toBe(`${ROLE} text-muted-foreground`);
  });
});
