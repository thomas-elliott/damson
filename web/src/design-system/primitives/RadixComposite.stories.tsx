import type { Meta, StoryObj } from "@storybook/react";
import { useState } from "react";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./Select";
import { TabCount, Tabs, TabsContent, TabsList, TabsTrigger } from "./Tabs";
import {
  SimpleTooltip,
  TooltipProvider,
} from "./Tooltip";

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
      <TabsContent value="assets" className="pt-4 text-sm-plus text-text-2">
        Assets pinned to this project.
      </TabsContent>
      <TabsContent value="manifest" className="pt-4 text-sm-plus text-text-2">
        The project manifest JSON.
      </TabsContent>
      <TabsContent value="activity" className="pt-4 text-sm-plus text-text-2">
        Recent activity.
      </TabsContent>
    </Tabs>
  ),
};

export const TooltipExample: Story = {
  render: () => (
    <div className="flex items-center gap-4">
      <SimpleTooltip label="Library" side="bottom">
        <IconButton aria-label="Library">
          <Settings size={14} />
        </IconButton>
      </SimpleTooltip>
      <SimpleTooltip label="Download asset" side="right">
        <IconButton aria-label="Download">
          <Download size={14} />
        </IconButton>
      </SimpleTooltip>
    </div>
  ),
};

export const DropdownMenuExample: Story = {
  render: () => {
    const [showSub, setShowSub] = useState(true);
    return (
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button>
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
            <Trash2 size={12} className="text-err" /> Remove sort
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
        <IconButton aria-label="More">
          <MoreHorizontal size={14} />
        </IconButton>
      </PopoverTrigger>
      <PopoverContent className="min-w-[240px]">
        <div className="flex flex-col gap-2">
          <div className="text-2xs uppercase tracking-wider text-text-4">
            Quick info
          </div>
          <p className="text-xs-plus text-text-2">
            Drop-in popover container. Use for rich content (filters, details)
            that needs more room than a dropdown menu.
          </p>
        </div>
      </PopoverContent>
    </Popover>
  ),
};

export const SelectExample: Story = {
  render: () => {
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
            Projects group assets pinned to a specific game. You can always
            rename or move them later.
          </DialogDescription>
          <label className="flex flex-col gap-1.5 text-xs-plus">
            <span className="text-text-4">Name</span>
            <Input placeholder="Crystal Quest" autoFocus />
          </label>
          <label className="flex flex-col gap-1.5 text-xs-plus">
            <span className="text-text-4">Path</span>
            <Input placeholder="~/Dev/crystal-quest" />
          </label>
        </DialogBody>
        <DialogFooter>
          <DialogClose asChild>
            <Button>Cancel</Button>
          </DialogClose>
          <Button variant="primary">Create project</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  ),
};
