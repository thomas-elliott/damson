import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";
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
import { contrastDebt } from "@/design-system/_a11yDebt";

import { Breadcrumb, BreadcrumbItem, BreadcrumbSeparator } from "./Breadcrumb";
import { useConfirm } from "./ConfirmDialog";
import { DetailPanel } from "./DetailPanel";
import { DropOverlay } from "./DropOverlay";
import { EmptyState } from "./EmptyState";
import { ErrorState } from "./ErrorState";
import { LoadingState } from "./LoadingState";
import { FullPageView } from "./FullPageView";
import { Pager } from "./Pager";
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
          <Chip label="licence:" onRemove={() => {}} removeLabel="Remove licence filter: MIT">
            MIT
          </Chip>
          <Chip label="tag:" onRemove={() => {}} removeLabel="Remove tag filter: ui">
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
            <IconButton size="sm" aria-label="Expand">
              <Maximize2 />
            </IconButton>
            <IconButton size="sm" aria-label="More">
              <MoreHorizontal />
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
          <dl className="grid grid-cols-[90px_1fr] gap-x-3 gap-y-2 text-label">
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
            <Button variant="secondary" leadingIcon={<Download size={12} />}>
              Download
            </Button>
            <Button variant="secondary" leadingIcon={<Pin size={12} />}>
              Pin
            </Button>
            <Button variant="secondary" leadingIcon={<Copy size={12} />}>
              Duplicate
            </Button>
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
            <Button variant="secondary" leadingIcon={<ExternalLink size={12} />}>
              Source
            </Button>
            <IconButton size="sm" aria-label="Close">
              <Maximize2 />
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
          <Button variant="secondary" onClick={() => setCount((c) => c + 1)}>
            Select more
          </Button>
          <Button variant="secondary" onClick={() => setCount(0)}>
            Clear selection
          </Button>
        </div>
        <SelectionBar
          count={count}
          actions={
            <>
              <IconButton size="sm" aria-label="Pin">
                <Pin />
              </IconButton>
              <IconButton size="sm" aria-label="Delete">
                <Trash2 />
              </IconButton>
              <IconButton size="sm" aria-label="More">
                <MoreHorizontal />
              </IconButton>
            </>
          }
        />
      </div>
    );
  },
};

export const PagerExample: Story = {
  parameters: { layout: "centered" },
  render: function PagerExampleDemo() {
    const [page, setPage] = useState(4);
    const [pageSize, setPageSize] = useState(50);
    return (
      <div className="flex flex-col items-start gap-4 bg-background p-4">
        <Pager
          aria-label="Interactive pagination"
          page={page}
          pageCount={9}
          onPageChange={setPage}
          pageSize={pageSize}
          pageSizes={[25, 50, 100, 200]}
          onPageSizeChange={(s) => {
            setPageSize(s);
            setPage(1);
          }}
        />
        <Pager aria-label="First page" page={1} pageCount={9} onPageChange={() => {}} />
        <Pager aria-label="Last page" page={9} pageCount={9} onPageChange={() => {}} />
        <Pager aria-label="Single page" page={1} pageCount={1} onPageChange={() => {}} />
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

export const ErrorStateExample: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="h-[360px] w-[640px] border border-border bg-background">
      <ErrorState title="Couldn't load assets" onRetry={() => {}} />
    </div>
  ),
};

export const ErrorStateWithWayOut: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="flex h-[360px] w-[640px] flex-col border border-border bg-background">
      <ErrorState
        className="flex-1"
        title="Couldn't sign you in"
        onRetry={() => {}}
        secondaryAction={<Button variant="ghost">Sign out</Button>}
      />
    </div>
  ),
};

export const LoadingStateExample: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="flex w-[640px] flex-col gap-4">
      <div className="flex h-[240px] flex-col border border-border bg-background">
        <LoadingState />
      </div>
      <div className="border border-border bg-card px-3.5">
        <LoadingState variant="inline" />
      </div>
    </div>
  ),
};

// ConfirmDialog is reached through useConfirm, as the app uses it; the play
// opens it so the a11y check runs against the open dialog.
function ConfirmDemo({ tone }: { tone: "default" | "destructive" }) {
  const { confirm, dialog } = useConfirm();
  const [answer, setAnswer] = useState<string>("No answer yet");
  return (
    <div className="flex flex-col items-start gap-3">
      <Button
        variant={tone === "destructive" ? "destructive" : "secondary"}
        onClick={async () => {
          const ok = await confirm(
            tone === "destructive"
              ? {
                  title: "Delete 3 orphaned files?",
                  description: "They aren't referenced by any asset. This can't be undone.",
                  confirmLabel: "Delete",
                  tone: "destructive",
                }
              : {
                  title: "Re-run metadata lookup?",
                  description: "The current store match will be replaced.",
                  confirmLabel: "Re-run",
                },
          );
          setAnswer(ok ? "Confirmed" : "Cancelled");
        }}
      >
        Open
      </Button>
      <p className="text-ui text-muted-foreground">{answer}</p>
      {dialog}
    </div>
  );
}

export const ConfirmDialogExample: Story = {
  parameters: { layout: "centered" },
  render: () => <ConfirmDemo tone="default" />,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: "Open" }));
    const body = within(canvasElement.ownerDocument.body);
    await expect(
      await body.findByRole("dialog", { name: "Re-run metadata lookup?" }),
    ).toBeVisible();
  },
};

export const ConfirmDialogDestructive: Story = {
  parameters: {
    layout: "centered",
    // Known contrast debt: destructive on destructive-soft, 4.42:1 (SMO-22).
    a11y: contrastDebt("destructive-soft"),
  },
  render: () => <ConfirmDemo tone="destructive" />,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: "Open" }));
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByRole("button", { name: "Delete" })).toBeVisible();
  },
};
