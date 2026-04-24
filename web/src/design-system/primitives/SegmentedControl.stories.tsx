import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";
import { LayoutGrid, List } from "lucide-react";

import { SegmentedControl } from "./SegmentedControl";

const meta: Meta<typeof SegmentedControl> = {
  title: "Primitives/SegmentedControl",
  component: SegmentedControl,
};
export default meta;
type Story = StoryObj<typeof SegmentedControl>;

export const ViewToggle: Story = {
  render: () => {
    const [v, setV] = useState<"grid" | "list">("grid");
    return (
      <SegmentedControl
        value={v}
        onValueChange={setV}
        options={[
          {
            value: "grid",
            icon: <LayoutGrid size={12} />,
            "aria-label": "Grid view",
          },
          {
            value: "list",
            icon: <List size={12} />,
            "aria-label": "List view",
          },
        ]}
      />
    );
  },
};

export const TextOptions: Story = {
  render: () => {
    const [v, setV] = useState<"compact" | "medium" | "large">("medium");
    return (
      <SegmentedControl
        value={v}
        onValueChange={setV}
        options={[
          { value: "compact", label: "Compact" },
          { value: "medium", label: "Medium" },
          { value: "large", label: "Large" },
        ]}
      />
    );
  },
};
