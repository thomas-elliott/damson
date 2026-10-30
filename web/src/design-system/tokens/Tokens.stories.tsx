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
        className="h-10 w-10 rounded-md border border-border"
        style={{ background: `var(${cssVar})` }}
      />
      <div className="flex flex-col">
        <span className="text-ui font-medium">{name}</span>
        <span className="text-mono text-subtle-foreground">{cssVar}</span>
      </div>
    </div>
  );
}

const surfaces = [
  { name: "background", v: "--background" },
  { name: "card", v: "--card" },
  { name: "muted", v: "--muted" },
  { name: "accent", v: "--accent" },
  { name: "border", v: "--border" },
  { name: "border-strong", v: "--border-strong" },
];
const text = [
  { name: "foreground", v: "--foreground" },
  { name: "muted-foreground", v: "--muted-foreground" },
  { name: "subtle-foreground", v: "--subtle-foreground" },
  { name: "faint-foreground", v: "--faint-foreground" },
];
const accent = [
  { name: "primary", v: "--primary" },
  { name: "primary-hover", v: "--primary-hover" },
  { name: "primary-soft", v: "--primary-soft" },
  { name: "ring", v: "--ring" },
];
const status = [
  { name: "success", v: "--success" },
  { name: "success-soft", v: "--success-soft" },
  { name: "warning", v: "--warning" },
  { name: "warning-soft", v: "--warning-soft" },
  { name: "destructive", v: "--destructive" },
  { name: "destructive-soft", v: "--destructive-soft" },
  { name: "unknown", v: "--unknown" },
  { name: "unknown-soft", v: "--unknown-soft" },
];
const category = [
  { name: "category-3d", v: "--category-3d" },
  { name: "category-2d", v: "--category-2d" },
  { name: "category-audio", v: "--category-audio" },
  { name: "category-vfx", v: "--category-vfx" },
  { name: "category-tools", v: "--category-tools" },
  { name: "category-other", v: "--category-other" },
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
      <h3 className="text-overline font-semibold text-faint-foreground">{heading}</h3>
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
      <Group heading="Status" swatches={status} />
      <Group heading="Tag category" swatches={category} />
    </div>
  ),
};

export const Typography: Story = {
  render: () => (
    <div className="flex min-w-[420px] flex-col gap-6">
      <section className="flex flex-col gap-2">
        <p className="text-overline text-subtle-foreground">Layer 1 — size primitives</p>
        <p className="text-body">text-body — 15px (default running text)</p>
        <p className="text-ui">text-ui — 14px (UI controls, list rows)</p>
        <p className="text-label">text-label — 12px (fine print floor)</p>
      </section>

      <section className="flex flex-col gap-2">
        <p className="text-overline text-subtle-foreground">
          Layer 2 — headings (paired with HTML h1/h2/h3)
        </p>
        <p className="font-display text-title-bold uppercase tracking-wide">
          font-display — titles in capitals
        </p>
        <p className="text-title-semibold">text-title-semibold — 22px / semibold</p>
        <p className="text-title-bold">text-title-bold — 22px / bold</p>
        <p className="text-title-medium">text-title-medium — 22px / medium</p>
        <p className="text-title-reg">text-title-reg — 22px / regular</p>
        <p className="text-heading-semibold">text-heading-semibold — 20px / semibold</p>
        <p className="text-subhead-semibold">text-subhead-semibold — 18px / semibold</p>
      </section>

      <section className="flex flex-col gap-2">
        <p className="text-overline text-subtle-foreground">Layer 2 — role tokens</p>
        <p className="text-overline">text-overline — section labels</p>
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
