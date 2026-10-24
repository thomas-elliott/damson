import type { Meta, StoryObj } from "@storybook/react";
import { expect, waitFor, within } from "storybook/test";

import type { AssetRow } from "@/hooks/useDataQueries";

import { AssetGrid } from "./AssetGrid";

const meta: Meta<typeof AssetGrid> = {
  title: "App/AssetGrid",
  component: AssetGrid,
  parameters: { layout: "fullscreen" },
  decorators: [
    (Story) => (
      <div className="h-[600px] w-full bg-background">
        <Story />
      </div>
    ),
  ],
};
export default meta;
type Story = StoryObj<typeof AssetGrid>;

const STORED: AssetRow = {
  id: 1,
  name: "POLYGON - Fantasy Kingdom",
  file_count: 1284,
  size_bytes: 734_003_200,
  needs_review: false,
  created_at: "2026-04-20T11:00:00Z",
};

const REFERENCE: AssetRow = {
  id: 2,
  name: "POLYGON_Fantasy_Characters",
  file_count: 0,
  size_bytes: 0,
  needs_review: false,
  created_at: "2026-04-20T11:00:00Z",
  is_reference: true,
};

/** A stored pack beside a reference asset: the reference shows its mark, never "0 B". */
export const WithReferenceAsset: Story = {
  args: { assets: [STORED, REFERENCE] },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Reference")).toBeVisible();
    await expect(canvas.getByText("1284 files")).toBeVisible();
    await expect(canvas.queryByText("0 B")).not.toBeInTheDocument();
  },
};

const MANY: AssetRow[] = Array.from({ length: 12 }, (_, i) => ({
  ...STORED,
  id: 100 + i,
  name: `Pack ${i + 1}`,
}));

// The library's grid density sets the minimum cell width, so at a fixed 800px
// a smaller density fits more cards on a row.
function FixedWidth(Story: () => React.ReactNode) {
  return (
    <div style={{ display: "flex", flexDirection: "column", width: 800, height: 600 }}>
      {Story()}
    </div>
  );
}

function firstRowCards(canvasElement: HTMLElement) {
  return canvasElement.querySelector('[data-index="0"]')?.children.length;
}

/** Small density: more, smaller cards per row. */
export const SmallDensity: Story = {
  args: { assets: MANY, density: "small" },
  decorators: [FixedWidth],
  play: async ({ canvasElement }) => {
    await waitFor(() => expect(firstRowCards(canvasElement)).toBe(5));
  },
};

/** Large density: fewer, bigger cards per row. */
export const LargeDensity: Story = {
  args: { assets: MANY, density: "large" },
  decorators: [FixedWidth],
  play: async ({ canvasElement }) => {
    await waitFor(() => expect(firstRowCards(canvasElement)).toBe(3));
  },
};
