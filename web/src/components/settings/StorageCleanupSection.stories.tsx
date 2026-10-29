import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, userEvent, within } from "storybook/test";

import type { StorageCleanupCandidate } from "@/hooks/useMutations";
import { stubFetch } from "@/components/_storyStubs";

import { StorageCleanupSection } from "./StorageCleanupSection";

/** The section starts collapsed and idle; each story opens it and runs a scan. */
const meta: Meta<typeof StorageCleanupSection> = {
  title: "App/Settings/StorageCleanupSection",
  component: StorageCleanupSection,
  parameters: { layout: "padded" },
  beforeEach: () => stubFetch(fakeApi),
};
export default meta;
type Story = StoryObj<typeof StorageCleanupSection>;

const CANDIDATES: StorageCleanupCandidate[] = [
  {
    prefix: "blobs",
    key: "a1/a1b2c3d4e5f6",
    size: 48_200_000,
    last_modified: "2026-09-12T10:00:00Z",
    filename: "Castle.prefab",
    library_id: 7,
    asset_id: 214,
    version_id: 3,
    kind: "file",
  },
  {
    prefix: "originals",
    key: "d4/d4e5f6a7b8c9",
    size: 684_221_440,
    last_modified: "2026-08-02T10:00:00Z",
  },
];

// Each story picks what the scan returns in its beforeEach.
let scenario: "found" | "clean" | "scanning" | "error" = "found";

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const fakeApi =
  (realFetch: typeof window.fetch) =>
  async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    const path = raw.startsWith("http") ? new URL(raw).pathname : raw.split("?")[0]!;
    if (path !== "/api/admin/storage/cleanup/scan") return realFetch(input, init);
    if (scenario === "scanning") return new Promise<Response>(() => {});
    if (scenario === "error") return json({ error: "boom" }, 500);
    return json({ candidates: scenario === "clean" ? [] : CANDIDATES });
  };

function Harness() {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <div className="w-[640px]">
        <StorageCleanupSection />
      </div>
    </QueryClientProvider>
  );
}

async function openAndScan(canvasElement: HTMLElement) {
  const canvas = within(canvasElement);
  await userEvent.click(canvas.getByRole("button", { name: "Storage cleanup" }));
  await userEvent.click(await canvas.findByRole("button", { name: "Scan for orphans" }));
  return canvas;
}

/** Orphans found and all selected, with object metadata where the store has it. */
export const Found: Story = {
  beforeEach: () => {
    scenario = "found";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = await openAndScan(canvasElement);
    await expect(await canvas.findByText("a1/a1b2c3d4e5f6")).toBeVisible();
    await expect(canvas.getByRole("button", { name: "Delete selected" })).toBeEnabled();
  },
};

export const Clean: Story = {
  beforeEach: () => {
    scenario = "clean";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = await openAndScan(canvasElement);
    await expect(await canvas.findByText("No orphaned objects")).toBeVisible();
  },
};

export const Scanning: Story = {
  beforeEach: () => {
    scenario = "scanning";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = await openAndScan(canvasElement);
    await expect(await canvas.findByRole("button", { name: "Scanning…" })).toBeDisabled();
  },
};

export const Error: Story = {
  beforeEach: () => {
    scenario = "error";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = await openAndScan(canvasElement);
    await expect(await canvas.findByText("Scan failed. Try again.")).toBeVisible();
  },
};
