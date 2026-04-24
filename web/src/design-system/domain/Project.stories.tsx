import type { Meta, StoryObj } from "@storybook/react";

import { LicenceBar } from "./LicenceBar";
import { ProjectCard, ProjectCardNew } from "./ProjectCard";
import { ProjectGlyph } from "./ProjectGlyph";
import { ProjectListRow } from "./ProjectListRow";
import { PROJECT_FIXTURES } from "./_fixtures";

const meta: Meta = {
  title: "Domain/Project",
};
export default meta;
type Story = StoryObj;

export const Glyphs: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <ProjectGlyph size="sm" glyph="A" color="oklch(0.4 0.14 20)" />
      <ProjectGlyph size="md" glyph="G" color="oklch(0.4 0.1 155)" />
      <ProjectGlyph size="lg" glyph="SS" color="oklch(0.4 0.14 260)" />
    </div>
  ),
};

export const Cards: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-3">
      {PROJECT_FIXTURES.map((p) => (
        <ProjectCard
          key={p.id}
          name={p.name}
          glyph={p.glyph}
          color={p.color}
          path={p.path}
          engine={p.engine}
        >
          <LicenceBar counts={{ ok: 110, warn: 12, err: 0, unk: 6 }} />
          <div className="flex justify-between font-mono text-xs text-text-3">
            <span>{p.assets} assets</span>
            <span>{p.updated}</span>
          </div>
        </ProjectCard>
      ))}
      <ProjectCardNew />
    </div>
  ),
};

export const List: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="overflow-hidden rounded-md border border-line bg-bg">
      {PROJECT_FIXTURES.map((p) => (
        <ProjectListRow
          key={p.id}
          name={p.name}
          glyph={p.glyph}
          color={p.color}
          style={{
            gridTemplateColumns: "2fr 1fr 1.5fr 1fr",
          }}
          cells={[
            <span className="font-mono text-xs text-text-3">{p.engine}</span>,
            <span className="truncate font-mono text-xs text-text-3">
              {p.path}
            </span>,
            <span className="font-mono text-xs text-text-3">
              {p.assets} assets · {p.updated}
            </span>,
          ]}
        />
      ))}
    </div>
  ),
};
