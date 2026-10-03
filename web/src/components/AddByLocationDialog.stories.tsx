import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, userEvent, within } from "storybook/test";

import { AddByLocationDialog } from "./AddByLocationDialog";

const meta: Meta<typeof AddByLocationDialog> = {
  title: "App/AddByLocationDialog",
  component: AddByLocationDialog,
  parameters: { layout: "fullscreen" },
};
export default meta;
type Story = StoryObj<typeof AddByLocationDialog>;

// 2x2 PNG for the proxied store cover.
const PNG_BYTES = Uint8Array.from(
  atob(
    "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAACp8Z5+AAAAFUlEQVR42mNkYPhfz0DFwEjNwAgAaZgD+8sVYVoAAAAASUVORK5CYII=",
  ),
  (c) => c.charCodeAt(0),
);

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Serves the two endpoints the dialog touches: the store lookup and the record.
const realFetch = window.fetch;
window.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
  const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
  const path = raw.startsWith("http") ? new URL(raw).pathname : raw.split("?")[0]!;
  if (path === "/api/metadata/image") {
    return new Response(PNG_BYTES, { status: 200, headers: { "Content-Type": "image/png" } });
  }
  if (path === "/api/metadata/preview") {
    return json({
      title: "POLYGON Fantasy Kingdom",
      publisher: "Synty Studios",
      cover_url: "https://cdn.example.com/cover.png",
    });
  }
  if (path === "/api/assets/references" && init?.method === "POST") {
    return json({ outcome: "created", asset_id: 41, name: "POLYGON Fantasy Kingdom" }, 201);
  }
  return realFetch(input, init);
};

function Harness() {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <AddByLocationDialog onOpenChange={() => {}} onOpenAsset={() => {}} />
    </QueryClientProvider>
  );
}

/** Empty, as it opens from the library toolbar. */
export const Empty: Story = { render: () => <Harness /> };

/** A store link is looked up and its match shown before anything is added. */
export const StoreMatch: Story = {
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.type(
      await body.findByLabelText("Store link"),
      "https://assetstore.unity.com/packages/k-1",
    );
    await expect(await body.findByText("Synty Studios")).toBeVisible();
    await userEvent.click(body.getByRole("button", { name: "Add" }));
    await expect(await body.findByText("Added to your library")).toBeVisible();
  },
};
