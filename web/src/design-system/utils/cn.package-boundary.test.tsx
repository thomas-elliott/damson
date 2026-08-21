import { render } from "@testing-library/react";
import { Button, cn as packageCn } from "@telliott/shared-ui";
import { describe, expect, it } from "vitest";

import { cn } from "@/design-system/utils";

// A component from @telliott/shared-ui merges className with the `cn` bundled in
// the package, which knows the contract's names and none of damson's. Since
// 0.2.0 an unrecognised `text-*` passes through instead of being read as a
// conflicting colour, so nothing is dropped — but nothing is resolved either.
// These assertions pin what that leaves.
const ROLE = "text-mono";
const CONTRACT_ROLE = "text-ui";
const DAMSON_COLOUR = "text-unknown";

describe("damson tokens across the package boundary", () => {
  it("leaves a size the component also sets to the cascade", () => {
    const { container } = render(<Button className={ROLE}>x</Button>);
    const classes = container.querySelector("button")!.className.split(/\s+/);

    expect(classes).toContain(ROLE);
    expect(classes).toContain(CONTRACT_ROLE);
    expect(cn(CONTRACT_ROLE, ROLE)).toBe(ROLE);
  });

  it("keeps a role alongside a colour", () => {
    expect(packageCn(ROLE, "text-muted-foreground")).toBe(`${ROLE} text-muted-foreground`);
  });

  // The cost of closing the colour group: damson's own colour tokens are not in
  // the contract's tokens.css, so the package's `cn` cannot dedupe them.
  it("duplicates a damson colour instead of resolving it", () => {
    expect(packageCn(DAMSON_COLOUR, "text-muted-foreground")).toBe(
      `${DAMSON_COLOUR} text-muted-foreground`,
    );
    expect(packageCn("text-foreground", "text-muted-foreground")).toBe("text-muted-foreground");
    expect(cn(DAMSON_COLOUR, "text-muted-foreground")).toBe("text-muted-foreground");
  });
});
