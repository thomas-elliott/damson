import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";

import { Checkbox } from "./Checkbox";
import { Separator } from "./Separator";

const meta: Meta = {
  title: "Primitives/RadixAtoms",
};
export default meta;
type Story = StoryObj;

export const Checkboxes: Story = {
  render: function CheckboxesDemo() {
    const [checked, setChecked] = useState(false);
    return (
      <div className="flex flex-col gap-2">
        <label className="flex cursor-pointer items-center gap-2 text-ui">
          <Checkbox checked={checked} onCheckedChange={(v) => setChecked(v === true)} />
          <span>Controlled checkbox</span>
        </label>
        <label className="flex cursor-pointer items-center gap-2 text-ui">
          <Checkbox defaultChecked />
          <span>Default checked</span>
        </label>
        <label className="flex cursor-pointer items-center gap-2 text-ui">
          <Checkbox checked="indeterminate" />
          <span>Indeterminate</span>
        </label>
        <label className="flex cursor-not-allowed items-center gap-2 text-ui opacity-70">
          <Checkbox disabled />
          <span>Disabled</span>
        </label>
      </div>
    );
  },
};

export const Separators: Story = {
  render: () => (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2 text-ui">
        <span>One</span>
        <Separator orientation="vertical" />
        <span>Two</span>
        <Separator orientation="vertical" />
        <span>Three</span>
      </div>
      <div className="flex min-w-[280px] flex-col gap-2 text-ui">
        <span>Top</span>
        <Separator />
        <span>Middle</span>
        <Separator />
        <span>Bottom</span>
      </div>
    </div>
  ),
};
