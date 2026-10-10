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
  { tag: tags.heading, color: "var(--foreground)", fontWeight: "600" },
  { tag: tags.strong, color: "var(--foreground)", fontWeight: "600" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.strikethrough, textDecoration: "line-through", color: "var(--subtle-foreground)" },
  { tag: tags.link, color: "var(--primary)" },
  { tag: tags.url, color: "var(--primary)" },
  { tag: tags.monospace, fontFamily: "var(--font-mono)", color: "var(--muted-foreground)" },
  { tag: tags.quote, color: "var(--muted-foreground)", fontStyle: "italic" },
  { tag: [tags.processingInstruction, tags.meta, tags.atom], color: "var(--faint-foreground)" },
]);

const theme = EditorView.theme({
  "&": {
    fontSize: "14px",
    color: "var(--foreground)",
    backgroundColor: "var(--background)",
    // The `rounded-sm` derivation spelled out: `@theme inline` emits the utility
    // but no `--radius-sm` property, so there is nothing to reference here.
    borderRadius: "var(--radius-sm)",
    border: "1px solid var(--border-strong)",
    transition: "border-color 100ms cubic-bezier(0.4, 0, 0.2, 1)",
  },
  "&.cm-focused": {
    outline: "none",
    borderColor: "var(--primary)",
    boxShadow: "inset 0 0 0 2px var(--ring)",
  },
  ".cm-scroller": { fontFamily: "var(--font-sans)", lineHeight: "1.5" },
  ".cm-content": { padding: "8px", caretColor: "var(--foreground)" },
  ".cm-line": { padding: "0" },
  ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--foreground)" },
  ".cm-placeholder": { color: "var(--faint-foreground)" },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": {
    backgroundColor: "var(--primary-soft)",
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
            // Already a tab stop as an editing host. Saying so explicitly lets
            // axe see the capped, scrollable editor as keyboard-reachable.
            tabindex: "0",
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
