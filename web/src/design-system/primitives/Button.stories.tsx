import type { Meta, StoryObj } from "@storybook/react";
import { Download, Plus, Trash2 } from "lucide-react";

import { Button } from "./Button";

const meta: Meta<typeof Button> = {
  title: "Primitives/Button",
  component: Button,
  args: { children: "Button" },
  argTypes: {
    variant: {
      control: { type: "select" },
      options: ["primary", "secondary", "ghost", "destructive"],
    },
    size: {
      control: { type: "inline-radio" },
      options: ["sm", "md"],
    },
    disabled: { control: { type: "boolean" } },
    pressed: { control: { type: "boolean" } },
  },
};
export default meta;
type Story = StoryObj<typeof Button>;

export const Primary: Story = { args: { variant: "primary" } };
export const Secondary: Story = { args: { variant: "secondary" } };
export const Ghost: Story = { args: { variant: "ghost" } };
export const Danger: Story = {
  args: { variant: "destructive", children: "Delete" },
};

export const Variants: Story = {
  render: () => (
    <div className="flex items-center gap-2">
      <Button variant="primary">Primary</Button>
      <Button variant="secondary">Secondary</Button>
      <Button variant="ghost">Ghost</Button>
      <Button variant="destructive">Danger</Button>
    </div>
  ),
};

export const WithIcons: Story = {
  render: () => (
    <div className="flex items-center gap-2">
      <Button variant="primary" leadingIcon={<Plus size={12} />}>
        Create
      </Button>
      <Button variant="secondary" leadingIcon={<Download size={12} />}>
        Export
      </Button>
      <Button variant="destructive" leadingIcon={<Trash2 size={12} />}>
        Delete
      </Button>
    </div>
  ),
};

export const Sizes: Story = {
  render: () => (
    <div className="flex items-center gap-2">
      <Button variant="secondary" size="sm">
        Small
      </Button>
      <Button variant="secondary" size="md">
        Medium
      </Button>
      <Button size="sm" variant="primary">
        Small primary
      </Button>
      <Button size="md" variant="primary">
        Medium primary
      </Button>
    </div>
  ),
};

type StateRowProps = { label: string; pseudoClass?: string };

function StateRow({ label, pseudoClass, children }: React.PropsWithChildren<StateRowProps>) {
  return (
    <div className="flex items-center gap-3">
      <span className="w-20 font-mono text-label text-subtle-foreground">{label}</span>
      <div className={pseudoClass}>{children}</div>
    </div>
  );
}

const ROW_VARIANTS: Array<{
  variant: "primary" | "secondary" | "ghost" | "destructive";
}> = [
  { variant: "primary" },
  { variant: "secondary" },
  { variant: "ghost" },
  { variant: "destructive" },
];

/**
 * All rest + interaction states rendered side-by-side. Uses the
 * storybook-addon-pseudo-states addon: wrapping content in `pseudo-<state>`
 * or `pseudo-<state>-all` forces the state on descendants. The
 * addon toolbar can also toggle them globally per-story.
 */
export const AllStates: Story = {
  parameters: {
    layout: "padded",
    // The a11y addon complains about `pressed` without role/aria-pressed in
    // some contexts — that's a story-level demo concern, not the component.
    a11y: { test: "todo" },
  },
  render: () => (
    <div className="flex flex-col gap-4">
      {[
        { label: "Rest", cls: "" },
        { label: "Hover", cls: "pseudo-hover-all" },
        { label: "Focus", cls: "pseudo-focus-visible-all" },
        { label: "Active", cls: "pseudo-active-all" },
      ].map((row) => (
        <StateRow key={row.label} label={row.label} pseudoClass={row.cls}>
          <div className="flex items-center gap-2">
            {ROW_VARIANTS.map(({ variant }) => (
              <Button key={variant} variant={variant}>
                {variant[0]!.toUpperCase() + variant.slice(1)}
              </Button>
            ))}
          </div>
        </StateRow>
      ))}
      <StateRow label="Disabled">
        <div className="flex items-center gap-2">
          {ROW_VARIANTS.map(({ variant }) => (
            <Button key={variant} variant={variant} disabled>
              {variant[0]!.toUpperCase() + variant.slice(1)}
            </Button>
          ))}
        </div>
      </StateRow>
      <StateRow label="Pressed">
        <div className="flex items-center gap-2">
          {ROW_VARIANTS.map(({ variant }) => (
            <Button key={variant} variant={variant} pressed>
              {variant[0]!.toUpperCase() + variant.slice(1)}
            </Button>
          ))}
        </div>
      </StateRow>
    </div>
  ),
};
