import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Input } from "./Input";
import { Textarea } from "./Textarea";

// Screen readers learn a control is invalid from aria-invalid on the control
// itself; a data attribute on a wrapper says nothing to them.
describe("invalid Input and Textarea", () => {
  it.each([
    ["Input", <Input key="i" aria-label="Name" invalid />],
    ["Textarea", <Textarea key="t" aria-label="Notes" invalid />],
  ])("%s marked invalid is announced as invalid", (_, control) => {
    render(control);
    expect(screen.getByRole("textbox")).toBeInvalid();
  });

  it.each([
    ["Input", <Input key="i" aria-label="Name" />],
    ["Textarea", <Textarea key="t" aria-label="Notes" />],
  ])("%s not marked invalid is not announced as invalid", (_, control) => {
    render(control);
    expect(screen.getByRole("textbox")).toBeValid();
  });
});
