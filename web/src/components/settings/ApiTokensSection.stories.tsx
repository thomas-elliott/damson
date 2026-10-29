import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, userEvent, within } from "storybook/test";

import type { ApiToken } from "@/hooks/useDataQueries";
import { contrastDebt } from "@/design-system/_a11yDebt";
import { stubFetch } from "@/components/_storyStubs";

import { ApiTokensSection } from "./ApiTokensSection";

const meta: Meta<typeof ApiTokensSection> = {
  title: "App/Settings/ApiTokensSection",
  component: ApiTokensSection,
  parameters: { layout: "padded" },
  beforeEach: () => stubFetch(fakeApi),
};
export default meta;
type Story = StoryObj<typeof ApiTokensSection>;

const TOKENS: ApiToken[] = [
  {
    id: 1,
    label: "laptop sync",
    prefix: "dmsn_4f2a",
    scope: "write",
    library_id: 7,
    library_name: "Main",
    created_at: "2026-09-01T10:00:00Z",
    last_used_at: "2026-10-28T08:30:00Z",
  },
  {
    id: 2,
    label: "CI importer",
    prefix: "dmsn_9b11",
    scope: "read",
    library_id: 7,
    library_name: "Main",
    created_at: "2026-08-14T10:00:00Z",
  },
  {
    id: 3,
    label: "old build box",
    prefix: "dmsn_0c7e",
    scope: "write",
    library_id: 7,
    library_name: "Main",
    created_at: "2026-02-02T10:00:00Z",
    revoked_at: "2026-06-01T10:00:00Z",
  },
];

// Each story picks what GET /api/me/tokens returns in its beforeEach.
let scenario: "populated" | "empty" | "loading" | "error" = "populated";

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
    if (path !== "/api/me/tokens") return realFetch(input, init);
    if (scenario === "loading") return new Promise<Response>(() => {});
    if (scenario === "error") return json({ error: "boom" }, 500);
    return json({ tokens: scenario === "empty" ? [] : TOKENS });
  };

function Harness() {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <div className="w-[640px]">
        <ApiTokensSection />
      </div>
    </QueryClientProvider>
  );
}

/** Active read and write tokens, plus a revoked one kept for the record. */
export const Populated: Story = {
  beforeEach: () => {
    scenario = "populated";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("laptop sync")).toBeVisible();
    await expect(canvas.getByText("Revoked")).toBeVisible();
  },
};

export const Empty: Story = {
  beforeEach: () => {
    scenario = "empty";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    await expect(await within(canvasElement).findByText("No access tokens")).toBeVisible();
  },
};

export const Loading: Story = {
  beforeEach: () => {
    scenario = "loading";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    await expect(await within(canvasElement).findByRole("status")).toBeVisible();
  },
};

export const Error: Story = {
  beforeEach: () => {
    scenario = "error";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    await expect(await within(canvasElement).findByText("Couldn't load tokens")).toBeVisible();
  },
};

/** The create dialog, as New token opens it. */
export const Creating: Story = {
  parameters: {
    // Known contrast debt: FieldRow hints, faint-foreground on card, 4.34:1 (SMO-3 finding 5).
    a11y: contrastDebt("faint-foreground"),
  },
  beforeEach: () => {
    scenario = "populated";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "New token" }));
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByRole("dialog", { name: "New access token" })).toBeVisible();
  },
};
