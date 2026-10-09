import type { Meta, StoryObj } from "@storybook/react";
import { Sliders } from "lucide-react";

import { Badge } from "./Badge";
import { Chip } from "./Chip";
import { Glyph } from "./Glyph";
import { Kbd } from "./Kbd";
import { Pill } from "./Pill";
import { ProgressBar } from "./ProgressBar";
import { StatusDot } from "./StatusDot";
import { Tag } from "./Tag";

const meta: Meta = {
  title: "Primitives/Display",
};
export default meta;
type Story = StoryObj;

export const Badges: Story = {
  render: () => (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <Badge>Neutral</Badge>
        <Badge tone="accent">Accent</Badge>
        <Badge tone="ok">
          <StatusDot tone="ok" /> Compliant
        </Badge>
        <Badge tone="warn">
          <StatusDot tone="warn" /> Review
        </Badge>
        <Badge tone="err">
          <StatusDot tone="err" /> Violation
        </Badge>
        <Badge tone="unk">
          <StatusDot tone="unk" /> Unknown
        </Badge>
      </div>
      <div className="flex items-center gap-2">
        <Badge size="sm">Small</Badge>
        <Badge size="md">Medium</Badge>
        <Badge mono>MIT</Badge>
        <Badge mono tone="ok">
          CC-BY-4.0
        </Badge>
      </div>
    </div>
  ),
};

export const Pills: Story = {
  render: () => (
    <div className="flex items-center gap-2">
      <Pill mono tone="ok">
        <StatusDot tone="ok" /> MIT
      </Pill>
      <Pill mono tone="warn">
        <StatusDot tone="warn" /> CC-BY-NC
      </Pill>
      <Pill mono tone="err">
        <StatusDot tone="err" /> Unity EULA
      </Pill>
      <Pill mono tone="unk">
        <StatusDot tone="unk" /> Unknown
      </Pill>
      <Pill tone="accent">
        <Sliders size={10} /> Smart
      </Pill>
    </div>
  ),
};

export const Chips: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      <Chip label="licence:" onRemove={() => {}} removeLabel="Remove licence filter: MIT">
        MIT
      </Chip>
      <Chip label="tag:" onRemove={() => {}} removeLabel="Remove tag filter: ui">
        ui
      </Chip>
      <Chip label="project:" onRemove={() => {}} removeLabel="Remove project filter: crystal-quest">
        crystal-quest
      </Chip>
    </div>
  ),
};

export const Tags: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      <Tag>sprites</Tag>
      <Tag>ui/icons</Tag>
      <Tag>game-ready</Tag>
      <Tag variant="add">Add tag</Tag>
    </div>
  ),
};

export const Kbds: Story = {
  render: () => (
    <div className="flex items-center gap-2">
      <Kbd>⌘K</Kbd>
      <Kbd>/</Kbd>
      <Kbd>↑</Kbd>
      <Kbd>↓</Kbd>
      <Kbd>Esc</Kbd>
    </div>
  ),
};

export const Progress: Story = {
  render: () => (
    <div className="flex min-w-[320px] flex-col gap-3">
      <ProgressBar value={20} aria-label="20%" />
      <ProgressBar value={60} aria-label="60%" />
      <ProgressBar value={100} aria-label="complete" />
      <ProgressBar value={50} tone="warn" aria-label="warn 50%" />
      <ProgressBar value={80} tone="err" aria-label="err 80%" />
    </div>
  ),
};

export const Glyphs: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <Glyph size="sm" color="oklch(0.6 0.16 30)">
        C
      </Glyph>
      <Glyph size="md" color="oklch(0.6 0.16 180)">
        V
      </Glyph>
      <Glyph size="lg" color="oklch(0.55 0.16 260)">
        S
      </Glyph>
      <Glyph size="md">?</Glyph>
    </div>
  ),
};

export const StatusDots: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <StatusDot tone="ok" />
      <StatusDot tone="warn" />
      <StatusDot tone="err" />
      <StatusDot tone="unk" />
      <StatusDot tone="accent" />
      <StatusDot tone="accent" size="md" ring />
    </div>
  ),
};
