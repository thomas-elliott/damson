import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, within } from "storybook/test";

import { TooltipProvider } from "@/design-system";
import type { AdminUser } from "@/hooks/useDataQueries";
import { stubFetch } from "@/components/_storyStubs";

import { ServerAdminSection } from "./ServerAdminSection";

/** The sysadmin block of Settings: server settings, users, and storage cleanup. */
const meta: Meta<typeof ServerAdminSection> = {
  title: "App/Settings/ServerAdminSection",
  component: ServerAdminSection,
  parameters: { layout: "padded" },
  beforeEach: () => stubFetch(fakeApi),
};
export default meta;
type Story = StoryObj<typeof ServerAdminSection>;

const USERS: AdminUser[] = [
  { id: 1, email: "thomas@example.com", display_name: "Thomas", role: "sysadmin" },
  { id: 2, email: "librarian@example.com", display_name: "Librarian", role: "user" },
  { id: 3, email: "artist@example.com", display_name: "Artist", role: "user" },
];

// Each story picks what the admin endpoints return in its beforeEach.
let scenario: "populated" | "empty" | "loading" | "failed" = "populated";

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
    if (path === "/api/auth/mode") return json({ mode: "local", needs_bootstrap: false });
    if (path !== "/api/admin/settings" && path !== "/api/admin/users") {
      return realFetch(input, init);
    }
    if (scenario === "loading") return new Promise<Response>(() => {});
    if (scenario === "failed") return json({ error: "internal error" }, 500);
    if (path === "/api/admin/settings") {
      return json({ max_upload_bytes: 68_719_476_736, licence_kb_auto_update: true });
    }
    return json({ users: scenario === "empty" ? [] : USERS });
  };

function Harness() {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <TooltipProvider>
        <div className="w-[640px]">
          <ServerAdminSection />
        </div>
      </TooltipProvider>
    </QueryClientProvider>
  );
}

/** Local auth, so each user gets Reset password. The only sysadmin's role is locked. */
export const Populated: Story = {
  beforeEach: () => {
    scenario = "populated";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Librarian")).toBeVisible();
    await expect(canvas.getByDisplayValue("68719476736")).toBeVisible();
  },
};

export const Empty: Story = {
  beforeEach: () => {
    scenario = "empty";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    await expect(await within(canvasElement).findByText("No users yet")).toBeVisible();
  },
};

export const Loading: Story = {
  beforeEach: () => {
    scenario = "loading";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const statuses = await within(canvasElement).findAllByRole("status");
    await expect(statuses).toHaveLength(2);
  },
};

/** Both admin endpoints failed: each section offers a Retry, not a spinner or "No users yet". */
export const Failed: Story = {
  beforeEach: () => {
    scenario = "failed";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Couldn't load server settings")).toBeVisible();
    await expect(await canvas.findByText("Couldn't load users")).toBeVisible();
    await expect(canvas.getAllByRole("button", { name: "Retry" })).toHaveLength(2);
    await expect(canvas.queryByText("Loading…")).not.toBeInTheDocument();
    await expect(canvas.queryByText("No users yet")).not.toBeInTheDocument();
  },
};
