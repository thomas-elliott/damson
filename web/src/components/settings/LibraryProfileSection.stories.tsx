import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { expect, userEvent, within } from "storybook/test";

import type { MeLibrary } from "@/hooks/useDataQueries";

import { LibraryProfileSection } from "./LibraryProfileSection";

/** Props only: the section reads no queries, and its writes aren't exercised here. */
const meta: Meta<typeof LibraryProfileSection> = {
  title: "App/Settings/LibraryProfileSection",
  component: LibraryProfileSection,
  parameters: { layout: "padded" },
};
export default meta;
type Story = StoryObj<typeof LibraryProfileSection>;

const LIBRARY: MeLibrary = {
  id: 7,
  name: "Main",
  description: "Shared kit for the Ashfall team.",
  role: "admin",
};

function Harness({ isAdmin }: { isAdmin: boolean }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <div className="w-[640px]">
          <LibraryProfileSection library={LIBRARY} isAdmin={isAdmin} />
        </div>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

/** Admins edit the name and description, and get the danger zone. */
export const Admin: Story = {
  render: () => <Harness isAdmin />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole("button", { name: "Save" })).toBeVisible();
    await expect(canvas.getByText("Danger zone")).toBeVisible();
  },
};

/** Members see the same fields, disabled, and no danger zone. */
export const ReadOnly: Story = {
  render: () => <Harness isAdmin={false} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByDisplayValue("Main")).toBeDisabled();
    await expect(canvas.queryByText("Danger zone")).not.toBeInTheDocument();
  },
};

/** Deleting asks for the library name before the button arms. */
export const ConfirmingDelete: Story = {
  parameters: {
    // Contrast gate: FieldRow hints are faint-foreground on card, 4.34:1 (audit finding 5),
    // and the Delete library button is destructive on destructive-soft, 4.45:1; both under
    // 4.5:1 and both palette debt. Drop this when they are fixed.
    a11y: { test: "todo" },
  },
  render: () => <Harness isAdmin />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Danger zone" }));
    await userEvent.click(await canvas.findByRole("button", { name: "Delete library" }));
    const dialog = await within(canvasElement.ownerDocument.body).findByRole("dialog", {
      name: "Delete library?",
    });
    await expect(within(dialog).getByRole("button", { name: "Delete library" })).toBeDisabled();
  },
};
