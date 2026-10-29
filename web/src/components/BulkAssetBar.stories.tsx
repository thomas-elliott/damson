import { useEffect, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, userEvent, waitFor, within } from "storybook/test";

import {
  NotificationProvider,
  useNotifications,
} from "@/components/notifications/NotificationProvider";
import type { CollectionView } from "@/hooks/useDataQueries";
import { contrastDebt } from "@/design-system/_a11yDebt";

import { BulkAssetBar } from "./BulkAssetBar";

const meta: Meta<typeof BulkAssetBar> = {
  title: "App/BulkAssetBar",
  component: BulkAssetBar,
  parameters: { layout: "fullscreen" },
  beforeEach: () => {
    window.localStorage.removeItem("damson.notifications");
  },
};
export default meta;
type Story = StoryObj<typeof BulkAssetBar>;

const collection = (id: number, name: string): CollectionView => ({
  id,
  name,
  description: "",
  asset_count: 0,
  created_at: "2026-10-01T09:00:00Z",
  updated_at: "2026-10-01T09:00:00Z",
});

const COLLECTIONS = [collection(1, "Props"), collection(2, "Ashfall shortlist")];

// Any selection that includes this id makes the bulk request fail.
const FAILING_ID = 500;

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Serves the collection list for the picker and the bulk endpoint itself.
const realFetch = window.fetch;
window.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
  const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
  const path = raw.startsWith("http") ? new URL(raw).pathname : raw.split("?")[0]!;
  if (path === "/api/collections") return json({ collections: COLLECTIONS });
  if (path === "/api/assets/bulk" && typeof init?.body === "string") {
    const { asset_ids } = JSON.parse(init.body) as { asset_ids: number[] };
    if (asset_ids.includes(FAILING_ID)) return json({ error: "storage unavailable" }, 500);
    return json({ affected: asset_ids.length });
  }
  if (path.startsWith("/api/")) return json({});
  return realFetch(input, init);
};

function OpenCentre() {
  const { setOpen } = useNotifications();
  useEffect(() => setOpen(true), [setOpen]);
  return null;
}

function Harness({ assetIds, showCentre }: { assetIds: number[]; showCentre?: boolean }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  const [selected, setSelected] = useState(assetIds);
  return (
    <QueryClientProvider client={client}>
      <NotificationProvider>
        {showCentre && <OpenCentre />}
        <div className="relative h-[480px] w-full bg-background">
          <BulkAssetBar assetIds={selected} onClear={() => setSelected([])} />
        </div>
      </NotificationProvider>
    </QueryClientProvider>
  );
}

/** Three assets selected in the library grid. */
export const Default: Story = {
  parameters: {
    // Known contrast debt: destructive on destructive-soft, 4.42:1 (SMO-22).
    a11y: contrastDebt("destructive-soft"),
  },
  render: () => <Harness assetIds={[1, 2, 3]} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    // The bar pops up from transparent, so wait for it to settle.
    await waitFor(() => expect(canvas.getByText("3 selected")).toBeVisible());
    await expect(canvas.getByRole("button", { name: "Delete" })).toBeVisible();
  },
};

/** The tag dialog, opened from the bar. */
export const TagDialog: Story = {
  parameters: {
    // Known contrast debt: destructive on destructive-soft, 4.42:1 (SMO-22).
    a11y: contrastDebt("destructive-soft"),
  },
  render: () => <Harness assetIds={[1, 2, 3]} />,
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByRole("button", { name: "Tag" }));
    await expect(await body.findByRole("dialog", { name: "Tag 3 assets" })).toBeVisible();
    await expect(body.getByRole("button", { name: "Add tag" })).toBeDisabled();
  },
};

/** The collection dialog, with the library's collections in the picker. */
export const CollectionDialog: Story = {
  parameters: {
    // Known contrast debt: destructive on destructive-soft, 4.42:1 (SMO-22).
    a11y: contrastDebt("destructive-soft"),
  },
  render: () => <Harness assetIds={[1, 2]} />,
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByRole("button", { name: "Collection" }));
    await expect(
      await body.findByRole("dialog", { name: "Add 2 assets to a collection" }),
    ).toBeVisible();
  },
};

/** Delete asks first. */
export const ConfirmDelete: Story = {
  parameters: {
    // Known contrast debt: destructive on destructive-soft, 4.42:1 (SMO-22).
    a11y: contrastDebt("destructive-soft"),
  },
  render: () => <Harness assetIds={[1, 2, 3]} />,
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByRole("button", { name: "Delete" }));
    await expect(await body.findByText("Delete 3 assets?")).toBeVisible();
  },
};

/** A failed bulk action keeps the selection and reports in the notification centre. */
export const Failed: Story = {
  parameters: {
    // Known contrast debt: destructive on destructive-soft, 4.42:1 (SMO-22);
    // the notification's timestamp, faint-foreground on card, 4.34:1 (SMO-3 finding 5).
    a11y: contrastDebt("destructive-soft", "faint-foreground"),
  },
  render: () => <Harness assetIds={[1, FAILING_ID]} showCentre />,
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByRole("button", { name: "Mark reviewed" }));
    await expect(await body.findByText("Bulk action failed")).toBeVisible();
    await waitFor(() => expect(body.getByText("2 selected")).toBeVisible());
  },
};
