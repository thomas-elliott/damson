import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";
import {
  Copy,
  Download,
  ExternalLink,
  Inbox,
  Maximize2,
  MoreHorizontal,
  Pin,
  Plus,
  Search,
  Trash2,
} from "lucide-react";

import { Button } from "@/design-system/primitives/Button";
import { IconButton } from "@/design-system/primitives/IconButton";
import { Pill } from "@/design-system/primitives/Pill";
import { SegmentedControl } from "@/design-system/primitives/SegmentedControl";
import { Separator } from "@/design-system/primitives/Separator";
import { StatusDot } from "@/design-system/primitives/StatusDot";
import { Chip } from "@/design-system/primitives/Chip";
import { Tag } from "@/design-system/primitives/Tag";

import { Breadcrumb, BreadcrumbItem, BreadcrumbSeparator } from "./Breadcrumb";
import { DetailPanel } from "./DetailPanel";
import { DropOverlay } from "./DropOverlay";
import { EmptyState } from "./EmptyState";
import { FullPageView } from "./FullPageView";
import { SelectionBar } from "./SelectionBar";
import { Toolbar, ToolbarSpacer } from "./Toolbar";

const meta: Meta = {
  title: "Compositions/Content",
  parameters: { layout: "fullscreen" },
};
export default meta;
type Story = StoryObj;

export const ToolbarExample: Story = {
  parameters: { layout: "centered" },
  render: function ToolbarExampleDemo() {
    const [view, setView] = useState<"grid" | "list">("grid");
    return (
      <div className="w-[720px] border border-border bg-background">
        <Toolbar>
          <Breadcrumb count="254 assets">
            <BreadcrumbItem>Library</BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem current>Sprites</BreadcrumbItem>
          </Breadcrumb>
          <Chip label="licence:" onRemove={() => {}}>
            MIT
          </Chip>
          <Chip label="tag:" onRemove={() => {}}>
            ui
          </Chip>
          <Button variant="ghost" size="sm">
            Clear all
          </Button>
          <ToolbarSpacer />
          <Button variant="ghost" size="sm" leadingIcon={<Search size={12} />}>
            Sort
          </Button>
          <Separator orientation="vertical" />
          <SegmentedControl
            value={view}
            onValueChange={setView}
            options={[
              {
                value: "grid",
                "aria-label": "Grid view",
                icon: <span className="text-mono">▦</span>,
              },
              {
                value: "list",
                "aria-label": "List view",
                icon: <span className="text-mono">☰</span>,
              },
            ]}
          />
        </Toolbar>
      </div>
    );
  },
};

export const DetailPanelExample: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="relative h-[560px] w-[460px] border border-border bg-background">
      <DetailPanel
        variant="flush"
        title="Stylised Sprite Pack v1.2"
        actions={
          <>
            <IconButton aria-label="Expand">
              <Maximize2 size={14} />
            </IconButton>
            <IconButton aria-label="More">
              <MoreHorizontal size={14} />
            </IconButton>
          </>
        }
        onClose={() => {}}
      >
        <div className="flex flex-col gap-4">
          <div className="aspect-video rounded-md border border-border bg-muted" />
          <div className="flex items-center gap-2">
            <Pill mono tone="ok">
              <StatusDot tone="ok" /> CC-BY-4.0
            </Pill>
            <Tag>sprites</Tag>
            <Tag>ui/icons</Tag>
            <Tag variant="add">Add tag</Tag>
          </div>
          <dl className="grid grid-cols-[90px_1fr] gap-x-3 gap-y-2 text-small">
            <dt className="text-subtle-foreground">Source</dt>
            <dd className="text-muted-foreground">itch.io · craftpix</dd>
            <dt className="text-subtle-foreground">Imported</dt>
            <dd className="text-muted-foreground">3 days ago</dd>
            <dt className="text-subtle-foreground">Size</dt>
            <dd className="font-mono text-muted-foreground">42.8 MB</dd>
            <dt className="text-subtle-foreground">Files</dt>
            <dd className="font-mono text-muted-foreground">128</dd>
          </dl>
          <div className="flex items-center gap-1.5">
            <Button leadingIcon={<Download size={12} />}>Download</Button>
            <Button leadingIcon={<Pin size={12} />}>Pin</Button>
            <Button leadingIcon={<Copy size={12} />}>Duplicate</Button>
          </div>
        </div>
      </DetailPanel>
    </div>
  ),
};

export const FullPageViewExample: Story = {
  render: () => (
    <div className="h-screen">
      <FullPageView
        header={
          <>
            <Breadcrumb>
              <BreadcrumbItem>Library</BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem current>Stylised Sprite Pack v1.2</BreadcrumbItem>
            </Breadcrumb>
            <ToolbarSpacer />
            <Button leadingIcon={<ExternalLink size={12} />}>Source</Button>
            <IconButton aria-label="Close">
              <Maximize2 size={14} />
            </IconButton>
          </>
        }
        preview={<div className="aspect-[16/9] rounded-lg border border-border bg-muted" />}
        side={
          <div className="flex flex-col gap-4">
            <div className="flex items-center gap-2">
              <Pill mono tone="ok">
                <StatusDot tone="ok" /> CC-BY-4.0
              </Pill>
              <Tag>sprites</Tag>
              <Tag>ui/icons</Tag>
            </div>
            <p className="text-ui text-muted-foreground">
              A 128-file sprite pack with 2× and 3× variants. Compliant for commercial use with
              attribution.
            </p>
          </div>
        }
      />
    </div>
  ),
};

export const DropOverlayExample: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="relative h-[320px] w-[520px] overflow-hidden border border-border bg-background p-4 text-ui text-subtle-foreground">
      Dropping files would show:
      <DropOverlay active caption="Drop files to import" />
    </div>
  ),
};

export const SelectionBarExample: Story = {
  parameters: { layout: "centered" },
  render: function SelectionBarExampleDemo() {
    const [count, setCount] = useState(3);
    return (
      <div className="relative h-[240px] w-[640px] overflow-hidden border border-border bg-background p-4 text-ui text-subtle-foreground">
        <div className="flex items-center gap-2">
          <Button onClick={() => setCount((c) => c + 1)}>Select more</Button>
          <Button onClick={() => setCount(0)}>Clear selection</Button>
        </div>
        <SelectionBar
          count={count}
          actions={
            <>
              <IconButton aria-label="Pin">
                <Pin size={14} />
              </IconButton>
              <IconButton aria-label="Delete">
                <Trash2 size={14} />
              </IconButton>
              <IconButton aria-label="More">
                <MoreHorizontal size={14} />
              </IconButton>
            </>
          }
        />
      </div>
    );
  },
};

export const EmptyStateExample: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="h-[360px] w-[640px] border border-border bg-background">
      <EmptyState
        icon={<Inbox size={32} />}
        title="Nothing here yet"
        description="Drop .unitypackage files, sprites, audio, or fonts onto this view to import them into your library."
        action={
          <Button variant="primary" leadingIcon={<Plus size={12} />}>
            Import
          </Button>
        }
      />
    </div>
  ),
};
