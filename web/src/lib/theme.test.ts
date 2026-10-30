import { afterEach, describe, expect, it, vi } from "vitest";

import { applyTheme } from "@/design-system/tokens";
import { resolveInitialTheme, writeStoredTheme } from "@/lib/theme";

function stubOsPreference(prefersDark: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => ({ matches: prefersDark, addEventListener() {}, removeEventListener() {} })),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  window.localStorage.clear();
  document.documentElement.classList.remove("dark");
});

describe("resolveInitialTheme", () => {
  it("defaults to dark when nothing is stored, whatever the OS prefers", () => {
    stubOsPreference(false);
    expect(resolveInitialTheme()).toBe("dark");
  });

  it("uses an explicit stored choice", () => {
    writeStoredTheme("light");
    expect(resolveInitialTheme()).toBe("light");

    writeStoredTheme("dark");
    expect(resolveInitialTheme()).toBe("dark");
  });

  it("ignores a stored value that is not a theme", () => {
    window.localStorage.setItem("damson.theme", "chartreuse");
    expect(resolveInitialTheme()).toBe("dark");
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
