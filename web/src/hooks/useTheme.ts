import { useCallback, useEffect, useState } from "react";

import { type ThemeMode } from "@/design-system/tokens";
import { applyAndStore, resolveInitialTheme } from "@/lib/theme";
import { useLibrarySettings } from "@/hooks/useDataQueries";

// useTheme returns the active theme mode and a setter that updates both the
// document attribute and localStorage. The library's `theme_default` seeds
// the initial value when the user hasn't picked one yet — once they pick,
// the stored choice wins forever.
export function useTheme() {
  const { data: settings } = useLibrarySettings();
  const [mode, setMode] = useState<ThemeMode>(() => resolveInitialTheme(settings?.theme_default));

  // Reflect the active mode onto the document (and persist it) whenever it
  // changes — including the first render, so a page hard-refreshed inside a
  // non-default theme doesn't flash dark. This effect runs on mount and so
  // already stores the resolved theme; a separate "adopt the library default
  // once settings load" effect would find a stored value here and never fire,
  // so the library default is seeded only through resolveInitialTheme above.
  useEffect(() => {
    applyAndStore(mode);
  }, [mode]);

  const setTheme = useCallback((next: ThemeMode) => {
    applyAndStore(next);
    setMode(next);
  }, []);

  const toggle = useCallback(() => {
    setTheme(mode === "dark" ? "light" : "dark");
  }, [mode, setTheme]);

  return { mode, setTheme, toggle } as const;
}
