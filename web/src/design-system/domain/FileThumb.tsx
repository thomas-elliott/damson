import { type CSSProperties } from "react";

import { cn } from "@/design-system/utils";

import { AssetThumb } from "./AssetThumb";
import { FileGlyph } from "./FileGlyph";
import { isImageType } from "./fileTypes";
import type { FileNode } from "./fileTree";

export interface FileThumbProps {
  node: FileNode;
  /** Larger glyph for the detail-panel hero. */
  large?: boolean;
  /**
   * A resolved thumbnail image URL (e.g. an object URL the caller fetched for a
   * derivative or native image). Rendered directly when set, taking priority
   * over `imageSrc` — the caller owns the decision of what's renderable.
   */
  imageUrl?: string;
  /**
   * Resolve a file's object key to a previewable image URL. When provided and
   * the node is an image type, the thumbnail renders the real file; otherwise
   * a type placeholder is shown. Defaults to none (always placeholder).
   */
  imageSrc?: (node: FileNode) => string | undefined;
  className?: string;
  style?: CSSProperties;
}

/**
 * File preview surface. Image-type files render inline from the file-serving
 * endpoint (via `imageSrc`); fixtures may instead carry a procedural `thumb`
 * descriptor. Everything else falls back to a type-styled placeholder.
 */
export function FileThumb({ node, large, imageUrl, imageSrc, className, style }: FileThumbProps) {
  const src = imageUrl ?? (isImageType(node.ftype) ? imageSrc?.(node) : undefined);
  if (src) {
    return (
      <img
        src={src}
        alt={node.name}
        loading="lazy"
        className={cn("block h-full w-full object-cover", className)}
        style={style}
      />
    );
  }
  if (node.thumb) {
    return <AssetThumb thumb={node.thumb} className={className} style={style} />;
  }
  return <FilePlaceholder node={node} large={large} className={className} style={style} />;
}

export interface FilePlaceholderProps {
  node: FileNode;
  large?: boolean;
  className?: string;
  style?: CSSProperties;
}

/** Striped placeholder for non-previewable files: type glyph + extension. */
export function FilePlaceholder({ node, large, className, style }: FilePlaceholderProps) {
  return (
    <div
      aria-hidden
      className={cn(
        "grid h-full w-full place-items-center gap-1.5 bg-muted text-subtle-foreground",
        className,
      )}
      style={style}
    >
      <FileGlyph ftype={node.ftype} size={large ? 30 : 22} strokeWidth={1.4} />
      <span className="font-mono text-[10px] uppercase tracking-wide text-faint-foreground">
        {node.ext.replace(".", "") || "file"}
      </span>
    </div>
  );
}
