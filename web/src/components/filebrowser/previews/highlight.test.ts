import { describe, expect, it } from "vitest";

import { highlight } from "./highlight";

describe("highlight", () => {
  it("renders themed HTML for a known grammar", async () => {
    const html = await highlight("int x = 1;", "csharp");
    expect(html).toContain("<pre");
    expect(html).toContain("shiki");
    // Source text survives, HTML-escaped into the markup.
    expect(html).toContain("int");
  });

  it("escapes HTML metacharacters in the source (no raw markup injected)", async () => {
    const html = await highlight('var s = "<script>alert(1)</script>";', "typescript");
    expect(html).not.toContain("<script>");
    expect(html).not.toContain("</script>");
    expect(html).toContain("script"); // present as escaped text, not a tag
  });

  it("falls back to plaintext for an unknown grammar instead of throwing", async () => {
    const html = await highlight("anything at all", "no-such-lang");
    expect(html).toContain("<pre");
    expect(html).toContain("anything at all");
  });
});
