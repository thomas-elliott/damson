import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";
import { Tag as TagIcon } from "lucide-react";
import { expect, userEvent, within } from "storybook/test";

import { Button } from "@/design-system/primitives/Button";
import { Popover, PopoverContent, PopoverTrigger } from "@/design-system/primitives/Popover";

import { TagIndexCard } from "./TagIndexCard";
import { TagPicker, type TagPickerTag } from "./TagPicker";

const meta: Meta = {
  title: "Domain/Tags",
};
export default meta;
type Story = StoryObj;

// Store-category tags, up to four levels deep. Counts are examples.
const TAGS: TagPickerTag[] = [
  ["3d", 214],
  ["3d/animation", 24],
  ["3d/character", 52],
  ["3d/character/animal", 12],
  ["3d/character/creature", 8],
  ["3d/character/humanoid", 28],
  ["3d/character/humanoid/human", 18],
  ["3d/character/humanoid/fantasy", 6],
  ["3d/character/humanoid/sci-fi", 3],
  ["3d/character/humanoid/zombie", 1],
  ["3d/character/robot", 4],
  ["3d/environment", 96],
  ["3d/environment/dungeon", 12],
  ["3d/environment/fantasy", 38],
  ["3d/environment/sci-fi", 21],
  ["3d/environment/urban", 14],
  ["3d/props", 77],
  ["3d/vegetation", 31],
  ["3d/vehicles", 18],
  ["2d", 174],
  ["2d/gui", 64],
  ["2d/textures", 110],
  ["audio", 37],
  ["audio/sound-fx", 37],
  ["audio/music", 9],
  ["vfx", 21],
  ["vfx/particles", 21],
  ["tools", 15],
  ["tools/utilities", 15],
].map(([path, count]) => ({ path: path as string, count: count as number }));

export const Picker: Story = {
  parameters: { layout: "padded" },
  render: function PickerDemo() {
    const [selected, setSelected] = useState(["3d/character/humanoid/human"]);
    return (
      <div className="w-fit rounded-lg border border-border bg-popover">
        <TagPicker
          tags={TAGS}
          selected={selected}
          onSelectedChange={setSelected}
          recent={["3d/environment/fantasy", "audio/sound-fx", "2d/gui"]}
          onApply={() => {}}
        />
      </div>
    );
  },
};

// A narrow screen: two columns, and a column's heading steps back up a level.
export const PickerNarrow: Story = {
  parameters: { layout: "padded" },
  render: function PickerNarrowDemo() {
    const [selected, setSelected] = useState(["3d/character/humanoid/human"]);
    return (
      <div className="w-[384px] rounded-lg border border-border bg-popover">
        <TagPicker
          tags={TAGS}
          selected={selected}
          onSelectedChange={setSelected}
          onApply={() => {}}
        />
      </div>
    );
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole("list", { name: "Tags in 3d/character/humanoid" })).toBeVisible();
    // Hidden columns are out of the accessibility tree too.
    await expect(canvas.queryByRole("list", { name: "Top-level tags" })).toBeNull();
    await userEvent.click(canvas.getByRole("button", { name: /Up a level from\s*humanoid/ }));
    await expect(canvas.getByRole("list", { name: "Tags in 3d/character" })).toBeVisible();
  },
};

// The way it's opened in the library: the Tags button shows the current path.
export const InPopover: Story = {
  parameters: { layout: "padded" },
  render: function InPopoverDemo() {
    const [open, setOpen] = useState(false);
    const [selected, setSelected] = useState(["3d/environment/fantasy"]);
    return (
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button variant="secondary" pressed={open} aria-pressed={open} leadingIcon={<TagIcon />}>
            Tags
            <span className="font-mono text-label opacity-80">
              {selected.length === 1 ? selected[0] : `${selected.length} ticked`}
            </span>
          </Button>
        </PopoverTrigger>
        <PopoverContent className="p-0">
          <TagPicker
            tags={TAGS}
            selected={selected}
            onSelectedChange={setSelected}
            onApply={() => setOpen(false)}
          />
        </PopoverContent>
      </Popover>
    );
  },
};

export const IndexCards: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
      <TagIndexCard
        path="3d/environment"
        count={96}
        selected
        topChildren={[
          { name: "fantasy", count: 38 },
          { name: "sci-fi", count: 21 },
          { name: "urban", count: 14 },
        ]}
        covers={[
          { name: "Stylized Forest Pack", publisher: "Polyforge" },
          { name: "Pine Valley Terrain", publisher: "Lowpoly Lab" },
          { name: "Haunted Village Kit", publisher: "Moss and Stone" },
        ]}
      />
      <TagIndexCard
        path="3d/props"
        count={77}
        topChildren={[
          { name: "furniture", count: 24 },
          { name: "weapons", count: 19 },
        ]}
        covers={[
          { name: "Ancient Furniture Set", publisher: "Kitbash Foundry" },
          { name: "Realistic Weapons Kit", publisher: "Kenji Sato" },
          { name: "Fantasy Weapons Pack", publisher: "Pixel Harbour" },
        ]}
      />
      <TagIndexCard
        path="audio/sound-fx"
        count={37}
        topChildren={[
          { name: "footsteps", count: 12 },
          { name: "ui", count: 8 },
        ]}
        covers={[
          { name: "Sci-Fi Footsteps Kit", publisher: "Northwind Audio" },
          { name: "Ambient Caves Loop Pack", publisher: "Northwind Audio" },
        ]}
      />
      <TagIndexCard path="vfx/particles" count={21} covers={[]} />
    </div>
  ),
};
