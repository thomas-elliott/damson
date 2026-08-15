import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { EmptyState } from "@/design-system/compositions/EmptyState";

describe("EmptyState", () => {
  it("keeps the status role the registry component does not set", () => {
    render(<EmptyState title="Nothing here" />);
    expect(screen.getByRole("status")).toHaveTextContent("Nothing here");
  });

  it("still renders when title is omitted", () => {
    render(<EmptyState description="No results" />);
    expect(screen.getByRole("status")).toHaveTextContent("No results");
  });

  it("renders description and action", () => {
    render(<EmptyState title="Empty" description="Add one" action={<button>New</button>} />);
    expect(screen.getByRole("button", { name: "New" })).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Add one");
  });
});
