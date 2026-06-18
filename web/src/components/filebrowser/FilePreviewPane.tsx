import { type ReactNode, Suspense, lazy } from "react";

import { FilePreview, type FileNode } from "@/design-system";

import { ImagePreview } from "./previews/ImagePreview";
import { previewKind } from "./previews/previewKind";

// Heavy viewers — each fetched only when its file type is opened.
const TextPreview = lazy(() => import("./previews/TextPreview"));
const AudioPreview = lazy(() => import("./previews/AudioPreview"));
const FontPreview = lazy(() => import("./previews/FontPreview"));
const ZipPreview = lazy(() => import("./previews/ZipPreview"));

function PaneLoading() {
  return (
    <div className="grid h-full w-full place-items-center text-ui text-text-3">
      Loading preview…
    </div>
  );
}

/**
 * App-layer preview dispatcher for the detail panel: picks the richest viewer a
 * file supports and renders it, each viewer fetching its own bytes. Falls back
 * to the presentational {@link FilePreview} (thumbnail/placeholder) for types
 * with no inline viewer yet. New viewers slot into the switch as later stages
 * add them.
 */
export function FilePreviewPane({ node }: { node: FileNode }) {
  switch (previewKind(node)) {
    case "text":
      return wrap(<TextPreview node={node} />);
    case "audio":
      return wrap(<AudioPreview node={node} />);
    case "font":
      return wrap(<FontPreview node={node} />);
    case "archive":
      return wrap(<ZipPreview node={node} />);
    case "image":
      return <ImagePreview node={node} />;
    default:
      return <FilePreview node={node} large />;
  }
}

function wrap(viewer: ReactNode) {
  return <Suspense fallback={<PaneLoading />}>{viewer}</Suspense>;
}
