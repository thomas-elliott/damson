import type { Meta, StoryObj } from "@storybook/react";

const meta: Meta = {
  title: "Tokens/Palette",
  parameters: { layout: "padded" },
};
export default meta;
type Story = StoryObj;

function Swatch({ name, cssVar }: { name: string; cssVar: string }) {
  return (
    <div className="flex items-center gap-3">
      <div
        className="h-10 w-10 rounded-md border border-line"
        style={{ background: `var(${cssVar})` }}
      />
      <div className="flex flex-col">
        <span className="text-sm-plus font-medium">{name}</span>
        <span className="font-mono text-xs text-text-3">{cssVar}</span>
      </div>
    </div>
  );
}

const surfaces = [
  { name: "bg", v: "--ds-bg" },
  { name: "panel", v: "--ds-panel" },
  { name: "surface", v: "--ds-surface" },
  { name: "surface-2", v: "--ds-surface-2" },
  { name: "line", v: "--ds-line" },
  { name: "line-2", v: "--ds-line-2" },
];
const text = [
  { name: "text", v: "--ds-text" },
  { name: "text-2", v: "--ds-text-2" },
  { name: "text-3", v: "--ds-text-3" },
  { name: "text-4", v: "--ds-text-4" },
];
const accent = [
  { name: "accent", v: "--ds-accent" },
  { name: "accent-2", v: "--ds-accent-2" },
  { name: "accent-tint", v: "--ds-accent-tint" },
  { name: "accent-ring", v: "--ds-accent-ring" },
];
const status = [
  { name: "ok", v: "--ds-ok" },
  { name: "ok-tint", v: "--ds-ok-tint" },
  { name: "warn", v: "--ds-warn" },
  { name: "warn-tint", v: "--ds-warn-tint" },
  { name: "err", v: "--ds-err" },
  { name: "err-tint", v: "--ds-err-tint" },
  { name: "unk", v: "--ds-unk" },
  { name: "unk-tint", v: "--ds-unk-tint" },
];

function Group({
  heading,
  swatches,
}: {
  heading: string;
  swatches: { name: string; v: string }[];
}) {
  return (
    <section className="flex flex-col gap-3">
      <h3 className="text-2xs font-semibold uppercase tracking-wider text-text-4">
        {heading}
      </h3>
      <div className="grid grid-cols-2 gap-3">
        {swatches.map((s) => (
          <Swatch key={s.v} name={s.name} cssVar={s.v} />
        ))}
      </div>
    </section>
  );
}

export const Palette: Story = {
  render: () => (
    <div className="flex min-w-[520px] flex-col gap-6">
      <Group heading="Surfaces" swatches={surfaces} />
      <Group heading="Text" swatches={text} />
      <Group heading="Accent (yellow)" swatches={accent} />
      <Group heading="Licence status" swatches={status} />
    </div>
  ),
};

export const Typography: Story = {
  render: () => (
    <div className="flex min-w-[420px] flex-col gap-6">
      <section className="flex flex-col gap-2">
        <p className="text-label text-text-3">Layer 1 — size primitives</p>
        <p className="text-body">text-body — 15px (default running text)</p>
        <p className="text-ui">text-ui — 14px (UI controls, list rows)</p>
        <p className="text-small">text-small — 12px (fine print floor)</p>
      </section>

      <section className="flex flex-col gap-2">
        <p className="text-label text-text-3">Layer 2 — headings (paired with HTML h1/h2/h3)</p>
        <p className="text-title">text-title — 22px / semibold</p>
        <p className="text-title-bold">text-title-bold — 22px / bold</p>
        <p className="text-title-medium">text-title-medium — 22px / medium</p>
        <p className="text-title-reg">text-title-reg — 22px / regular</p>
        <p className="text-heading">text-heading — 20px / semibold</p>
        <p className="text-subhead">text-subhead — 18px / semibold</p>
      </section>

      <section className="flex flex-col gap-2">
        <p className="text-label text-text-3">Layer 2 — role tokens</p>
        <p className="text-label">text-label — section labels</p>
        <p className="text-mono">text-mono — sha256:ab3c1d…</p>
        <p className="text-caption">text-caption — helper text under inputs</p>
        <p className="text-num">text-num — 1,234,567 (tabular numerals)</p>
        <p className="text-table-row">text-table-row — body of a table cell</p>
        <p className="text-table-header">text-table-header — column header</p>
        <p className="text-list-row">text-list-row — body of a list-view row</p>
      </section>
    </div>
  ),
};
