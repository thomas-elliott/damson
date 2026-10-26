import { test as base, expect, type APIRequestContext, type Page } from "@playwright/test";

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
  // Truncate the e2e backend's data tables before every test. Auto-applied
  // so each test starts from empty tables. This relies on the
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
  await page.addInitScript(({ token, expiresAt }) => {
    window.localStorage.setItem("damson.local.token", token);
    window.localStorage.setItem("damson.local.expiresAt", expiresAt);
  }, session);
}

// createLibrary creates a library via the API and returns its id. The
// caller becomes the library's admin; their current_library_id is left
// untouched, so a follow-up switchCurrentLibrary is required before any
// library-scoped seeding (uploadImport) lands in the new library.
export async function createLibrary(
  request: APIRequestContext,
  session: Session,
  name: string,
): Promise<{ id: number; name: string }> {
  const res = await request.post("/api/libraries", {
    headers: { Authorization: `Bearer ${session.token}` },
    data: { name },
  });
  if (!res.ok()) {
    throw new Error(`create library failed: ${res.status()} ${await res.text()}`);
  }
  return (await res.json()) as { id: number; name: string };
}

// switchCurrentLibrary flips the authenticated user's active library via
// the API. Used to pre-stage scope before seeding library-scoped rows in
// E2E setup; the test itself drives the UI picker for the real switch.
export async function switchCurrentLibrary(
  request: APIRequestContext,
  session: Session,
  libraryId: number,
): Promise<void> {
  const res = await request.post("/api/me/current-library", {
    headers: { Authorization: `Bearer ${session.token}` },
    data: { library_id: libraryId },
  });
  if (!res.ok()) {
    throw new Error(`switch library failed: ${res.status()} ${await res.text()}`);
  }
}

// uploadImport posts a multipart import directly to the backend, then
// blocks until the background worker reaches a terminal stage so the
// caller can rely on asset_files + current_version being populated. POST
// /api/imports returns 202 with only {asset_id, import_id}; the
// asset_version_id is resolved from /api/assets/:id once `done`.
export async function uploadImport(
  request: APIRequestContext,
  session: Session,
  filename: string,
  fileBytes: Buffer,
  contentType = "application/octet-stream",
): Promise<{ id: number; asset_id: number; asset_version_id: number }> {
  const res = await request.post("/api/imports", {
    headers: { Authorization: `Bearer ${session.token}` },
    multipart: {
      file: { name: filename, mimeType: contentType, buffer: fileBytes },
    },
  });
  if (!res.ok()) {
    throw new Error(`import failed: ${res.status()} ${await res.text()}`);
  }
  const { asset_id, import_id } = (await res.json()) as {
    asset_id: number;
    import_id: number;
  };

  await waitForImportDone(request, session, import_id);

  // current_version_id is set when the worker finalizes; pull it from
  // /api/assets/:id so subset/pin tests can keep using a single version id.
  const detail = await request.get(`/api/assets/${asset_id}`, {
    headers: { Authorization: `Bearer ${session.token}` },
  });
  if (!detail.ok()) {
    throw new Error(`asset detail failed: ${detail.status()} ${await detail.text()}`);
  }
  const body = (await detail.json()) as {
    versions: { id: number; is_current: boolean }[];
  };
  const current = body.versions.find((v) => v.is_current) ?? body.versions[0];
  if (!current) {
    throw new Error("import finished but asset has no versions");
  }
  return { id: import_id, asset_id, asset_version_id: current.id };
}

// waitForImportDone polls /api/imports/:id until the row reaches a
// terminal stage. Failed imports throw with the recorded error so the
// caller doesn't carry on against half-built rows.
async function waitForImportDone(
  request: APIRequestContext,
  session: Session,
  importId: number,
): Promise<void> {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const res = await request.get(`/api/imports/${importId}`, {
      headers: { Authorization: `Bearer ${session.token}` },
    });
    if (res.ok()) {
      const row = (await res.json()) as { stage: string; error?: string };
      if (row.stage === "done") return;
      if (row.stage === "failed") {
        throw new Error(`import failed: ${row.error ?? "(no error message)"}`);
      }
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`import ${importId} did not finish within 15s`);
}

// createReferenceAsset records an asset by its location, with no import, so
// a spec can fill a library past a page quickly.
export async function createReferenceAsset(
  request: APIRequestContext,
  session: Session,
  name: string,
): Promise<{ asset_id: number }> {
  const res = await request.post("/api/assets/references", {
    headers: { Authorization: `Bearer ${session.token}` },
    data: { name, location_path: `/library/${name.toLowerCase().replaceAll(" ", "-")}` },
  });
  if (!res.ok()) {
    throw new Error(`create reference asset: ${res.status()} ${await res.text()}`);
  }
  return (await res.json()) as { asset_id: number };
}

// createTestUser hits the /api/test/users endpoint exposed by the e2e
// binary. There's no public admin-invite flow yet, so multi-user specs
// (e.g. PER-82's sysadmin-vs-member gating) need this lever.
export async function createTestUser(
  request: APIRequestContext,
  opts: {
    email: string;
    password: string;
    role?: "user" | "sysadmin";
    libraryId?: number;
    membershipRole?: "admin" | "member";
    setCurrent?: boolean;
  },
): Promise<{ id: number; email: string; role: string }> {
  const res = await request.post("/api/test/users", {
    data: {
      email: opts.email,
      password: opts.password,
      role: opts.role,
      library_id: opts.libraryId,
      membership_role: opts.membershipRole,
      set_current: opts.setCurrent,
    },
  });
  if (!res.ok()) {
    throw new Error(`create test user failed: ${res.status()} ${await res.text()}`);
  }
  return (await res.json()) as { id: number; email: string; role: string };
}

// subsetVersion creates a new asset version via the subset endpoint. The
// new version becomes the asset's current_version. Used in PER-79 to seed
// a v2 against an asset whose v1 came from an import — without a public
// "add version" API this is the only way to get pin-upgrade arithmetic
// without a second multipart upload.
export async function subsetVersion(
  request: APIRequestContext,
  session: Session,
  assetId: number,
  sourceVersionId: number,
  fileIds: number[],
  versionLabel?: string,
): Promise<{ asset_id: number; asset_version_id: number; version_label: string }> {
  const res = await request.post(`/api/assets/${assetId}/versions/${sourceVersionId}/subset`, {
    headers: { Authorization: `Bearer ${session.token}` },
    data: { file_ids: fileIds, version_label: versionLabel },
  });
  if (!res.ok()) {
    throw new Error(`subset failed: ${res.status()} ${await res.text()}`);
  }
  return (await res.json()) as {
    asset_id: number;
    asset_version_id: number;
    version_label: string;
  };
}

// getAssetDetail fetches the same payload the SPA's detail panel renders.
// Tests use it to discover file_ids for subsetVersion without driving any
// UI.
export async function getAssetDetail(
  request: APIRequestContext,
  session: Session,
  assetId: number,
): Promise<{
  id: number;
  current_version_id: number | null;
  files: Array<{ id: number; relative_path: string }>;
}> {
  const res = await request.get(`/api/assets/${assetId}`, {
    headers: { Authorization: `Bearer ${session.token}` },
  });
  if (!res.ok()) {
    throw new Error(`get asset detail: ${res.status()} ${await res.text()}`);
  }
  return (await res.json()) as {
    id: number;
    current_version_id: number | null;
    files: Array<{ id: number; relative_path: string }>;
  };
}

// addAssetToCollection / removeAssetFromCollection wrap the membership
// API. PER-80 uses these because the SPA doesn't expose an add-to-
// collection UI yet (the hooks exist in useMutations but no page wires
// them).
export async function addAssetToCollection(
  request: APIRequestContext,
  session: Session,
  collectionId: number,
  assetId: number,
): Promise<void> {
  const res = await request.post(`/api/collections/${collectionId}/assets`, {
    headers: { Authorization: `Bearer ${session.token}` },
    data: { asset_id: assetId },
  });
  if (!res.ok()) {
    throw new Error(`add asset to collection: ${res.status()} ${await res.text()}`);
  }
}

export async function removeAssetFromCollection(
  request: APIRequestContext,
  session: Session,
  collectionId: number,
  assetId: number,
): Promise<void> {
  const res = await request.delete(`/api/collections/${collectionId}/assets/${assetId}`, {
    headers: { Authorization: `Bearer ${session.token}` },
  });
  if (!res.ok()) {
    throw new Error(`remove asset from collection: ${res.status()} ${await res.text()}`);
  }
}

// listAuditEntries returns the library-scoped audit feed. Tests use it to
// assert specific action_type rows exist (e.g. project.pin.upgrade in
// PER-79) without depending on the audit UI's pagination.
export async function listAuditEntries(
  request: APIRequestContext,
  session: Session,
  query: Record<string, string | number | undefined> = {},
): Promise<
  Array<{
    id: number;
    action_type: string;
    target_type: string;
    target_id: number | null;
    diff: unknown;
  }>
> {
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v !== undefined) params.set(k, String(v));
  }
  const url = "/api/audit" + (params.toString() ? `?${params.toString()}` : "");
  const res = await request.get(url, {
    headers: { Authorization: `Bearer ${session.token}` },
  });
  if (!res.ok()) {
    throw new Error(`list audit: ${res.status()} ${await res.text()}`);
  }
  const body = (await res.json()) as {
    entries: Array<{
      id: number;
      action_type: string;
      target_type: string;
      target_id: number | null;
      diff: unknown;
    }>;
  };
  return body.entries;
}

export { expect };
