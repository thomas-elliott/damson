import { useEffect } from "react";

/**
 * Bind document-level keyboard shortcuts. The handler is skipped when the
 * event target is an editable element (input/textarea/contenteditable) so
 * shortcuts don't steal keystrokes from forms.
 *
 * Shortcuts come in two flavours:
 *   - Plain key (e.g. "/", "Escape", "ArrowDown")
 *   - Modifier+key in "ctrl+a" / "meta+k" form. macOS users get cmd via
 *     `meta`; cross-platform shortcuts can register both keys.
 *
 * The handler is rebound on every change to `bindings` — pass a memoised
 * object if you want to avoid this.
 */
export type KeyHandler = (e: KeyboardEvent) => void;

export interface KeyBinding {
  /** "Escape", "/", "Enter", "ArrowUp", … */
  key: string;
  /** Set true to require a modifier. Default: ignore modifier state. */
  meta?: boolean;
  ctrl?: boolean;
  shift?: boolean;
  handler: KeyHandler;
  /** When true, the handler still fires while focus is in an editable element. */
  allowInEditable?: boolean;
}

export function useKeyboardShortcuts(bindings: KeyBinding[]) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const inEditable =
        !!target &&
        (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable);

      for (const b of bindings) {
        if (b.key.length === 1 ? e.key.toLowerCase() !== b.key.toLowerCase() : e.key !== b.key) {
          continue;
        }
        if (b.meta && !e.metaKey) continue;
        if (b.ctrl && !e.ctrlKey) continue;
        if (b.shift && !e.shiftKey) continue;
        if (!b.meta && e.metaKey && b.ctrl !== true) continue;
        if (!b.ctrl && e.ctrlKey && b.meta !== true) continue;
        if (inEditable && !b.allowInEditable) continue;
        b.handler(e);
        return;
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [bindings]);
}
