import type { Meta, StoryObj } from "@storybook/react";

import { LicenceBar } from "./LicenceBar";
import { ProjectCard, ProjectCardNew } from "./ProjectCard";
import { ProjectGlyph } from "./ProjectGlyph";
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
          distribution={p.distribution}
        >
          <LicenceBar counts={{ ok: 110, warn: 12, err: 0, unk: 6 }} />
          <div className="flex justify-between text-mono text-subtle-foreground">
            <span>{p.assets} assets</span>
            <span>{p.updated}</span>
          </div>
        </ProjectCard>
      ))}
      <ProjectCardNew />
    </div>
  ),
};
