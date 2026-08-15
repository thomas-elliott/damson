import type { Meta, StoryObj } from "@storybook/react";
import { expect, within } from "storybook/test";

const meta: Meta = {
  title: "Tokens/Scoped theme",
  parameters: { layout: "padded" },
  globals: { theme: "light" },
};
export default meta;
type Story = StoryObj;

function Panel({ label }: { label: string }) {
  return (
    <div data-testid={label} className="bg-card text-foreground rounded-md border p-4">
      <p className="text-ui font-medium">{label}</p>
      <p className="text-subtle-foreground text-label">Tier three text</p>
    </div>
  );
}

export const DarkIslandInsideLight: Story = {
  render: () => (
    <div className="flex flex-col gap-4">
      <Panel label="root" />
      <div className="dark rounded-lg p-4">
        <Panel label="island" />
      </div>
    </div>
  ),
  play: async ({ canvasElement }) => {
    const c = within(canvasElement);
    const root = await c.findByTestId("root");
    const island = await c.findByTestId("island");
    const card = (el: HTMLElement) => getComputedStyle(el).getPropertyValue("--card").trim();

    // The island is themed by a wrapper below :root, which is what
    // addon-themes and any scoped preview do. Plain `@theme` bakes the value in
    // at build time and these two would match.
    expect(card(root)).not.toBe("");
    expect(card(island)).not.toBe("");
    expect(card(root)).not.toBe(card(island));
  },
};
