import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Markdown } from "./Markdown";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("Markdown", () => {
  it("renders block structure", () => {
    const { container } = render(
      <Markdown>{`# Title

Some **bold** and *italic* and \`code\`.

- one
- two

> quoted`}</Markdown>,
    );
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Title");
    expect(container.querySelector("strong")).toHaveTextContent("bold");
    expect(container.querySelector("em")).toHaveTextContent("italic");
    expect(container.querySelector("code")).toHaveTextContent("code");
    expect(container.querySelectorAll("li")).toHaveLength(2);
    expect(container.querySelector("blockquote")).toHaveTextContent("quoted");
  });

  it("joins soft-wrapped lines into one paragraph", () => {
    const { container } = render(<Markdown>{"one\ntwo"}</Markdown>);
    expect(container.querySelectorAll("p")).toHaveLength(1);
    expect(container.querySelector("p")).toHaveTextContent("one two");
  });

  it("links http targets and drops unsafe schemes", () => {
    const { container } = render(
      <Markdown>{`[safe](https://example.com) [bad](javascript:alert(1)) [rel](/local)`}</Markdown>,
    );
    expect(screen.getByRole("link", { name: "safe" })).toHaveAttribute(
      "href",
      "https://example.com",
    );
    expect(screen.queryByRole("link", { name: "bad" })).toBeNull();
    expect(screen.queryByRole("link", { name: "rel" })).toBeNull();
    // Rejected links keep their label as plain text; the target is not shown.
    expect(container.textContent).toBe("safe bad rel");
  });

  it("renders images as links out rather than <img>", () => {
    const { container } = render(<Markdown>{`![hero](https://cdn.example.com/a.png)`}</Markdown>);
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByRole("link", { name: "hero" })).toHaveAttribute(
      "href",
      "https://cdn.example.com/a.png",
    );
  });

  it("emits raw HTML as text, never as markup", () => {
    const { container } = render(<Markdown>{`<script>alert(1)</script><b>not bold</b>`}</Markdown>);
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("b")).toBeNull();
    expect(container.textContent).toContain("<script>alert(1)</script>");
  });

  it("renders GFM tables with a well-formed body", () => {
    const { container } = render(
      <Markdown>{`| Kit | Prefabs |
| --- | --- |
| Buildings | 412 |
| Props | 680 |`}</Markdown>,
    );
    expect(container.querySelectorAll("thead th")).toHaveLength(2);
    expect(container.querySelectorAll("tbody tr")).toHaveLength(2);
    expect(container.querySelectorAll("table > tr")).toHaveLength(0);
  });

  it("renders without React warnings", () => {
    const error = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <Markdown>{`# Title

Text with a [link](https://example.com) and \`code\`.

| a | b |
| --- | --- |
| 1 | 2 |

- [x] done
- [ ] todo

\`\`\`ts
const x = 1;
\`\`\`

---`}</Markdown>,
    );
    expect(error).not.toHaveBeenCalled();
  });
});
