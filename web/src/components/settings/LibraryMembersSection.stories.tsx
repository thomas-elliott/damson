import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, userEvent, within } from "storybook/test";

import { TooltipProvider } from "@/design-system";
import type { MemberView } from "@/hooks/useDataQueries";
import { contrastDebt } from "@/design-system/_a11yDebt";
import { stubFetch } from "@/components/_storyStubs";

import { LibraryMembersSection } from "./LibraryMembersSection";

const meta: Meta<typeof LibraryMembersSection> = {
  title: "App/Settings/LibraryMembersSection",
  component: LibraryMembersSection,
  parameters: { layout: "padded" },
  beforeEach: () => stubFetch(fakeApi),
};
export default meta;
type Story = StoryObj<typeof LibraryMembersSection>;

const MEMBERS: MemberView[] = [
  {
    user_id: 1,
    email: "thomas@example.com",
    display_name: "Thomas",
    role: "admin",
    global_role: "sysadmin",
  },
  {
    user_id: 2,
    email: "librarian@example.com",
    display_name: "Librarian",
    role: "member",
    global_role: "user",
  },
  {
    user_id: 3,
    email: "artist@example.com",
    display_name: "Artist",
    role: "member",
    global_role: "user",
  },
];

// Library ids pick the fixture, so stories don't depend on render order.
const LIBRARY = 7;
const EMPTY_LIBRARY = 8;
const LOADING_LIBRARY = 9;
const FAILED_LIBRARY = 10;

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
    const members = /^\/api\/libraries\/(\d+)\/members$/.exec(path);
    if (members) {
      const id = Number(members[1]);
      if (id === LOADING_LIBRARY) return new Promise<Response>(() => {});
      if (id === FAILED_LIBRARY) return json({ error: "internal error" }, 500);
      return json({ members: id === LIBRARY ? MEMBERS : [] });
    }
    return realFetch(input, init);
  };

function Harness({ libraryId, isAdmin }: { libraryId: number; isAdmin: boolean }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <TooltipProvider>
        <div className="w-[640px]">
          <LibraryMembersSection libraryId={libraryId} isAdmin={isAdmin} />
        </div>
      </TooltipProvider>
    </QueryClientProvider>
  );
}

/** An admin's view: role selects, remove buttons, and Invite. The last admin's role is locked. */
export const Admin: Story = {
  render: () => <Harness libraryId={LIBRARY} isAdmin />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Librarian")).toBeVisible();
    await expect(canvas.getByRole("button", { name: "Invite" })).toBeVisible();
  },
};

/** A member sees the list without any controls. */
export const ReadOnly: Story = {
  render: () => <Harness libraryId={LIBRARY} isAdmin={false} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Librarian")).toBeVisible();
    await expect(canvas.queryByRole("button", { name: "Invite" })).not.toBeInTheDocument();
  },
};

export const Empty: Story = {
  render: () => <Harness libraryId={EMPTY_LIBRARY} isAdmin />,
  play: async ({ canvasElement }) => {
    await expect(await within(canvasElement).findByText("No members yet")).toBeVisible();
  },
};

/** Still loading: the list says so rather than "No members yet", and the heading leaves out a count. */
export const Loading: Story = {
  render: () => <Harness libraryId={LOADING_LIBRARY} isAdmin />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Loading…")).toBeVisible();
    await expect(canvas.getByRole("button", { name: "Members" })).toBeVisible();
    await expect(canvas.queryByText("No members yet")).not.toBeInTheDocument();
  },
};

/** The members failed to load: a Retry instead of "No members yet". */
export const Failed: Story = {
  render: () => <Harness libraryId={FAILED_LIBRARY} isAdmin />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Couldn't load members")).toBeVisible();
    await expect(canvas.getByRole("button", { name: "Retry" })).toBeVisible();
    await expect(canvas.queryByText("No members yet")).not.toBeInTheDocument();
  },
};

/** Local mode: inviting an unknown email can set the new user's password. */
export const Inviting: Story = {
  parameters: {
    // Known contrast debt: FieldRow hints, faint-foreground on card, 4.34:1 (SMO-3 finding 5).
    a11y: contrastDebt("faint-foreground"),
  },
  render: () => <Harness libraryId={LIBRARY} isAdmin />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Invite" }));
    const dialog = await within(canvasElement.ownerDocument.body).findByRole("dialog");
    await expect(dialog).toBeVisible();
  },
};
