import { useEffect, useRef } from "react";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import { EditorView, keymap, placeholder as cmPlaceholder } from "@codemirror/view";
import { tags } from "@lezer/highlight";

import { cn } from "@/design-system/utils";

import type { MarkdownEditorProps } from "./MarkdownEditor";

// Markdown source highlighting mapped onto the design tokens rather than a
// stock CodeMirror theme, so the editor reads as one surface with Input and
// Textarea beside it.
const highlight = HighlightStyle.define([
  { tag: tags.heading, color: "var(--ds-text)", fontWeight: "600" },
  { tag: tags.strong, color: "var(--ds-text)", fontWeight: "600" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.strikethrough, textDecoration: "line-through", color: "var(--ds-text-3)" },
  { tag: tags.link, color: "var(--ds-accent)" },
  { tag: tags.url, color: "var(--ds-accent)" },
  { tag: tags.monospace, fontFamily: "var(--ds-font-mono)", color: "var(--ds-text-2)" },
  { tag: tags.quote, color: "var(--ds-text-2)", fontStyle: "italic" },
  { tag: [tags.processingInstruction, tags.meta, tags.atom], color: "var(--ds-text-4)" },
]);

const theme = EditorView.theme({
  "&": {
    fontSize: "14px",
    color: "var(--ds-text)",
    backgroundColor: "var(--ds-bg)",
    borderRadius: "var(--ds-r-sm)",
    border: "1px solid var(--ds-line-2)",
    transition: "border-color 100ms cubic-bezier(0.4, 0, 0.2, 1)",
  },
  "&.cm-focused": {
    outline: "none",
    borderColor: "var(--ds-accent)",
    boxShadow: "inset 0 0 0 2px var(--ds-accent-ring)",
  },
  ".cm-scroller": { fontFamily: "var(--ds-font-sans)", lineHeight: "1.5" },
  ".cm-content": { padding: "8px", caretColor: "var(--ds-text)" },
  ".cm-line": { padding: "0" },
  ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--ds-text)" },
  ".cm-placeholder": { color: "var(--ds-text-4)" },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": {
    backgroundColor: "var(--ds-accent-tint)",
  },
});

/**
 * CodeMirror 6 host behind {@link MarkdownEditor}'s lazy boundary. Everything
 * here is loaded on demand, so opening the panel in read mode never pays for it.
 */
export default function MarkdownEditorView({
  value,
  onChange,
  placeholder,
  minHeight = 120,
  maxHeight,
  autoFocus,
  className,
  id,
  "aria-label": ariaLabel,
  "aria-describedby": describedBy,
  "aria-invalid": invalid,
}: MarkdownEditorProps) {
  const host = useRef<HTMLDivElement | null>(null);
  const viewRef = useRef<EditorView | null>(null);
  // The change handler is read through a ref so a new closure per render
  // doesn't force the editor to be torn down and rebuilt.
  const onChangeRef = useRef(onChange);
  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useEffect(() => {
    const parent = host.current;
    if (!parent) return;
    const view = new EditorView({
      parent,
      state: EditorState.create({
        doc: value,
        extensions: [
          history(),
          keymap.of([...defaultKeymap, ...historyKeymap]),
          markdown({ base: markdownLanguage }),
          syntaxHighlighting(highlight),
          EditorView.lineWrapping,
          theme,
          EditorView.theme({
            ".cm-content": { minHeight: `${minHeight}px` },
            ...(maxHeight
              ? { ".cm-scroller": { maxHeight: `${maxHeight}px`, overflowY: "auto" } }
              : {}),
          }),
          cmPlaceholder(placeholder ?? ""),
          EditorView.contentAttributes.of({
            ...(id ? { id } : {}),
            ...(ariaLabel ? { "aria-label": ariaLabel } : {}),
            ...(describedBy ? { "aria-describedby": describedBy } : {}),
            ...(invalid ? { "aria-invalid": "true" } : {}),
          }),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) onChangeRef.current(update.state.doc.toString());
          }),
        ],
      }),
    });
    viewRef.current = view;
    if (autoFocus) view.focus();
    return () => {
      view.destroy();
      viewRef.current = null;
    };
    // Rebuilding on every prop change would drop the cursor and undo history;
    // `value` is reconciled by the effect below instead.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Adopt externally-set values (a metadata fetch filling the field, a reset on
  // cancel) without clobbering what the user is typing.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current === value) return;
    view.dispatch({ changes: { from: 0, to: current.length, insert: value } });
  }, [value]);

  return <div ref={host} className={cn("w-full overflow-hidden", className)} />;
}
