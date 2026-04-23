import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";

function Hello() {
  return <p>hello</p>;
}

describe("SPA sanity", () => {
  it("renders", () => {
    const { getByText } = render(<Hello />);
    expect(getByText("hello")).toBeInTheDocument();
  });
});
