import { applyTheme, type ThemeMode } from "@/design-system/tokens";

const STORAGE_KEY = "damson.theme";

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

export function applyAndStore(mode: ThemeMode) {
  applyTheme(mode);
  writeStoredTheme(mode);
}

// Dark is Damson's default; light only when someone has chosen it.
export function resolveInitialTheme(): ThemeMode {
  return readStoredTheme() ?? "dark";
}
