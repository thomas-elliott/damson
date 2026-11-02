import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { expect, within } from "storybook/test";

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

/** Admins edit the name and description; a team's one library can't be deleted. */
export const Admin: Story = {
  render: () => <Harness isAdmin />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole("button", { name: "Save" })).toBeVisible();
    await expect(canvas.queryByRole("button", { name: /delete/i })).not.toBeInTheDocument();
  },
};

/** Members see the same fields, disabled. */
export const ReadOnly: Story = {
  render: () => <Harness isAdmin={false} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByDisplayValue("Main")).toBeDisabled();
  },
};
