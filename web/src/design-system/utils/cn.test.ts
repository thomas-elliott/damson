import { describe, expect, it } from "vitest";

import { cn } from "@/design-system/utils/cn";

const classes = (s: string) => s.split(/\s+/).filter(Boolean);

describe("cn", () => {
  it("keeps a contract role size beside a text colour", () => {
    for (const size of [
      "text-display",
      "text-title",
      "text-heading",
      "text-subhead",
      "text-field",
      "text-body",
      "text-ui",
      "text-label",
    ]) {
      const c = classes(cn(size, "text-muted-foreground"));
      expect(c, size).toContain(size);
      expect(c, size).toContain("text-muted-foreground");
    }
  });

  it("keeps a damson-local composed role beside a text colour", () => {
    for (const role of [
      "text-overline",
      "text-mono",
      "text-caption",
      "text-num",
      "text-table-row",
      "text-table-header",
      "text-list-row",
      "text-title-semibold",
      "text-heading-medium",
      "text-subhead-bold",
    ]) {
      const c = classes(cn(role, "text-muted-foreground"));
      expect(c, role).toContain(role);
      expect(c, role).toContain("text-muted-foreground");
    }
  });

  it("still treats two sizes as one group", () => {
    expect(cn("text-ui", "text-body")).toBe("text-body");
    expect(cn("text-mono", "text-caption")).toBe("text-caption");
    expect(cn("text-body", "text-mono")).toBe("text-mono");
    expect(cn("text-overline", "text-label")).toBe("text-label");
  });

  it("still treats two text colours as one group", () => {
    expect(cn("text-muted-foreground", "text-destructive")).toBe("text-destructive");
  });
});
