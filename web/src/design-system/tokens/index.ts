import "./tokens.css";

export type ThemeMode = "dark" | "light";

export function applyTheme(mode: ThemeMode) {
  if (mode === "light") {
    document.documentElement.setAttribute("data-theme", "light");
  } else {
    document.documentElement.removeAttribute("data-theme");
  }
}

export const LICENCE_STATUSES = ["ok", "warn", "err", "unk"] as const;
export type LicenceStatus = (typeof LICENCE_STATUSES)[number];
