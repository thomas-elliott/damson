import { Suspense, lazy } from "react";

import { cn } from "@/design-system/utils";

const MarkdownEditorView = lazy(() => import("./MarkdownEditorView"));

export interface MarkdownEditorProps {
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  /** Minimum body height in px. Grows with the content past this. */
  minHeight?: number;
  /** Cap the height in px; the editor scrolls internally beyond it. */
  maxHeight?: number;
  autoFocus?: boolean;
  className?: string;
  /** Wired by FieldRow; forwarded onto CodeMirror's editable element. */
  id?: string;
  /**
   * Required for accessibility. CodeMirror's surface is a `contenteditable`
   * with `role="textbox"`, which a FieldRow `<label for>` cannot name — so pass
   * the field's label here as well.
   */
  "aria-label": string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
}

/**
 * Markdown source editor — a CodeMirror 6 surface styled to match
 * {@link Textarea}, with source highlighting, wrapping, and undo history.
 * Pair with {@link Markdown} for the rendered read view.
 *
 * CodeMirror is code-split behind `lazy`, so views that only read markdown
 * don't load it.
 */
export function MarkdownEditor(props: MarkdownEditorProps) {
  return (
    <Suspense
      fallback={
        <div
          className={cn(
            "w-full animate-pulse rounded-sm border border-line-2 bg-bg",
            props.className,
          )}
          style={{ height: props.minHeight ?? 120 }}
        />
      }
    >
      <MarkdownEditorView {...props} />
    </Suspense>
  );
}
