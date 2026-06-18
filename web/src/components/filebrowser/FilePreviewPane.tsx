import { Suspense, lazy } from "react";

import { FilePreview, type FileNode } from "@/design-system";

import { ImagePreview } from "./previews/ImagePreview";
import { previewKind } from "./previews/previewKind";

// Heavy (Shiki) — only fetched when a text file is opened.
const TextPreview = lazy(() => import("./previews/TextPreview"));

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
      return (
        <Suspense fallback={<PaneLoading />}>
          <TextPreview node={node} />
        </Suspense>
      );
    case "image":
      return <ImagePreview node={node} />;
    default:
      return <FilePreview node={node} large />;
  }
}
