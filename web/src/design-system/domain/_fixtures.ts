/** Shared fixtures for domain-component stories. Shapes mirror docs/mockup/src/data.jsx. */
import type { ThumbDescriptor } from "./AssetThumb";
import type { LicenceStatus } from "@/design-system/tokens";

export interface AssetFixture {
  id: string;
  name: string;
  type: "unitypackage" | "sprite" | "audio" | "font" | "file";
  spdx: string;
  licenceStatus: LicenceStatus;
  size: number;
  files: number;
  imported: string;
  needsReview?: boolean;
  thumb: ThumbDescriptor;
}

export const ASSET_FIXTURES: AssetFixture[] = [
  {
    id: "a01",
    name: "POLYGON - Fantasy Kingdom",
    type: "unitypackage",
    spdx: "Unity-EULA",
    licenceStatus: "warn",
    size: 684_221_440,
    files: 1284,
    imported: "2026-03-12",
    thumb: {
      kind: "sprite",
      palette: ["#7d9266", "#b8a375", "#5a6b52", "#d4c28b"],
      pattern: "landscape",
    },
  },
  {
    id: "a02",
    name: "hero_run_cycle",
    type: "sprite",
    spdx: "CC-BY-4.0",
    licenceStatus: "ok",
    size: 48_200,
    files: 1,
    imported: "2026-04-18",
    thumb: {
      kind: "sprite",
      palette: ["#c85a4c", "#2b2f3a", "#f0d37a"],
      pattern: "character",
    },
  },
  {
    id: "a03",
    name: "ambient_forest_loop",
    type: "audio",
    spdx: "CC0-1.0",
    licenceStatus: "ok",
    size: 2_845_000,
    files: 1,
    imported: "2026-04-11",
    thumb: { kind: "waveform", seed: 7 },
  },
  {
    id: "a04",
    name: "Alagard",
    type: "font",
    spdx: "OFL-1.1",
    licenceStatus: "ok",
    size: 32_480,
    files: 2,
    imported: "2026-02-01",
    thumb: { kind: "font", sample: "Ag", family: "Georgia" },
  },
  {
    id: "a05",
    name: "Sci-Fi UI Pack Vol. 3",
    type: "unitypackage",
    spdx: "Unity-EULA",
    licenceStatus: "warn",
    size: 142_000_000,
    files: 412,
    imported: "2026-04-20",
    needsReview: true,
    thumb: {
      kind: "sprite",
      palette: ["#2d4a6b", "#58a7d4", "#0f1620"],
      pattern: "ui",
    },
  },
  {
    id: "a06",
    name: "metal_impact_01",
    type: "audio",
    spdx: "CC-BY-4.0",
    licenceStatus: "ok",
    size: 186_000,
    files: 1,
    imported: "2026-04-15",
    thumb: { kind: "waveform", seed: 3 },
  },
  {
    id: "a07",
    name: "tileset_dungeon_16x",
    type: "sprite",
    spdx: "CC-BY-SA-4.0",
    licenceStatus: "ok",
    size: 98_400,
    files: 1,
    imported: "2026-03-28",
    thumb: {
      kind: "sprite",
      palette: ["#3a3147", "#6b5477", "#9b8aa8", "#d4cce0"],
      pattern: "tiles",
    },
  },
  {
    id: "a08",
    name: "portrait_noir_pack",
    type: "sprite",
    spdx: "CC-BY-NC-4.0",
    licenceStatus: "err",
    size: 1_800_000,
    files: 24,
    imported: "2026-03-02",
    thumb: {
      kind: "sprite",
      palette: ["#1a1a1a", "#8a7a6c", "#4a3c3c"],
      pattern: "portrait",
    },
  },
  {
    id: "a09",
    name: "sunset_gradient_01",
    type: "sprite",
    spdx: "Unknown",
    licenceStatus: "unk",
    size: 12_400,
    files: 1,
    imported: "2026-04-22",
    thumb: {
      kind: "sprite",
      palette: ["#ff6a3d", "#d5357a", "#5c3b8c"],
      pattern: "gradient",
    },
  },
];

export interface ProjectFixture {
  id: string;
  name: string;
  glyph: string;
  color: string;
  path: string;
  engine: string;
  assets: number;
  updated: string;
}

/* Project glyph colours: OKLCH L<=0.4 so --ds-text (L=0.93) is comfortably
 * over WCAG AA (≥5:1) on top. */
export const PROJECT_FIXTURES: ProjectFixture[] = [
  {
    id: "p01",
    name: "Ashfall",
    glyph: "A",
    color: "oklch(0.4 0.14 20)",
    path: "~/Dev/ashfall",
    engine: "Unity 2023.3",
    assets: 128,
    updated: "2026-04-20",
  },
  {
    id: "p02",
    name: "Grove",
    glyph: "G",
    color: "oklch(0.4 0.1 155)",
    path: "~/Dev/grove",
    engine: "Godot 4.3",
    assets: 64,
    updated: "2026-04-14",
  },
  {
    id: "p03",
    name: "Salt & Stars",
    glyph: "SS",
    color: "oklch(0.4 0.14 260)",
    path: "~/Dev/salt-stars",
    engine: "Unity 6.0",
    assets: 42,
    updated: "2026-03-28",
  },
];

export { formatBytes } from "@/design-system/utils";
