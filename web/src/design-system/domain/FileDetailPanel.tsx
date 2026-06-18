import { type ReactNode } from "react";
import { ChevronLeft, ChevronRight, Download, FolderPlus, RefreshCw, X } from "lucide-react";

import { cn } from "@/design-system/utils";
import { formatBytes } from "@/design-system/utils/format";
import { Button } from "@/design-system/primitives/Button";
import { IconButton } from "@/design-system/primitives/IconButton";
import { Section } from "@/design-system/compositions/Section";
import { KeyValueList } from "./KeyValueList";

import { FilePreview } from "./FilePreview";
import { FileGlyph } from "./FileGlyph";
import { fileTypeMeta } from "./fileTypes";
import type { FileNode } from "./fileTree";

export interface FileDetailPanelProps {
  /** Asset the file belongs to — used in the full path bar. */
  assetName: string;
  node: FileNode;
  /** Folder path of the file within the asset. */
  dir: string[];
  /** 0-based position in the current listing and total, for the stepper. */
  index: number;
  total: number;
  onPrev?: () => void;
  onNext?: () => void;
  onClose?: () => void;
  onDownload?: () => void;
  onAddToSubset?: () => void;
  /** Current backend preview status, shown as a detail row when present. */
  previewStatus?: string;
  /** When provided, renders a "Regenerate preview" action for this file. */
  onRequeuePreview?: () => void;
  /** Disables the regenerate action while a requeue request is in flight. */
  requeuePending?: boolean;
  /** Object URL for the open file's bytes, for the default inline-image hero. */
  previewUrl?: string;
  /**
   * Hero preview content. When provided (the app passes a per-type preview
   * dispatcher), it replaces the default image/placeholder hero entirely.
   */
  preview?: ReactNode;
  /** Extra detail rows (referenced-by, dimensions, …). */
  extra?: ReactNode;
}

/**
 * File detail panel — reuses the asset detail-panel chrome (sticky header with
 * a stepper + close, scrollable hero/body). Rendered inside a DetailPanel or
 * a FullPageView side slot by the caller.
 */
export function FileDetailPanel({
  assetName,
  node,
  dir,
  index,
  total,
  onPrev,
  onNext,
  onClose,
  onDownload,
  onAddToSubset,
  previewStatus,
  onRequeuePreview,
  requeuePending,
  previewUrl,
  preview,
  extra,
}: FileDetailPanelProps) {
  const meta = fileTypeMeta(node.ftype);
  return (
    <aside
      aria-label="File details"
      className="flex h-full flex-col border-l border-line bg-panel text-text"
    >
      <header className="flex shrink-0 items-center gap-1 border-b border-line px-3.5 py-2.5">
        <IconButton aria-label="Previous file" onClick={onPrev} title="Previous (↑)">
          <ChevronLeft size={14} />
        </IconButton>
        <IconButton aria-label="Next file" onClick={onNext} title="Next (↓)">
          <ChevronRight size={14} />
        </IconButton>
        <span className="text-mono tabular-nums text-text-3">
          {index + 1} / {total}
        </span>
        <div className="flex-1" />
        <IconButton aria-label="Close details" onClick={onClose} title="Close (Esc)">
          <X size={14} />
        </IconButton>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-3.5 py-3">
        <div className="mb-3 aspect-square w-full overflow-hidden rounded-md border border-line bg-bg">
          {preview ?? <FilePreview node={node} large previewUrl={previewUrl} />}
        </div>
        <h2 className="break-all text-subhead text-text">{node.name}</h2>
        <div className="mt-1 flex flex-wrap items-center gap-1.5 text-caption">
          <FileGlyph ftype={node.ftype} size={11} strokeWidth={1.5} />
          <span>{meta.label}</span>
          <span aria-hidden>·</span>
          <span>{formatBytes(node.size)}</span>
          {node.modified && (
            <>
              <span aria-hidden>·</span>
              <span>modified {node.modified}</span>
            </>
          )}
        </div>
        <div className="mt-2 break-all rounded-sm bg-bg px-2 py-1.5 font-mono text-small text-text-3">
          {[assetName, ...dir, node.name].join(" / ")}
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" onClick={onDownload}>
            <Download size={12} /> Download
          </Button>
          {onAddToSubset && (
            <Button variant="secondary" size="sm" onClick={onAddToSubset}>
              <FolderPlus size={12} /> Add to subset
            </Button>
          )}
          {onRequeuePreview && (
            <Button
              variant="secondary"
              size="sm"
              onClick={onRequeuePreview}
              disabled={requeuePending}
            >
              <RefreshCw size={12} /> Regenerate preview
            </Button>
          )}
        </div>

        <div className="mt-4">
          <Section title="Details">
            <KeyValueList
              entries={[
                { key: "Path", value: [...dir, node.name].join("/"), mono: true },
                {
                  key: "Type",
                  value: (
                    <span>
                      {meta.label}{" "}
                      <span className="font-mono text-text-4">({node.ext || "—"})</span>
                    </span>
                  ),
                },
                { key: "Size", value: formatBytes(node.size), mono: true },
                ...(node.modified ? [{ key: "Modified", value: node.modified, mono: true }] : []),
                ...(previewStatus ? [{ key: "Preview", value: previewStatus, mono: true }] : []),
              ]}
            />
          </Section>
          {extra}
        </div>
      </div>
    </aside>
  );
}

/** Header counter helper — exported for callers that build their own chrome. */
export function FileDetailCounter({
  index,
  total,
  className,
}: {
  index: number;
  total: number;
  className?: string;
}) {
  return (
    <span className={cn("text-mono tabular-nums text-text-3", className)}>
      {index + 1} / {total}
    </span>
  );
}
