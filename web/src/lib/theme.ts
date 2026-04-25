// Theme state management — lives in localStorage so the user's choice
// survives reloads and isn't blocked on /api/settings. The library's
// `theme_default` is the seed when there's no stored preference yet.
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

// applyAndStore is the single mutation entrypoint used by the toggle and the
// initial bootstrap so both follow the same path.
export function applyAndStore(mode: ThemeMode) {
  applyTheme(mode);
  writeStoredTheme(mode);
}

// resolveInitialTheme picks an effective theme on app boot. Stored value
// wins; otherwise we apply the library default (which becomes a no-op `dark`
// when no library is selected yet).
export function resolveInitialTheme(libraryDefault?: ThemeMode | null): ThemeMode {
  const stored = readStoredTheme();
  if (stored) return stored;
  if (libraryDefault === "light") return "light";
  return "dark";
}
