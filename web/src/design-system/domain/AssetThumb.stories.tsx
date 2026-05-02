import type { Meta, StoryObj } from "@storybook/react";

import { AssetThumb, type SpritePattern } from "./AssetThumb";

const meta: Meta = {
  title: "Domain/AssetThumb",
};
export default meta;
type Story = StoryObj;

const SPRITE_PATTERNS: SpritePattern[] = [
  "abstract",
  "landscape",
  "character",
  "tiles",
  "ui",
  "portrait",
  "gradient",
  "fire",
  "icons",
];

const SAMPLE_PALETTES: Record<SpritePattern, string[]> = {
  abstract: ["#f5a623", "#2b2b2b"],
  landscape: ["#7d9266", "#b8a375", "#5a6b52", "#d4c28b"],
  character: ["#c85a4c", "#2b2f3a", "#f0d37a"],
  tiles: ["#3a3147", "#6b5477", "#9b8aa8", "#d4cce0"],
  ui: ["#2d4a6b", "#58a7d4", "#0f1620"],
  portrait: ["#1a1a1a", "#8a7a6c", "#4a3c3c"],
  gradient: ["#ff6a3d", "#d5357a", "#5c3b8c"],
  fire: ["#ff7a18", "#ffc93c", "#0f0f0f"],
  icons: ["#2b2b2b", "#f5a623"],
};

export const AllPatterns: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="grid grid-cols-3 gap-4">
      {SPRITE_PATTERNS.map((p) => (
        <div key={p} className="flex flex-col gap-1.5">
          <div className="aspect-square overflow-hidden rounded-md border border-line">
            <AssetThumb thumb={{ kind: "sprite", palette: SAMPLE_PALETTES[p], pattern: p }} />
          </div>
          <div className="text-mono text-text-3">{p}</div>
        </div>
      ))}
    </div>
  ),
};

export const Waveforms: Story = {
  render: () => (
    <div className="grid grid-cols-4 gap-3">
      {[1, 3, 7, 13].map((seed) => (
        <div key={seed} className="flex flex-col gap-1.5">
          <div className="aspect-square overflow-hidden rounded-md border border-line bg-panel">
            <AssetThumb thumb={{ kind: "waveform", seed }} />
          </div>
          <div className="text-mono text-text-3">seed {seed}</div>
        </div>
      ))}
    </div>
  ),
};

export const Fonts: Story = {
  render: () => (
    <div className="grid grid-cols-4 gap-3">
      {[
        { sample: "Ag", family: "Georgia" },
        { sample: "Vr", family: "Menlo" },
        { sample: "Hx", family: "Times" },
        { sample: "Qt", family: "Courier" },
      ].map((s) => (
        <div key={s.family} className="flex flex-col gap-1.5">
          <div className="aspect-square overflow-hidden rounded-md border border-line">
            <AssetThumb thumb={{ kind: "font", sample: s.sample, family: s.family }} />
          </div>
          <div className="text-mono text-text-3">{s.family}</div>
        </div>
      ))}
    </div>
  ),
};

export const Unknown: Story = {
  render: () => (
    <div className="h-[160px] w-[160px] overflow-hidden rounded-md border border-line">
      <AssetThumb thumb={{ kind: "unknown" }} />
    </div>
  ),
};
