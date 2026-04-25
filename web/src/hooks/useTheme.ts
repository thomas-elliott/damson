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
  const [mode, setMode] = useState<ThemeMode>(() =>
    resolveInitialTheme(settings?.theme_default),
  );

  // When the library's default arrives (or changes via PATCH) and the user
  // hasn't stored an explicit choice yet, fall in line with the library.
  useEffect(() => {
    const stored = window.localStorage.getItem("damson.theme");
    if (!stored && settings?.theme_default) {
      const next = settings.theme_default === "light" ? "light" : "dark";
      applyAndStore(next);
      setMode(next);
    }
  }, [settings?.theme_default]);

  // Reflect the active mode onto the document the first render too, so a
  // page hard-refreshed inside a non-default theme doesn't flash dark.
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
