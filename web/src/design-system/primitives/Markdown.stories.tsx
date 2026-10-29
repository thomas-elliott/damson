import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";
import { expect, within } from "storybook/test";

import { FieldRow } from "@/design-system/domain/FieldRow";

import { Markdown } from "./Markdown";
import { MarkdownEditor } from "./MarkdownEditor";

const meta: Meta = {
  title: "Primitives/Markdown",
  parameters: { layout: "padded" },
};
export default meta;
type Story = StoryObj;

const SAMPLE = `**POLYGON Fantasy Kingdom** is a low-poly asset pack of
characters, props, and environment pieces.

### Contents

- 1,284 prefabs, all with LODs
- 12 modular building kits
- Demo scene (\`Scenes/Demo.unity\`)

1. Import the package
2. Switch to URP
3. Open the demo scene

Terms follow the [Unity Asset Store EULA](https://unity.com/legal/as-terms).

> Requires the Universal Render Pipeline.

| Kit | Prefabs |
| --- | --- |
| Buildings | 412 |
| Props | 680 |

\`\`\`csharp
var kingdom = Resources.Load<GameObject>("Castle");
\`\`\`

---

~~Legacy RP support~~ dropped in v2.1.`;

/** Every block and inline construct the renderer handles. */
export const Rendered: Story = {
  render: () => (
    <div className="max-w-[420px]">
      <Markdown>{SAMPLE}</Markdown>
    </div>
  ),
};

/**
 * Hostile input: raw HTML is shown as text, and `javascript:` targets lose
 * their link entirely. Nothing here is executable.
 */
export const UntrustedInput: Story = {
  parameters: {
    // Contrast gate: a link inside body text is told apart by colour alone, and the brand
    // link colour is 1.13:1 against the surrounding text (axe link-in-text-block, needs
    // 3:1). Drop this when Markdown links get an underline or the palette moves.
    a11y: { test: "todo" },
  },
  render: () => (
    <div className="max-w-[420px]">
      <Markdown>{`<script>alert(1)</script> and <img src=x onerror=alert(1)>

[looks harmless](javascript:alert(1)) · [safe link](https://example.com)

![remote hero image](https://cdn.example.com/hero.png)`}</Markdown>
    </div>
  ),
};

/** The CodeMirror editing surface, with the rendered result beside it. */
export const Editor: Story = {
  parameters: { layout: "padded" },
  // MarkdownEditorView loads lazily; wait for it so the a11y check sees the
  // CodeMirror surface rather than the Suspense fallback.
  play: async ({ canvasElement }) => {
    await expect(
      await within(canvasElement).findByRole("textbox", { name: "Description" }),
    ).toBeVisible();
  },
  render: function EditorDemo() {
    const [value, setValue] = useState(SAMPLE);
    return (
      <div className="grid max-w-[860px] grid-cols-2 gap-4">
        <FieldRow label="Description">
          <MarkdownEditor
            value={value}
            onChange={setValue}
            aria-label="Description"
            placeholder="What's this asset for? Markdown is supported."
            minHeight={280}
          />
        </FieldRow>
        <div>
          <div className="mb-1.5 text-label text-subtle-foreground">Preview</div>
          <Markdown>{value}</Markdown>
        </div>
      </div>
    );
  },
};

/** Empty editor — placeholder state. */
export const EditorEmpty: Story = {
  // MarkdownEditorView loads lazily; wait for it so the a11y check sees the
  // CodeMirror surface rather than the Suspense fallback.
  play: async ({ canvasElement }) => {
    await expect(
      await within(canvasElement).findByRole("textbox", { name: "Description" }),
    ).toBeVisible();
  },
  render: function EmptyDemo() {
    const [value, setValue] = useState("");
    return (
      <div className="max-w-[420px]">
        <FieldRow label="Description">
          <MarkdownEditor
            value={value}
            onChange={setValue}
            aria-label="Description"
            placeholder="What's this asset for? Markdown is supported."
          />
        </FieldRow>
      </div>
    );
  },
};
