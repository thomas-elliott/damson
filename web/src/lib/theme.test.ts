import { afterEach, describe, expect, it, vi } from "vitest";

import { applyTheme } from "@/design-system/tokens";
import { resolveInitialTheme, systemTheme, watchSystemTheme, writeStoredTheme } from "@/lib/theme";

type Listener = (e: MediaQueryListEvent) => void;

function stubMatchMedia(prefersDark: boolean) {
  const listeners = new Set<Listener>();
  const mql = {
    matches: prefersDark,
    addEventListener: (_: string, fn: Listener) => listeners.add(fn),
    removeEventListener: (_: string, fn: Listener) => listeners.delete(fn),
  };
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => mql),
  );
  return {
    emit(dark: boolean) {
      for (const fn of listeners) fn({ matches: dark } as MediaQueryListEvent);
    },
    listenerCount: () => listeners.size,
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
  window.localStorage.clear();
  document.documentElement.classList.remove("dark");
});

describe("systemTheme", () => {
  it("reports dark when the OS prefers dark", () => {
    stubMatchMedia(true);
    expect(systemTheme()).toBe("dark");
  });

  it("reports light when the OS prefers light", () => {
    stubMatchMedia(false);
    expect(systemTheme()).toBe("light");
  });
});

describe("resolveInitialTheme", () => {
  it("follows the OS when nothing is stored", () => {
    stubMatchMedia(false);
    expect(resolveInitialTheme()).toBe("light");
    stubMatchMedia(true);
    expect(resolveInitialTheme()).toBe("dark");
  });

  it("prefers an explicit stored choice over the OS", () => {
    stubMatchMedia(true);
    writeStoredTheme("light");
    expect(resolveInitialTheme()).toBe("light");

    stubMatchMedia(false);
    writeStoredTheme("dark");
    expect(resolveInitialTheme()).toBe("dark");
  });

  it("ignores a stored value that is not a theme", () => {
    stubMatchMedia(true);
    window.localStorage.setItem("damson.theme", "chartreuse");
    expect(resolveInitialTheme()).toBe("dark");
  });
});

describe("watchSystemTheme", () => {
  it("reports OS changes and unsubscribes cleanly", () => {
    const mm = stubMatchMedia(false);
    const seen: string[] = [];
    const stop = watchSystemTheme((m) => seen.push(m));

    mm.emit(true);
    mm.emit(false);
    expect(seen).toEqual(["dark", "light"]);

    stop();
    expect(mm.listenerCount()).toBe(0);
    mm.emit(true);
    expect(seen).toEqual(["dark", "light"]);
  });
});

describe("applyTheme", () => {
  it("adds the dark class for dark and removes it for light", () => {
    applyTheme("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);

    applyTheme("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });

  it("does not set the legacy data-theme attribute", () => {
    applyTheme("light");
    expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
  });
});
