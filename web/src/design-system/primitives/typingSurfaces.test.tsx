import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Input } from "@/design-system/primitives/Input";
import { SearchInput } from "@/design-system/primitives/SearchInput";
import { Textarea } from "@/design-system/primitives/Textarea";

const BELOW_FLOOR = ["text-ui", "text-label", "text-small", "text-sm", "text-xs", "text-base"];

const classesOf = (el: Element | null) => (el?.className ?? "").split(/\s+/).filter(Boolean);

describe("typing surfaces hold the 16px floor", () => {
  it("Input renders its field at text-field", () => {
    render(<Input aria-label="Name" />);
    const c = classesOf(screen.getByLabelText("Name"));

    expect(c).toContain("text-field");
    for (const size of BELOW_FLOOR) expect(c, size).not.toContain(size);
  });

  it("Textarea renders at text-field", () => {
    render(<Textarea aria-label="Notes" />);
    const c = classesOf(screen.getByLabelText("Notes"));

    expect(c).toContain("text-field");
    for (const size of BELOW_FLOOR) expect(c, size).not.toContain(size);
  });

  it("SearchInput inherits the floor from Input", () => {
    render(<SearchInput aria-label="Search assets" kbd="/" />);
    const c = classesOf(screen.getByLabelText("Search assets"));

    expect(c).toContain("text-field");
    for (const size of BELOW_FLOOR) expect(c, size).not.toContain(size);
  });

  it("Input's wrapper does not reintroduce a smaller size around the field", () => {
    const { container } = render(<Input aria-label="Email" />);
    const wrapper = container.firstElementChild;
    const c = classesOf(wrapper);

    expect(c).toContain("text-field");
    for (const size of BELOW_FLOOR) expect(c, size).not.toContain(size);
  });
});
