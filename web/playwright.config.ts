import { defineConfig, devices } from "@playwright/test";

const PORT = Number(process.env.E2E_PORT ?? 5173);
const BASE_URL = `http://localhost:${PORT}`;

// Default target is `vite dev` so the suite exercises the same dev pipeline
// developers hit locally — Vite plugins, Tailwind transform, on-demand
// imports. That way an upgrade that breaks `vite dev` (but not `vite build`)
// fails E2E instead of slipping through. Set E2E_TARGET=preview to run the
// production bundle via `vite preview` as a parallel check.
const target = process.env.E2E_TARGET ?? "dev";
const webServerCommand =
  target === "preview"
    ? `npm run build && npx vite preview --port ${PORT} --strictPort`
    : `npx vite --port ${PORT} --strictPort`;

export default defineConfig({
  testDir: "./e2e/tests",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI
    ? [["github"], ["html", { open: "never" }]]
    : [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: webServerCommand,
    url: BASE_URL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    stdout: "pipe",
    stderr: "pipe",
  },
});
