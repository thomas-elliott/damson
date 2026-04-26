import {
  test as base,
  expect,
  type APIRequestContext,
  type Page,
} from "@playwright/test";

export const ADMIN_EMAIL = "admin@e2e.local";
export const ADMIN_PASSWORD = "hunter2hunter2";

export type Session = {
  token: string;
  expiresAt: string;
};

type Fixtures = {
  // Capture browser console.error and pageerror events for the duration of
  // the test and assert nothing was logged at teardown. Opt in by listing
  // `noConsoleErrors` in the test arg destructure — best for boot/smoke
  // paths where any error is a real regression. Skip it for tests that
  // intentionally drive failure paths.
  noConsoleErrors: void;
  // Wipe + re-AutoMigrate the e2e backend before every test. Auto-applied
  // so each test starts from a known-empty schema. This relies on the
  // /api/test/reset endpoint exposed by cmd/e2e-server (test build only).
  resetBackend: void;
};

export const test = base.extend<Fixtures>({
  resetBackend: [
    async ({ request }, use) => {
      await resetBackend(request);
      await use();
    },
    { auto: true },
  ],
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

export async function resetBackend(request: APIRequestContext): Promise<void> {
  const res = await request.post("/api/test/reset");
  if (!res.ok()) {
    throw new Error(`reset failed: ${res.status()} ${await res.text()}`);
  }
}

// bootstrapAdmin creates the very first user via /api/auth/bootstrap. The
// server-side handler also creates a default library and membership, so
// the returned token is immediately usable for library-scoped routes.
export async function bootstrapAdmin(
  request: APIRequestContext,
  email = ADMIN_EMAIL,
  password = ADMIN_PASSWORD,
): Promise<Session> {
  const res = await request.post("/api/auth/bootstrap", {
    data: { email, password },
  });
  if (!res.ok()) {
    throw new Error(`bootstrap failed: ${res.status()} ${await res.text()}`);
  }
  const body = (await res.json()) as { token: string; expires_at: string };
  return { token: body.token, expiresAt: body.expires_at };
}

// loginViaApi exchanges credentials for a token without driving the UI.
// Use for tests that need an authenticated session but aren't testing the
// login form itself.
export async function loginViaApi(
  request: APIRequestContext,
  email = ADMIN_EMAIL,
  password = ADMIN_PASSWORD,
): Promise<Session> {
  const res = await request.post("/api/auth/login", {
    data: { email, password },
  });
  if (!res.ok()) {
    throw new Error(`login failed: ${res.status()} ${await res.text()}`);
  }
  const body = (await res.json()) as { token: string; expires_at: string };
  return { token: body.token, expiresAt: body.expires_at };
}

// installSession seeds the local-auth localStorage keys before the SPA
// loads so AuthGuard resolves "ready" immediately on first navigation,
// no login form interaction required. Mirrors lib/localAuth.ts.
export async function installSession(page: Page, session: Session): Promise<void> {
  await page.addInitScript(
    ({ token, expiresAt }) => {
      window.localStorage.setItem("damson.local.token", token);
      window.localStorage.setItem("damson.local.expiresAt", expiresAt);
    },
    session,
  );
}

// uploadImport posts a multipart import directly to the backend. Used to
// seed assets in tests that aren't exercising the UI dropzone.
export async function uploadImport(
  request: APIRequestContext,
  session: Session,
  filename: string,
  fileBytes: Buffer,
  contentType = "application/octet-stream",
): Promise<{ id: number; asset_id: number }> {
  const res = await request.post("/api/imports", {
    headers: { Authorization: `Bearer ${session.token}` },
    multipart: {
      file: { name: filename, mimeType: contentType, buffer: fileBytes },
    },
  });
  if (!res.ok()) {
    throw new Error(`import failed: ${res.status()} ${await res.text()}`);
  }
  return (await res.json()) as { id: number; asset_id: number };
}

export { expect };
