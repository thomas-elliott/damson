import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";

import { AssetCard } from "./AssetCard";
import { AssetListHead, AssetListRow } from "./AssetListRow";
import { LicencePill } from "./LicencePill";
import { ASSET_FIXTURES, formatBytes } from "./_fixtures";

const meta: Meta = {
  title: "Domain/Asset",
};
export default meta;
type Story = StoryObj;

export const Grid: Story = {
  parameters: { layout: "padded" },
  render: () => {
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
                    <span className="text-text-4">·</span>
                    <span className="font-mono">{a.files} files</span>
                  </>
                )}
              </>
            }
            trailing={
              <LicencePill status={a.licenceStatus} spdx={a.spdx} noDot />
            }
            needsReview={a.needsReview}
            selected={selected === a.id}
            onSelect={() =>
              setSelected((prev) => (prev === a.id ? null : a.id))
            }
          />
        ))}
      </div>
    );
  },
};

export const ListView: Story = {
  parameters: { layout: "padded" },
  render: () => {
    const [selected, setSelected] = useState<string | null>(null);
    return (
      <div className="overflow-hidden rounded-md border border-line bg-bg">
        <table className="w-full border-separate border-spacing-0">
          <AssetListHead columns={["Type", "Licence", "Size", "Imported"]} />
          <tbody>
            {ASSET_FIXTURES.map((a) => (
              <AssetListRow
                key={a.id}
                name={a.name}
                thumb={a.thumb}
                selected={selected === a.id}
                onSelect={() =>
                  setSelected((prev) => (prev === a.id ? null : a.id))
                }
                cells={[
                  <span className="font-mono text-xs text-text-3">
                    {a.type}
                  </span>,
                  <LicencePill status={a.licenceStatus} spdx={a.spdx} noDot />,
                  <span className="font-mono text-xs text-text-3">
                    {formatBytes(a.size)}
                  </span>,
                  <span className="font-mono text-xs text-text-3">
                    {a.imported}
                  </span>,
                ]}
              />
            ))}
          </tbody>
        </table>
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
        trailing={<LicencePill status="ok" spdx="CC-BY-4.0" noDot />}
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
        trailing={<LicencePill status="warn" spdx="Unity-EULA" noDot />}
      />
    </div>
  ),
};
