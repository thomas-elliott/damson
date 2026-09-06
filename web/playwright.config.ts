import { defineConfig, devices } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(dirname, "..");

const FRONTEND_PORT = Number(process.env.E2E_PORT ?? 5173);
const BACKEND_PORT = Number(process.env.E2E_BACKEND_PORT ?? 8080);
const FRONTEND_URL = `http://localhost:${FRONTEND_PORT}`;
const BACKEND_URL = `http://localhost:${BACKEND_PORT}`;
const E2E_DATA_DIR = process.env.E2E_DATA_DIR ?? path.join(dirname, "e2e", ".e2e-data");

// Default target is `vite dev` so the suite exercises the same dev pipeline
// developers hit locally — Vite plugins, Tailwind transform, on-demand
// imports. That way an upgrade that breaks `vite dev` (but not `vite build`)
// fails E2E instead of slipping through. Set E2E_TARGET=preview to run the
// production bundle via `vite preview` as a parallel check.
const target = process.env.E2E_TARGET ?? "dev";
const frontendCommand =
  target === "preview"
    ? `npm run build && npx vite preview --port ${FRONTEND_PORT} --strictPort`
    : `npx vite --port ${FRONTEND_PORT} --strictPort`;

// The Go binary is built once into web/e2e/.bin so CI doesn't pay the
// compile cost per Playwright run. Locally, `just test e2e` does the same.
const backendBinary = path.join(dirname, "e2e", ".bin", "damson-e2e");
const backendCommand = `${backendBinary} `; // empty arg list — env-driven

export default defineConfig({
  testDir: "./e2e/tests",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI
    ? [["github"], ["html", { open: "never" }]]
    : [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: FRONTEND_URL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    extraHTTPHeaders: {
      // Tag direct API calls from Playwright (e.g. `apiRequest.post('/api/...')`)
      // so log scrapers can tell test traffic apart from browser traffic.
      "x-damson-e2e": "1",
    },
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  // webServer accepts an array; both processes are started in parallel and
  // Playwright waits for each `url` to respond before running tests. Vite's
  // dev proxy forwards /api/* to the backend port (see vite.config.ts).
  webServer: [
    {
      command: backendCommand,
      url: `${BACKEND_URL}/api/health`,
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
      stdout: "pipe",
      stderr: "pipe",
      env: {
        E2E_LISTEN_ADDR: `:${BACKEND_PORT}`,
        E2E_DATA_DIR,
        E2E_BASE_URL: BACKEND_URL,
        // The e2e binary is Postgres-only and requires this DSN; it opens
        // Postgres and runs the SQL migrations. Forwarded from the process
        // env (set by `just test e2e`, which defaults it to the shared local
        // Postgres on 5432).
        DAMSON_TEST_POSTGRES_DSN: process.env.DAMSON_TEST_POSTGRES_DSN ?? "",
      },
      cwd: repoRoot,
    },
    {
      command: frontendCommand,
      url: FRONTEND_URL,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
      // OIDC config is fed in so getUserManager() in lib/oidc.ts can build
      // a UserManager — used by the oidc-callback E2E spec, which mocks
      // /api/auth/mode + the IdP endpoints via page.route(). Inert in
      // every other test (the SPA stays in local mode because the e2e
      // backend reports `mode: "local"` from /api/auth/mode).
      env: {
        VITE_OIDC_AUTHORITY: "https://oidc-mock.test",
        VITE_OIDC_CLIENT_ID: "damson-test",
        VITE_OIDC_SCOPE: "openid profile email",
        VITE_OIDC_REDIRECT_URI: `${FRONTEND_URL}/auth/callback`,
        VITE_OIDC_SILENT_REDIRECT_URI: `${FRONTEND_URL}/auth/silent-callback`,
        VITE_OIDC_POST_LOGOUT_REDIRECT_URI: `${FRONTEND_URL}/login`,
      },
    },
  ],
});
