import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";
import {
  Box,
  Boxes,
  FileSearch,
  FolderGit2,
  Inbox,
  Library,
  Settings,
} from "lucide-react";

import { SearchInput } from "@/design-system/primitives/SearchInput";
import { StatusDot } from "@/design-system/primitives/StatusDot";
import { TooltipProvider } from "@/design-system/primitives/Tooltip";

import { AppShell } from "./AppShell";
import { FilterRow, FilterSidebar } from "./FilterSidebar";
import { NavRail, NavRailItem, NavRailSpacer } from "./NavRail";
import { ResizeHandle } from "./ResizeHandle";
import { Section } from "./Section";

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

function DamsonBrand() {
  return (
    <div className="grid h-[30px] w-[30px] place-items-center rounded-md bg-gradient-to-br from-accent to-accent-2 font-mono text-ui font-bold text-accent-fg">
      D
    </div>
  );
}

function RailSample({ route, setRoute }: { route: string; setRoute: (r: string) => void }) {
  return (
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
  render: () => {
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
            <div className="grid h-full place-items-center text-ui text-text-3">
              Main content (route: {route}, sidebar: {sidebarW}px)
            </div>
          }
        />
      </div>
    );
  },
};

export const RailOnly: Story = {
  render: () => {
    const [route, setRoute] = useState("library");
    return (
      <AppShell
        rail={<RailSample route={route} setRoute={setRoute} />}
        main={
          <div className="grid h-full place-items-center text-ui text-text-3">
            Main content only (route: {route})
          </div>
        }
      />
    );
  },
};
