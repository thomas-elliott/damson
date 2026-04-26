import { test as base, expect, type Page } from "@playwright/test";

export type AuthMode = {
  mode: "local" | "oidc";
  needs_bootstrap: boolean;
};

export const DEFAULT_AUTH_MODE: AuthMode = {
  mode: "local",
  needs_bootstrap: false,
};

type Fixtures = {
  // Capture browser console.error and pageerror events for the duration of
  // the test and assert nothing was logged at teardown. Opt in by listing
  // `noConsoleErrors` in the test arg destructure — best for boot/smoke
  // paths where any error is a real regression. Skip it for tests that
  // intentionally drive failure paths.
  noConsoleErrors: void;
};

export const test = base.extend<Fixtures>({
  noConsoleErrors: [
    async ({ page }, use) => {
      const errors: string[] = [];
      page.on("pageerror", (err) => {
        errors.push(`pageerror: ${err.message}`);
      });
      page.on("console", (msg) => {
        if (msg.type() === "error") {
          errors.push(`console.error: ${msg.text()}`);
        }
      });
      await use();
      expect(errors, "browser surfaced unexpected console/page errors").toEqual([]);
    },
    { auto: false },
  ],
});

// mockAuthMode stubs the GET /api/auth/mode response that AuthGuard /
// LoginPage / BootstrapPage all gate on. Call before the first navigation.
export async function mockAuthMode(
  page: Page,
  overrides: Partial<AuthMode> = {},
): Promise<void> {
  const body = { ...DEFAULT_AUTH_MODE, ...overrides };
  await page.route("**/api/auth/mode", (route) => route.fulfill({ json: body }));
}

export { expect };
