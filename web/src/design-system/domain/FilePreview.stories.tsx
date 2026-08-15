import { type ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import { FilePreview } from "./FilePreview";
import type { FileNode } from "./fileTree";

const meta: Meta<typeof FilePreview> = {
  title: "Domain/FilePreview",
  component: FilePreview,
};
export default meta;
type Story = StoryObj<typeof FilePreview>;

function node(name: string, ftype: FileNode["ftype"]): FileNode {
  const ext = (name.match(/\.[a-z0-9]+$/i)?.[0] ?? "").toLowerCase();
  return { type: "file", name, ext, ftype, size: 4096 };
}

// A tiny inline SVG-as-data-URL stands in for a resolved blob: object URL so the
// image branch renders without a network fetch.
const SAMPLE_IMAGE =
  "data:image/svg+xml;utf8," +
  encodeURIComponent(
    `<svg xmlns='http://www.w3.org/2000/svg' width='160' height='160'>
       <rect width='160' height='160' fill='#caa84a'/>
       <circle cx='80' cy='80' r='44' fill='#2a2620'/>
     </svg>`,
  );

const frame = (children: ReactNode) => (
  <div className="h-[220px] w-[220px] overflow-hidden rounded-md border border-border bg-background">
    {children}
  </div>
);

/** Image type with a resolved URL → renders the bytes inline. */
export const Image: Story = {
  render: () =>
    frame(
      <FilePreview node={node("Brick_Albedo.png", "texture")} previewUrl={SAMPLE_IMAGE} large />,
    ),
};

/** Image type whose bytes haven't resolved yet → placeholder fallback. */
export const ImageLoading: Story = {
  render: () => frame(<FilePreview node={node("Brick_Albedo.png", "texture")} large />),
};

/** Non-image type → type placeholder, regardless of any URL. */
export const NonImageFallback: Story = {
  render: () => frame(<FilePreview node={node("PlayerController.cs", "script")} large />),
};
