import type { Meta, StoryObj } from "@storybook/react";
import { MemoryRouter } from "react-router";

import { UrlPager } from "./UrlPager";

// The pager as every paged screen mounts it: the last child of the page's
// flex column, under the scrolling content.
const meta: Meta<typeof UrlPager> = {
  title: "App/UrlPager",
  component: UrlPager,
  parameters: { layout: "fullscreen" },
  args: {
    "aria-label": "Library pages",
    total: 297,
    query: { loading: false, placeholder: false, error: null },
  },
  decorators: [
    (Story, { parameters }) => (
      <MemoryRouter initialEntries={[(parameters.url as string | undefined) ?? "/library"]}>
        <div className="flex h-[320px] w-full flex-col bg-background">
          <div className="min-h-0 flex-1 p-3.5 text-ui text-subtle-foreground">Content</div>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
};
export default meta;
type Story = StoryObj<typeof UrlPager>;

export const FirstPage: Story = {};

export const MiddlePage: Story = {
  parameters: { url: "/library?page=3" },
};

export const PageSize: Story = {
  parameters: { url: "/library?page=2&page_size=25" },
};
