import "./tokens.css";

export type ThemeMode = "dark" | "light";

export function applyTheme(mode: ThemeMode) {
  document.documentElement.classList.toggle("dark", mode === "dark");
}

export const LICENCE_STATUSES = ["ok", "warn", "err", "unk"] as const;
export type LicenceStatus = (typeof LICENCE_STATUSES)[number];
