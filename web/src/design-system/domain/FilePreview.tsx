import { type CSSProperties } from "react";

import { cn } from "@/design-system/utils";

import { FileThumb } from "./FileThumb";
import { isImageType } from "./fileTypes";
import type { FileNode } from "./fileTree";

export interface FilePreviewProps {
  node: FileNode;
  /**
   * Object URL for the file's bytes, resolved by the caller from an
   * authenticated blob fetch (see `useObjectUrl`). Consumed only by types that
   * preview from the raw file (currently images); undefined while loading or
   * for types with no inline viewer yet.
   */
  previewUrl?: string;
  /** Larger glyphs/labels for the detail-panel hero. */
  large?: boolean;
  className?: string;
  style?: CSSProperties;
}

/**
 * Per-type preview dispatcher for the file detail panel. Picks the richest
 * viewer a file type supports and falls back to {@link FileThumb} (a procedural
 * thumbnail or a type placeholder) when none applies or the bytes aren't
 * available yet.
 *
 * Stage 1 ships only the inline-image viewer. Later stages slot their viewers
 * in above the fallback, lazy-loaded so heavy deps stay out of the main bundle:
 *
 * ```ts
 * const CodePreview = lazy(() => import("./previews/CodePreview"));
 * if (node.ftype === "script") {
 *   return <Suspense fallback={…}><CodePreview node={node} /></Suspense>;
 * }
 * ```
 */
export function FilePreview({ node, previewUrl, large, className, style }: FilePreviewProps) {
  if (isImageType(node.ftype) && previewUrl) {
    return (
      <img
        src={previewUrl}
        alt={node.name}
        loading="lazy"
        className={cn("block h-full w-full object-cover", className)}
        style={style}
      />
    );
  }

  // No inline viewer (or bytes still loading): procedural thumb / placeholder.
  return <FileThumb node={node} large={large} className={className} style={style} />;
}
