import { useCallback, useEffect, useState } from "react";

import { applyTheme, type ThemeMode } from "@/design-system/tokens";
import { applyAndStore, readStoredTheme, resolveInitialTheme, watchSystemTheme } from "@/lib/theme";

export function useTheme() {
  const [mode, setMode] = useState<ThemeMode>(() => resolveInitialTheme());

  // applyTheme, not applyAndStore: storing on mount would mean a stored value
  // always exists, and the follow-the-OS branch below could never run.
  useEffect(() => {
    applyTheme(mode);
  }, [mode]);

  // Depends on `mode` so an explicit choice re-runs this and unsubscribes.
  useEffect(() => {
    if (readStoredTheme() != null) return;
    return watchSystemTheme(setMode);
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
