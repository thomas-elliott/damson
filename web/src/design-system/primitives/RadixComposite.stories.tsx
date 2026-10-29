import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { ChevronDown, Download, MoreHorizontal, Settings, Trash2 } from "lucide-react";

import { Button } from "./Button";
import { IconButton } from "./IconButton";
import {
  Dialog,
  DialogBody,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "./Dialog";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "./DropdownMenu";
import { Input } from "./Input";
import { Popover, PopoverContent, PopoverTrigger } from "./Popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./Select";
import { TabCount, Tabs, TabsContent, TabsList, TabsTrigger } from "./Tabs";
import { SimpleTooltip, TooltipProvider } from "./Tooltip";

const meta: Meta = {
  title: "Primitives/RadixComposite",
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

export const TabsExample: Story = {
  render: () => (
    <Tabs defaultValue="assets" className="min-w-[440px]">
      <TabsList>
        <TabsTrigger value="assets">
          Assets <TabCount>24</TabCount>
        </TabsTrigger>
        <TabsTrigger value="manifest">Manifest</TabsTrigger>
        <TabsTrigger value="activity">
          Activity <TabCount>7</TabCount>
        </TabsTrigger>
      </TabsList>
      <TabsContent value="assets" className="pt-4 text-ui text-muted-foreground">
        Assets pinned to this project.
      </TabsContent>
      <TabsContent value="manifest" className="pt-4 text-ui text-muted-foreground">
        The project manifest JSON.
      </TabsContent>
      <TabsContent value="activity" className="pt-4 text-ui text-muted-foreground">
        Recent activity.
      </TabsContent>
    </Tabs>
  ),
};

export const TooltipExample: Story = {
  render: () => (
    <div className="flex items-center gap-4">
      <SimpleTooltip label="Library" side="bottom">
        <IconButton size="sm" aria-label="Library">
          <Settings />
        </IconButton>
      </SimpleTooltip>
      <SimpleTooltip label="Download asset" side="right">
        <IconButton size="sm" aria-label="Download">
          <Download />
        </IconButton>
      </SimpleTooltip>
    </div>
  ),
};

export const DropdownMenuExample: Story = {
  render: function DropdownMenuExampleDemo() {
    const [showSub, setShowSub] = useState(true);
    return (
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="secondary">
            Sort <ChevronDown size={12} />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuLabel>Sort by</DropdownMenuLabel>
          <DropdownMenuItem>
            Name <DropdownMenuShortcut>A→Z</DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuItem>
            Imported <DropdownMenuShortcut>Newest</DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuItem>
            Size <DropdownMenuShortcut>Largest</DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuCheckboxItem
            checked={showSub}
            onCheckedChange={(v) => setShowSub(v === true)}
          >
            Show subfolders
          </DropdownMenuCheckboxItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem>
            <Trash2 size={12} className="text-destructive" /> Remove sort
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
  },
};

export const PopoverExample: Story = {
  render: () => (
    <Popover>
      <PopoverTrigger asChild>
        <IconButton size="sm" aria-label="More">
          <MoreHorizontal />
        </IconButton>
      </PopoverTrigger>
      <PopoverContent className="min-w-[240px]">
        <div className="flex flex-col gap-2">
          <div className="text-overline text-faint-foreground">Quick info</div>
          <p className="text-label text-muted-foreground">
            Drop-in popover container. Use for rich content (filters, details) that needs more room
            than a dropdown menu.
          </p>
        </div>
      </PopoverContent>
    </Popover>
  ),
};

export const SelectExample: Story = {
  render: function SelectExampleDemo() {
    const [v, setV] = useState("name");
    return (
      <Select value={v} onValueChange={setV}>
        <SelectTrigger className="min-w-[180px]" aria-label="Sort by">
          <SelectValue placeholder="Sort by…" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="name">Name</SelectItem>
          <SelectItem value="imported">Imported</SelectItem>
          <SelectItem value="size">Size</SelectItem>
          <SelectItem value="licence">Licence</SelectItem>
        </SelectContent>
      </Select>
    );
  },
};

export const DialogExample: Story = {
  render: () => (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="primary">New project</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New project</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <DialogDescription>
            Projects group assets pinned to a specific game. You can always rename or move them
            later.
          </DialogDescription>
          <label className="flex flex-col gap-1.5 text-label">
            <span className="text-faint-foreground">Name</span>
            <Input placeholder="Crystal Quest" autoFocus />
          </label>
          <label className="flex flex-col gap-1.5 text-label">
            <span className="text-faint-foreground">Path</span>
            <Input placeholder="~/Dev/crystal-quest" />
          </label>
        </DialogBody>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="secondary">Cancel</Button>
          </DialogClose>
          <Button variant="primary">Create project</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  ),
};

// Opened from state, the way Damson's dialogs are, with a field that takes
// focus as it mounts. Closing returns focus to the button that opened it.
export const DialogFocusReturn: Story = {
  render: function DialogFocusReturnDemo() {
    const [open, setOpen] = useState(false);
    return (
      <>
        <Button variant="secondary" onClick={() => setOpen(true)}>
          Add by location
        </Button>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add by location</DialogTitle>
              <DialogDescription>Record an asset by where it lives.</DialogDescription>
            </DialogHeader>
            <DialogBody>
              <Input aria-label="Path" placeholder="/mnt/assets/…" autoFocus />
            </DialogBody>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setOpen(false)}>
                Cancel
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </>
    );
  },
  play: async ({ canvasElement }) => {
    const opener = within(canvasElement).getByRole("button", { name: "Add by location" });
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(opener);
    await expect(await body.findByLabelText("Path")).toHaveFocus();
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(opener).toHaveFocus());
    await userEvent.click(opener);
    await userEvent.click(await body.findByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(opener).toHaveFocus());
  },
};
