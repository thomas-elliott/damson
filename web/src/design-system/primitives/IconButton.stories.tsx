import type { Meta, StoryObj } from "@storybook/react";
import { ChevronLeft, ChevronRight, Download, MoreHorizontal, Search, X } from "lucide-react";

import { IconButton } from "./IconButton";

const meta: Meta<typeof IconButton> = {
  title: "Primitives/IconButton",
  component: IconButton,
};
export default meta;
type Story = StoryObj<typeof IconButton>;

export const Sizes: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <IconButton aria-label="Close" size="sm">
        <X size={14} />
      </IconButton>
      <IconButton aria-label="Close" size="md">
        <X size={16} />
      </IconButton>
    </div>
  ),
};

export const Row: Story = {
  render: () => (
    <div className="flex items-center gap-1">
      <IconButton aria-label="Previous">
        <ChevronLeft size={14} />
      </IconButton>
      <IconButton aria-label="Next">
        <ChevronRight size={14} />
      </IconButton>
      <IconButton aria-label="Search">
        <Search size={14} />
      </IconButton>
      <IconButton aria-label="Download">
        <Download size={14} />
      </IconButton>
      <IconButton aria-label="More">
        <MoreHorizontal size={14} />
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
          <span className="w-20 font-mono text-label text-subtle-foreground">{row.label}</span>
          <div className={row.cls}>
            <div className="flex items-center gap-1">
              <IconButton aria-label="Previous">
                <ChevronLeft size={14} />
              </IconButton>
              <IconButton aria-label="More">
                <MoreHorizontal size={14} />
              </IconButton>
              <IconButton aria-label="Previous" active>
                <ChevronLeft size={14} />
              </IconButton>
            </div>
          </div>
        </div>
      ))}
      <div className="flex items-center gap-3">
        <span className="w-20 font-mono text-label text-subtle-foreground">Disabled</span>
        <div className="flex items-center gap-1">
          <IconButton aria-label="Previous" disabled>
            <ChevronLeft size={14} />
          </IconButton>
          <IconButton aria-label="More" disabled>
            <MoreHorizontal size={14} />
          </IconButton>
        </div>
      </div>
    </div>
  ),
};
