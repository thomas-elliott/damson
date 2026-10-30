import { useCallback, useEffect, useState } from "react";

import { applyTheme, type ThemeMode } from "@/design-system/tokens";
import { applyAndStore, resolveInitialTheme } from "@/lib/theme";

export function useTheme() {
  const [mode, setMode] = useState<ThemeMode>(() => resolveInitialTheme());

  useEffect(() => {
    applyTheme(mode);
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
