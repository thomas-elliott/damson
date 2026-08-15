import type { Meta, StoryObj } from "@storybook/react";
import { ChevronLeft, ChevronRight, Download, MoreHorizontal, Search, X } from "lucide-react";

import { IconButton } from "./IconButton";

const meta: Meta<typeof IconButton> = {
  title: "Primitives/IconButton",
  component: IconButton,
  args: { "aria-label": "More actions", children: <MoreHorizontal /> },
  argTypes: {
    variant: {
      control: { type: "select" },
      options: ["primary", "secondary", "ghost", "destructive"],
    },
    size: {
      control: { type: "inline-radio" },
      options: ["sm", "md", "lg"],
    },
    disabled: { control: { type: "boolean" } },
    pressed: { control: { type: "boolean" } },
  },
};
export default meta;
type Story = StoryObj<typeof IconButton>;

export const Sizes: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <IconButton aria-label="Close" size="sm">
        <X />
      </IconButton>
      <IconButton aria-label="Close" size="md">
        <X />
      </IconButton>
      <IconButton aria-label="Close" size="lg">
        <X />
      </IconButton>
    </div>
  ),
};

export const Row: Story = {
  render: () => (
    <div className="flex items-center gap-1">
      <IconButton aria-label="Previous" size="sm">
        <ChevronLeft />
      </IconButton>
      <IconButton aria-label="Next" size="sm">
        <ChevronRight />
      </IconButton>
      <IconButton aria-label="Search" size="sm">
        <Search />
      </IconButton>
      <IconButton aria-label="Download" size="sm">
        <Download />
      </IconButton>
      <IconButton aria-label="More" size="sm">
        <MoreHorizontal />
      </IconButton>
    </div>
  ),
};

export const AllStates: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="flex flex-col gap-3">
      {[
        { label: "Rest", cls: "" },
        { label: "Hover", cls: "pseudo-hover-all" },
        { label: "Focus", cls: "pseudo-focus-visible-all" },
        { label: "Active", cls: "pseudo-active-all" },
      ].map((row) => (
        <div key={row.label} className="flex items-center gap-3">
          <span className="w-20 font-mono text-overline text-subtle-foreground">{row.label}</span>
          <div className={row.cls}>
            <div className="flex items-center gap-1">
              <IconButton aria-label="Previous" size="sm">
                <ChevronLeft />
              </IconButton>
              <IconButton aria-label="More" size="sm">
                <MoreHorizontal />
              </IconButton>
              <IconButton aria-label="Previous" size="sm" pressed aria-pressed>
                <ChevronLeft />
              </IconButton>
            </div>
          </div>
        </div>
      ))}
      <div className="flex items-center gap-3">
        <span className="w-20 font-mono text-overline text-subtle-foreground">Disabled</span>
        <div className="flex items-center gap-1">
          <IconButton aria-label="Previous" size="sm" disabled>
            <ChevronLeft />
          </IconButton>
          <IconButton aria-label="More" size="sm" disabled>
            <MoreHorizontal />
          </IconButton>
        </div>
      </div>
    </div>
  ),
};
