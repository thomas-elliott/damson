import "@testing-library/jest-dom/vitest";

// jsdom has no ResizeObserver, and Radix's ScrollArea (inside FilterSidebar)
// constructs one on mount — without this, any test rendering a sidebar throws.
if (!("ResizeObserver" in globalThis)) {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}
