import type { Meta, StoryObj } from "@storybook/react";

import { ScrollArea } from "./ScrollArea";

const meta: Meta<typeof ScrollArea> = {
  title: "Primitives/ScrollArea",
  component: ScrollArea,
};
export default meta;
type Story = StoryObj<typeof ScrollArea>;

export const Vertical: Story = {
  render: () => (
    <ScrollArea className="h-[240px] w-[320px] rounded-md border border-border bg-card">
      <div className="flex flex-col gap-2 p-3 text-ui">
        {Array.from({ length: 40 }, (_, i) => (
          <div key={i} className="text-muted-foreground">
            Row {i + 1} — asset, tag, or project entry
          </div>
        ))}
      </div>
    </ScrollArea>
  ),
};

export const Horizontal: Story = {
  render: () => (
    <ScrollArea className="h-[120px] w-[420px] rounded-md border border-border bg-card">
      <div className="flex w-[1200px] items-center gap-3 p-3">
        {Array.from({ length: 20 }, (_, i) => (
          <div
            key={i}
            className="grid h-20 w-20 shrink-0 place-items-center rounded-md bg-muted text-caption"
          >
            tile {i + 1}
          </div>
        ))}
      </div>
    </ScrollArea>
  ),
};
