import type { Meta, StoryObj } from "@storybook/react";
import { Check, FileAudio2, Image, Package, X } from "lucide-react";

import { Button } from "@/design-system/primitives/Button";
import { IconButton } from "@/design-system/primitives/IconButton";
import { Input } from "@/design-system/primitives/Input";
import { Textarea } from "@/design-system/primitives/Textarea";

import { ActivityRow } from "./ActivityRow";
import { CollectionCard } from "./CollectionCard";
import { FieldRow } from "./FieldRow";
import { KeyValueList } from "./KeyValueList";
import { QueueItem } from "./QueueItem";
import { VersionItem } from "./VersionItem";

const meta: Meta = {
  title: "Domain/Misc",
};
export default meta;
type Story = StoryObj;

export const Collections: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
      <CollectionCard name="Environments" count={84} />
      <CollectionCard name="Character sheets" count={22} />
      <CollectionCard
        name="Recent imports"
        count={36}
        kind="smart"
        queryDescription="imported < 30 days"
      />
      <CollectionCard
        name="Needs review"
        count={18}
        kind="smart"
        queryDescription="licence.status = warn OR needsReview = true"
      />
    </div>
  ),
};

export const Queue: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="flex flex-col gap-2">
      <QueueItem
        thumb={<Package size={20} />}
        name="POLYGON - Fantasy Kingdom.unitypackage"
        sub={
          <>
            <span>652 MB</span>
            <span>·</span>
            <span>detected: Unity-EULA</span>
          </>
        }
        progress={42}
        actions={
          <IconButton size="sm" aria-label="Cancel upload">
            <X />
          </IconButton>
        }
      />
      <QueueItem
        thumb={<Image size={20} />}
        name="hero_run_cycle.png"
        sub={
          <>
            <span>48 KB</span>
            <span>·</span>
            <span>awaiting review</span>
          </>
        }
        actions={
          <>
            <Button variant="secondary" size="sm">
              Review
            </Button>
            <Button size="sm" variant="primary" leadingIcon={<Check size={12} />}>
              Confirm
            </Button>
          </>
        }
      />
      <QueueItem
        thumb={<FileAudio2 size={20} />}
        name="metal_impact_01.wav"
        sub={
          <>
            <span>186 KB</span>
            <span>·</span>
            <span>detected: CC-BY-4.0</span>
          </>
        }
        progress={100}
      />
    </div>
  ),
};

export const KeyValue: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="w-[360px]">
      <KeyValueList
        entries={[
          { key: "Source", value: "Unity Asset Store" },
          { key: "Imported", value: "2026-03-12" },
          { key: "Size", value: "652.6 MB", mono: true },
          { key: "Files", value: "1,284", mono: true },
          { key: "Blob", value: "sha256:ab3c…8e91", mono: true },
          {
            key: "Tags",
            value: "environment, 3d, low-poly, fantasy",
          },
        ]}
      />
    </div>
  ),
};

export const Versions: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="flex w-[440px] flex-col gap-1">
      <VersionItem
        version="1.12.3"
        date="2026-03-12"
        meta="1,284 files"
        notes="Added winter biome variants"
        current
      />
      <VersionItem
        version="1.11.0"
        date="2026-01-04"
        meta="1,210 files"
        notes="Unity 2023.3 compatibility"
        onSelect={() => {}}
      />
      <VersionItem
        version="1.10.2"
        date="2025-09-18"
        meta="1,187 files"
        notes="Bugfix: LOD0 meshes"
        onSelect={() => {}}
      />
      <VersionItem
        version="1.10.0"
        date="2025-07-22"
        meta="1,180 files"
        notes="New: buildings pack"
        onSelect={() => {}}
      />
    </div>
  ),
};

export const Activity: Story = {
  parameters: { layout: "padded" },
  render: () => (
    <div className="overflow-hidden rounded-md border border-border bg-background">
      <ActivityRow
        timestamp="2026-04-24 14:12"
        actor="librarian@example.com"
        action="pinned"
        target="POLYGON - Fantasy Kingdom"
      />
      <ActivityRow
        timestamp="2026-04-24 13:02"
        actor="librarian@example.com"
        action="updated licence to"
        target="Unity-EULA"
      />
      <ActivityRow
        timestamp="2026-04-23 22:58"
        actor="system"
        action="imported asset"
        target="hero_run_cycle.png"
      />
      <ActivityRow
        timestamp="2026-04-23 09:40"
        actor="librarian@example.com"
        action="created project"
        target="Ashfall"
      />
    </div>
  ),
};

export const Fields: Story = {
  parameters: { layout: "centered" },
  render: () => (
    <div className="flex w-[360px] flex-col gap-4">
      <FieldRow label="Name" description="The name shown across the library.">
        <Input placeholder="Untitled asset" />
      </FieldRow>
      <FieldRow label="Source URL" hint="Leave blank if imported from disk.">
        <Input placeholder="https://…" />
      </FieldRow>
      <FieldRow label="Email" error="Must be a valid email address.">
        <Input placeholder="name@example.com" defaultValue="not-an-email" />
      </FieldRow>
      <FieldRow label="Notes">
        <Textarea placeholder="Internal team notes — never shown to contributors." />
      </FieldRow>
    </div>
  ),
};
