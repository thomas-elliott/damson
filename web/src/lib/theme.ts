import { applyTheme, type ThemeMode } from "@/design-system/tokens";

const STORAGE_KEY = "damson.theme";

const DARK_QUERY = "(prefers-color-scheme: dark)";

export function readStoredTheme(): ThemeMode | null {
  try {
    const v = window.localStorage.getItem(STORAGE_KEY);
    if (v === "dark" || v === "light") return v;
  } catch {
    // localStorage may throw in privacy modes; fall through to null.
  }
  return null;
}

export function writeStoredTheme(mode: ThemeMode | null) {
  try {
    if (mode == null) {
      window.localStorage.removeItem(STORAGE_KEY);
    } else {
      window.localStorage.setItem(STORAGE_KEY, mode);
    }
  } catch {
    // Privacy mode — non-fatal; we still applied the theme to the document.
  }
}

export function systemTheme(): ThemeMode {
  try {
    return window.matchMedia(DARK_QUERY).matches ? "dark" : "light";
  } catch {
    return "dark";
  }
}

export function watchSystemTheme(onChange: (mode: ThemeMode) => void): () => void {
  let mql: MediaQueryList;
  try {
    mql = window.matchMedia(DARK_QUERY);
  } catch {
    return () => {};
  }
  const handler = (e: MediaQueryListEvent) => onChange(e.matches ? "dark" : "light");
  mql.addEventListener("change", handler);
  return () => mql.removeEventListener("change", handler);
}

export function applyAndStore(mode: ThemeMode) {
  applyTheme(mode);
  writeStoredTheme(mode);
}

export function resolveInitialTheme(): ThemeMode {
  return readStoredTheme() ?? systemTheme();
}
