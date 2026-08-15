import type { Meta, StoryObj } from "@storybook/react";
import { Mail } from "lucide-react";

import { Input } from "./Input";
import { SearchInput } from "./SearchInput";
import { Textarea } from "./Textarea";

const meta: Meta = {
  title: "Primitives/Input",
};
export default meta;
type Story = StoryObj;

export const Text: Story = {
  render: () => (
    <div className="flex min-w-[320px] flex-col gap-3">
      <Input placeholder="Untitled asset" aria-label="Asset name" />
      <Input placeholder="Email" aria-label="Email" leadingIcon={<Mail size={13} />} />
      <Input
        placeholder="Invalid"
        aria-label="Invalid sample"
        invalid
        defaultValue="not-an-email"
      />
      <Input placeholder="Disabled" aria-label="Disabled" disabled />
    </div>
  ),
};

export const Search: Story = {
  render: () => (
    <div className="flex min-w-[320px] flex-col gap-3">
      <SearchInput placeholder="Search assets, tags…" aria-label="Search assets" />
      <SearchInput
        placeholder="Search assets, tags…"
        aria-label="Search assets with shortcut"
        kbd="⌘K"
      />
    </div>
  ),
};

export const Multiline: Story = {
  render: () => (
    <div className="flex min-w-[320px] flex-col gap-3">
      <Textarea placeholder="Internal notes about this asset…" aria-label="Internal notes" />
      <Textarea
        placeholder="Invalid"
        invalid
        aria-label="Invalid"
        defaultValue="Too long… very long…"
      />
    </div>
  ),
};

export const AllStates: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="flex min-w-[320px] flex-col gap-3">
      {[
        { label: "Rest", cls: "" },
        { label: "Hover", cls: "pseudo-hover-all" },
        { label: "Focus", cls: "pseudo-focus-within-all" },
      ].map((row) => (
        <div key={row.label} className="flex items-center gap-3">
          <span className="w-20 font-mono text-label text-subtle-foreground">{row.label}</span>
          <div className={row.cls + " flex-1"}>
            <Input
              placeholder="name@example.com"
              aria-label={`Input ${row.label}`}
              leadingIcon={<Mail size={13} />}
            />
          </div>
        </div>
      ))}
      <div className="flex items-center gap-3">
        <span className="w-20 font-mono text-label text-subtle-foreground">Invalid</span>
        <div className="flex-1">
          <Input invalid defaultValue="not-an-email" aria-label="Invalid input" />
        </div>
      </div>
      <div className="flex items-center gap-3">
        <span className="w-20 font-mono text-label text-subtle-foreground">Disabled</span>
        <div className="flex-1">
          <Input disabled placeholder="placeholder" aria-label="Disabled input" />
        </div>
      </div>
    </div>
  ),
};
