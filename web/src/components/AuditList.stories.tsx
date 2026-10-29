import type { Meta, StoryObj } from "@storybook/react";
import { MemoryRouter } from "react-router";

import type { AuditEntry } from "@/hooks/useDataQueries";

import { AuditList } from "./AuditList";

const meta: Meta<typeof AuditList> = {
  title: "App/AuditList",
  component: AuditList,
  parameters: { layout: "padded" },
  decorators: [
    (Story) => (
      <MemoryRouter>
        <div className="max-w-content-wide">
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
};
export default meta;
type Story = StoryObj<typeof AuditList>;

const entry = (e: Partial<AuditEntry> & Pick<AuditEntry, "id" | "action_type">): AuditEntry => ({
  target_type: "asset",
  user_display_name: "Librarian",
  user_email: "librarian@example.com",
  created_at: "2026-10-29T13:10:00Z",
  ...e,
});

export const Named: Story = {
  args: {
    linkTargets: true,
    entries: [
      entry({
        id: 1,
        action_type: "project.create",
        target_type: "project",
        target_id: 1,
        target_name: "Ashfall",
      }),
      entry({
        id: 2,
        action_type: "asset.licence.update",
        target_id: 214,
        target_name: "POLYGON - Fantasy Kingdom",
      }),
      entry({
        id: 3,
        action_type: "collection.add",
        target_type: "collection",
        target_id: 4,
        target_name: "Props",
      }),
      // The asset is gone, so the id stands in for its name.
      entry({ id: 4, action_type: "asset.delete", target_id: 297 }),
    ],
  },
};

export const Empty: Story = {
  args: { entries: [] },
};
