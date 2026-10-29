import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LoadingState } from "@/design-system/compositions/LoadingState";

describe("LoadingState", () => {
  it("announces itself as a status with the canonical wording", () => {
    render(<LoadingState />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading…");
  });

  it("takes a caller's label", () => {
    render(<LoadingState variant="inline" label="Loading files…" />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading files…");
  });
});
