import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";

import { tidyAssetName } from "@/design-system/utils";

import { AssetCard } from "./AssetCard";
import { LicencePill } from "./LicencePill";
import { ASSET_FIXTURES, formatBytes } from "./_fixtures";

const meta: Meta = {
  title: "Domain/Asset",
};
export default meta;
type Story = StoryObj;

export const Grid: Story = {
  parameters: { layout: "padded" },
  render: function GridDemo() {
    const [selected, setSelected] = useState<string | null>(null);
    return (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
        {ASSET_FIXTURES.map((a) => (
          <AssetCard
            key={a.id}
            name={a.name}
            thumb={a.thumb}
            typeBadge={a.type === "unitypackage" ? "unitypkg" : a.type}
            sub={
              <>
                <span className="font-mono">{formatBytes(a.size)}</span>
                {a.files > 1 && (
                  <>
                    <span className="opacity-60">·</span>
                    <span className="font-mono">{a.files} files</span>
                  </>
                )}
              </>
            }
            trailing={<LicencePill status={a.licenceStatus} spdx={a.spdx} />}
            needsReview={a.needsReview}
            selected={selected === a.id}
            onSelect={() => setSelected((prev) => (prev === a.id ? null : a.id))}
          />
        ))}
      </div>
    );
  },
};

export const SingleCard: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="w-[220px]">
      <AssetCard
        name="hero_run_cycle"
        thumb={ASSET_FIXTURES[1]!.thumb}
        typeBadge="sprite"
        sub={<span className="font-mono">48 KB</span>}
        trailing={<LicencePill status="ok" spdx="CC-BY-4.0" />}
      />
    </div>
  ),
};

export const WithReview: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="w-[220px]">
      <AssetCard
        name="Sci-Fi UI Pack Vol. 3"
        thumb={ASSET_FIXTURES[4]!.thumb}
        typeBadge="unitypkg"
        needsReview
        sub={<span className="font-mono">142 MB · 412 files</span>}
        trailing={<LicencePill status="warn" spdx="Unity-EULA" />}
      />
    </div>
  ),
};

// Packs with no preview: the name is set on a colour taken from the publisher.
export const TypesetCovers: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
      <AssetCard name="Modular Dungeon Kit" publisher="Polyforge" />
      <AssetCard
        name={tidyAssetName("POLYGON_Fantasy_Village_URP")}
        publisher="Synty Studios Limited Interactive"
        needsReview
        updateAvailable
      />
      <AssetCard
        name="Ambient Caves Loop Pack"
        publisher="Northwind Audio"
        selected
        sub={<span className="font-mono">86 MB · 41 files</span>}
      />
      <AssetCard name="Untitled import" sub={<span className="font-mono">2 MB</span>} />
    </div>
  ),
};
