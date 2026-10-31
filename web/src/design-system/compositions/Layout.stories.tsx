import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";
import { Box, Boxes, FileSearch, FolderGit2, Inbox, Library, Settings } from "lucide-react";

import { SearchInput } from "@/design-system/primitives/SearchInput";
import { StatusDot } from "@/design-system/primitives/StatusDot";
import { TooltipProvider } from "@/design-system/primitives/Tooltip";

import { AppShell } from "./AppShell";
import { BrandMark } from "./BrandMark";
import { FilterRow, FilterSidebar } from "./FilterSidebar";
import { Breadcrumb, BreadcrumbItem } from "./Breadcrumb";
import { NavRail, NavRailItem, NavRailSpacer } from "./NavRail";
import { PageContent } from "./PageContent";
import { ResizeHandle } from "./ResizeHandle";
import { Section } from "./Section";
import { Toolbar } from "./Toolbar";

const meta: Meta = {
  title: "Compositions/Layout",
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

function RailSample({ route, setRoute }: { route: string; setRoute: (r: string) => void }) {
  return (
    <NavRail brand={<BrandMark />}>
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
        label="Triage"
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
  );
}

function SidebarSample() {
  return (
    <FilterSidebar header={<SearchInput kbd="⌘K" aria-label="Search" />}>
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
          <FilterRow icon={<Box size={12} />} label=".unitypackage" count={52} />
          <FilterRow label="Audio" count={37} />
          <FilterRow label="Font" count={22} />
          <FilterRow label="Sprite" count={110} />
        </div>
      </Section>
      <Section title="Tags" defaultOpen={false}>
        <div className="text-caption">(no selected tags)</div>
      </Section>
    </FilterSidebar>
  );
}

export const AppShellWithSidebar: Story = {
  render: function AppShellWithSidebarDemo() {
    const [route, setRoute] = useState("library");
    const [sidebarW, setSidebarW] = useState(248);
    return (
      <div className="relative">
        <AppShell
          sidebarWidth={sidebarW}
          rail={<RailSample route={route} setRoute={setRoute} />}
          sidebar={
            <div className="relative h-full">
              <SidebarSample />
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
            <div className="grid h-full place-items-center text-ui text-subtle-foreground">
              Main content (route: {route}, sidebar: {sidebarW}px)
            </div>
          }
        />
      </div>
    );
  },
};

export const RailOnly: Story = {
  render: function RailOnlyDemo() {
    const [route, setRoute] = useState("library");
    return (
      <AppShell
        rail={<RailSample route={route} setRoute={setRoute} />}
        main={
          <div className="grid h-full place-items-center text-ui text-subtle-foreground">
            Main content only (route: {route})
          </div>
        }
      />
    );
  },
};

// A capped column under a full-bleed toolbar: the content starts on the
// toolbar's edge rather than re-centring.
export const PageContentWide: Story = {
  render: function PageContentWideDemo() {
    const [route, setRoute] = useState("audit");
    return (
      <AppShell
        rail={<RailSample route={route} setRoute={setRoute} />}
        main={
          <>
            <Toolbar>
              <Breadcrumb>
                <BreadcrumbItem current>Audit log</BreadcrumbItem>
              </Breadcrumb>
            </Toolbar>
            <PageContent width="wide" className="gap-2">
              {["Oct 29, 01:10 PM", "Oct 29, 11:58 AM", "Oct 28, 04:02 PM"].map((t) => (
                <div key={t} className="rounded-md border border-border bg-card p-3 text-ui">
                  {t}
                </div>
              ))}
            </PageContent>
          </>
        }
      />
    );
  },
};
