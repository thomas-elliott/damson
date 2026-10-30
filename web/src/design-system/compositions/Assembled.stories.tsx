import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";
import {
  Boxes,
  FileSearch,
  FolderGit2,
  Inbox,
  Library,
  Maximize2,
  MoreHorizontal,
  Pin,
  Plus,
  Search,
  Settings,
  Trash2,
} from "lucide-react";

import { Button } from "@/design-system/primitives/Button";
import { IconButton } from "@/design-system/primitives/IconButton";
import { Pill } from "@/design-system/primitives/Pill";
import { SearchInput } from "@/design-system/primitives/SearchInput";
import { SegmentedControl } from "@/design-system/primitives/SegmentedControl";
import { Separator } from "@/design-system/primitives/Separator";
import { StatusDot } from "@/design-system/primitives/StatusDot";
import { Tag } from "@/design-system/primitives/Tag";
import { Chip } from "@/design-system/primitives/Chip";
import { TooltipProvider } from "@/design-system/primitives/Tooltip";

import { AppShell } from "./AppShell";
import { Breadcrumb, BreadcrumbItem, BreadcrumbSeparator } from "./Breadcrumb";
import { DetailPanel } from "./DetailPanel";
import { FilterRow, FilterSidebar } from "./FilterSidebar";
import { NavRail, NavRailItem, NavRailSpacer } from "./NavRail";
import { ResizeHandle } from "./ResizeHandle";
import { Section } from "./Section";
import { SelectionBar } from "./SelectionBar";
import { Toolbar, ToolbarSpacer } from "./Toolbar";

const meta: Meta = {
  title: "Compositions/Assembled",
  parameters: { layout: "fullscreen" },
  decorators: [
    (Story) => (
      <TooltipProvider>
        <Story />
      </TooltipProvider>
    ),
  ],
};
export default meta;
type Story = StoryObj;

function DamsonBrand() {
  return (
    <div className="grid h-[30px] w-[30px] place-items-center rounded-md bg-gradient-to-br from-primary to-primary-hover font-mono text-ui font-bold text-primary-foreground">
      D
    </div>
  );
}

function PlaceholderCard() {
  return (
    <div className="flex flex-col gap-2">
      <div className="aspect-square rounded-md border border-border bg-muted" />
      <div className="flex items-center justify-between">
        <span className="truncate text-ui font-medium text-foreground">Asset name</span>
        <StatusDot tone="ok" />
      </div>
      <span className="text-mono text-subtle-foreground">12.4 MB</span>
    </div>
  );
}

export const LibraryView: Story = {
  render: function LibraryViewDemo() {
    const [route, setRoute] = useState("library");
    const [view, setView] = useState<"grid" | "list">("grid");
    const [sidebarW, setSidebarW] = useState(260);
    const [selected, setSelected] = useState(2);
    const [detailOpen, setDetailOpen] = useState(true);

    return (
      <AppShell
        sidebarWidth={sidebarW}
        rail={
          <NavRail brand={<DamsonBrand />}>
            <NavRailItem
              label="Library"
              icon={<Library size={16} />}
              active={route === "library"}
              onSelect={() => setRoute("library")}
            />
            <NavRailItem
              label="Projects"
              icon={<FolderGit2 size={16} />}
              active={route === "projects"}
              onSelect={() => setRoute("projects")}
            />
            <NavRailItem
              label="Collections"
              icon={<Boxes size={16} />}
              active={route === "collections"}
              onSelect={() => setRoute("collections")}
            />
            <NavRailItem
              label="Import queue"
              icon={<Inbox size={16} />}
              badge={3}
              active={route === "queue"}
              onSelect={() => setRoute("queue")}
            />
            <NavRailItem
              label="Audit log"
              icon={<FileSearch size={16} />}
              active={route === "audit"}
              onSelect={() => setRoute("audit")}
            />
            <NavRailSpacer />
            <NavRailItem
              label="Settings"
              icon={<Settings size={16} />}
              active={route === "settings"}
              onSelect={() => setRoute("settings")}
            />
          </NavRail>
        }
        sidebar={
          <div className="relative h-full">
            <FilterSidebar header={<SearchInput kbd="⌘K" aria-label="Search assets" />}>
              <Section title="Licence">
                <div className="flex flex-col gap-0.5">
                  <FilterRow label="Compliant" count={124} dot={<StatusDot tone="ok" />} active />
                  <FilterRow label="Needs review" count={18} dot={<StatusDot tone="warn" />} />
                  <FilterRow label="Violation" count={3} dot={<StatusDot tone="err" />} />
                  <FilterRow label="Unknown" count={42} dot={<StatusDot tone="unk" />} />
                </div>
              </Section>
              <Section title="File type">
                <div className="flex flex-col gap-0.5">
                  <FilterRow label=".unitypackage" count={52} />
                  <FilterRow label="Audio" count={37} />
                  <FilterRow label="Font" count={22} />
                  <FilterRow label="Sprite" count={110} />
                </div>
              </Section>
              <Section title="Tags" defaultOpen={false}>
                <div className="text-caption">(no selected tags)</div>
              </Section>
            </FilterSidebar>
            <div className="absolute inset-y-0 right-0 z-resizer">
              <ResizeHandle
                value={sidebarW}
                onChange={setSidebarW}
                min={200}
                max={380}
                aria-label="Resize sidebar"
              />
            </div>
          </div>
        }
        main={
          <>
            <Toolbar>
              <Breadcrumb count="187 assets">
                <BreadcrumbItem>Library</BreadcrumbItem>
                <BreadcrumbSeparator />
                <BreadcrumbItem current>Compliant</BreadcrumbItem>
              </Breadcrumb>
              <Chip
                label="licence:"
                onRemove={() => {}}
                removeLabel="Remove licence filter: Compliant"
              >
                Compliant
              </Chip>
              <Button variant="ghost" size="sm">
                Clear
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
            <div className="relative min-h-0 flex-1 overflow-y-auto p-3.5">
              <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
                {Array.from({ length: 12 }, (_, i) => (
                  <PlaceholderCard key={i} />
                ))}
              </div>
              <SelectionBar
                count={selected}
                actions={
                  <>
                    <IconButton size="sm" aria-label="Pin">
                      <Pin />
                    </IconButton>
                    <IconButton size="sm" aria-label="Delete" onClick={() => setSelected(0)}>
                      <Trash2 />
                    </IconButton>
                    <IconButton size="sm" aria-label="More">
                      <MoreHorizontal />
                    </IconButton>
                  </>
                }
              />
              {detailOpen && (
                <DetailPanel
                  title="Stylised Sprite Pack v1.2"
                  actions={
                    <IconButton size="sm" aria-label="Expand">
                      <Maximize2 />
                    </IconButton>
                  }
                  onClose={() => setDetailOpen(false)}
                >
                  <div className="flex flex-col gap-4">
                    <div className="aspect-video rounded-md border border-border bg-muted" />
                    <div className="flex flex-wrap items-center gap-2">
                      <Pill mono tone="ok">
                        CC-BY-4.0
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
                      <Button variant="primary" leadingIcon={<Pin size={12} />}>
                        Pin to project
                      </Button>
                      <Button variant="secondary" leadingIcon={<Plus size={12} />}>
                        Add to collection
                      </Button>
                    </div>
                  </div>
                </DetailPanel>
              )}
            </div>
          </>
        }
      />
    );
  },
};
