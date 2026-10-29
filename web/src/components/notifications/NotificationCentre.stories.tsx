import { useEffect, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, userEvent, within } from "storybook/test";

import { contrastDebt } from "@/design-system/_a11yDebt";
import { holdXhrUploads } from "@/components/_storyStubs";

import { NotificationCentre } from "./NotificationCentre";
import {
  NotificationProvider,
  useNotifications,
  type AppNotification,
} from "./NotificationProvider";

/**
 * The centre as the rail's bell opens it. The provider renders it, reads the
 * persisted list from localStorage, and uploads go over XHR, so each story
 * seeds storage first and the module holds every upload open.
 */
const meta: Meta<typeof NotificationCentre> = {
  title: "App/NotificationCentre",
  component: NotificationCentre,
  parameters: { layout: "fullscreen" },
  beforeEach: () => holdXhrUploads(),
};
export default meta;
type Story = StoryObj<typeof NotificationCentre>;

const STORAGE_KEY = "damson.notifications";
const MINUTE = 60 * 1000;

function seed(list: Omit<AppNotification, "createdAt">[], agesInMinutes: number[]) {
  const now = Date.now();
  const stored: AppNotification[] = list.map((n, i) => ({
    ...n,
    createdAt: now - (agesInMinutes[i] ?? 0) * MINUTE,
    ...(n.read ? { readAt: now } : {}),
  }));
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(stored));
}

const UNREAD: Omit<AppNotification, "createdAt">[] = [
  { id: "n1", tone: "success", title: "Uploaded Brick_Albedo.png", read: false, source: "upload" },
  {
    id: "n2",
    tone: "warn",
    title: "Castle.unitypackage already exists on that asset",
    message: "version v2.1 has the same contents",
    read: false,
    source: "upload",
  },
  {
    id: "n3",
    tone: "error",
    title: "Bulk action failed",
    message: "storage unavailable",
    read: false,
    source: "toast",
  },
];

const READ: Omit<AppNotification, "createdAt">[] = [
  {
    id: "r1",
    tone: "info",
    title: "Licence detected for Kenney Nature Kit",
    read: true,
    source: "toast",
  },
  { id: "r2", tone: "success", title: "Tagged 12 assets", read: true, source: "toast" },
];

function Open({ files }: { files?: File[] }) {
  const { setOpen, startUpload } = useNotifications();
  useEffect(() => {
    setOpen(true);
    if (files) startUpload(files);
    // Mount-only: the stories open the centre once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return null;
}

function Harness({ files }: { files?: File[] }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <NotificationProvider>
        <div className="h-[640px] w-full bg-background" />
        <Open files={files} />
      </NotificationProvider>
    </QueryClientProvider>
  );
}

/** Nothing new. */
export const Empty: Story = {
  beforeEach: () => {
    window.localStorage.removeItem(STORAGE_KEY);
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("No new notifications")).toBeVisible();
  },
};

/** One of each tone waiting to be read. */
export const Unread: Story = {
  parameters: {
    // Known contrast debt: timestamps, faint-foreground on card, 4.34:1 (SMO-3 finding 5).
    a11y: contrastDebt("faint-foreground"),
  },
  beforeEach: () => {
    seed([...UNREAD, ...READ], [0, 4, 18, 90, 60 * 26]);
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Bulk action failed")).toBeVisible();
    await expect(canvas.getAllByRole("button", { name: "Mark read" })).toHaveLength(3);
  },
};

/** The Read tab: no Mark read, and its own Clear all. */
export const Read: Story = {
  parameters: {
    // Known contrast debt: timestamps, faint-foreground on card, 4.34:1 (SMO-3 finding 5).
    a11y: contrastDebt("faint-foreground"),
  },
  beforeEach: () => {
    seed(READ, [90, 60 * 26]);
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("tab", { name: "Read" }));
    await expect(await canvas.findByText("Tagged 12 assets")).toBeVisible();
    await expect(canvas.queryByRole("button", { name: "Mark read" })).not.toBeInTheDocument();
  },
};

/** Uploads in flight get their own tab. Past three, the rest queue. */
export const Uploads: Story = {
  beforeEach: () => {
    window.localStorage.removeItem(STORAGE_KEY);
  },
  render: () => (
    <Harness
      files={["Brick_Albedo.png", "Castle.unitypackage", "Rocks.fbx", "Footsteps.wav"].map(
        (name) => new File([new Uint8Array(2048)], name),
      )}
    />
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("tab", { name: /Uploads/ }));
    await expect(await canvas.findByText("Footsteps.wav")).toBeVisible();
    await expect(canvas.getByText(/Queued/)).toBeVisible();
  },
};
