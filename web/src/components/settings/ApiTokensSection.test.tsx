import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiTokensSection } from "./ApiTokensSection";

const fetchJsonMock = vi.fn();
const fetchMaybeJsonMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return {
    ...actual,
    fetchJson: (...args: unknown[]) => fetchJsonMock(...args),
    fetchMaybeJson: (...args: unknown[]) => fetchMaybeJsonMock(...args),
  };
});

const SECRET = "damson_pat_s3cr3tvalue00000000000000000000000000";

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ApiTokensSection />
    </QueryClientProvider>,
  );
}

function tokenRow(over: Record<string, unknown> = {}) {
  return {
    id: 1,
    label: "laptop sync",
    prefix: "damson_pat_abcd1234",
    scope: "write",
    library_id: 7,
    library_name: "Main",
    created_at: "2026-01-01T00:00:00Z",
    last_used_at: null,
    expires_at: null,
    revoked_at: null,
    ...over,
  };
}

describe("ApiTokensSection", () => {
  beforeEach(() => {
    fetchJsonMock.mockReset();
    fetchMaybeJsonMock.mockReset();
  });

  it("lists tokens without ever exposing a secret", async () => {
    fetchJsonMock.mockResolvedValue({
      tokens: [tokenRow(), tokenRow({ id: 2, label: "read only", scope: "read" })],
    });
    const { container } = mount();

    expect(await screen.findByText("laptop sync")).toBeInTheDocument();
    expect(screen.getByText("read only")).toBeInTheDocument();
    // Two active tokens, reflected in the heading count.
    expect(screen.getByText(/Access tokens \(2\)/)).toBeInTheDocument();
    // The list response carries no secret, so none can be on screen.
    expect(container.textContent).not.toContain(SECRET);
  });

  it("shows the secret exactly once, then never again", async () => {
    fetchJsonMock.mockImplementation((_url: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        return Promise.resolve({ ...tokenRow({ id: 9, label: "cli" }), token: SECRET });
      }
      return Promise.resolve({ tokens: [] });
    });
    const user = userEvent.setup();
    mount();

    await user.click(await screen.findByRole("button", { name: "New token" }));
    await user.type(screen.getByPlaceholderText("laptop sync"), "cli");
    await user.click(screen.getByRole("button", { name: "Create" }));

    const field = await screen.findByLabelText("Access token");
    expect(field).toHaveValue(SECRET);

    // The POST body carried the label and the default write scope.
    const post = fetchJsonMock.mock.calls.find(
      (c) => (c[1] as RequestInit | undefined)?.method === "POST",
    );
    expect(post?.[0]).toBe("/api/me/tokens");
    expect(JSON.parse((post?.[1] as RequestInit).body as string)).toMatchObject({
      label: "cli",
      scope: "write",
    });

    // Dismissing drops it: the secret is gone from the DOM and the only
    // remaining source of truth is the (secret-free) list.
    await user.click(screen.getByRole("button", { name: "Done" }));
    await waitFor(() => {
      expect(screen.queryByLabelText("Access token")).not.toBeInTheDocument();
    });
    expect(document.body.textContent).not.toContain(SECRET);
  });

  it("reports a clipboard failure instead of silently doing nothing", async () => {
    fetchJsonMock.mockImplementation((_url: string, init?: RequestInit) =>
      init?.method === "POST"
        ? Promise.resolve({ ...tokenRow({ id: 9 }), token: SECRET })
        : Promise.resolve({ tokens: [] }),
    );
    const user = userEvent.setup();
    // Reproduce a non-secure context exactly: browsers leave
    // navigator.clipboard undefined, so the call throws rather than rejecting.
    // Applied after userEvent.setup(), which installs its own stub.
    Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
    mount();
    await user.click(await screen.findByRole("button", { name: "New token" }));
    await user.type(screen.getByPlaceholderText("laptop sync"), "cli");
    await user.click(screen.getByRole("button", { name: "Create" }));
    await user.click(await screen.findByRole("button", { name: /Copy/ }));

    expect(await screen.findByText(/Couldn.t reach the clipboard/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Copied" })).not.toBeInTheDocument();
  });

  it("surfaces a failed revoke rather than appearing to succeed", async () => {
    fetchJsonMock.mockResolvedValue({ tokens: [tokenRow()] });
    fetchMaybeJsonMock.mockRejectedValue(new Error("token not found"));
    const user = userEvent.setup();
    mount();

    await user.click(await screen.findByRole("button", { name: /Revoke/ }));
    await user.click(await screen.findByRole("button", { name: "Revoke" }));

    expect(await screen.findByText(/Couldn.t revoke laptop sync/)).toBeInTheDocument();
    // The row is still there — nothing pretended to happen.
    expect(screen.getByText("laptop sync")).toBeInTheDocument();
  });

  it("does not claim there are no tokens when the list failed to load", async () => {
    fetchJsonMock.mockRejectedValue(new Error("boom"));
    mount();

    expect(await screen.findByText("Couldn't load tokens")).toBeInTheDocument();
    expect(screen.queryByText("No access tokens")).not.toBeInTheDocument();
  });

  it("distinguishes a revoked token from an active one", async () => {
    fetchJsonMock.mockResolvedValue({
      tokens: [tokenRow({ revoked_at: "2026-02-01T00:00:00Z" })],
    });
    mount();

    expect(await screen.findByText("Revoked")).toBeInTheDocument();
    // A revoked token offers no revoke action, and doesn't count as active.
    expect(screen.queryByRole("button", { name: /Revoke/ })).not.toBeInTheDocument();
    expect(screen.getByText(/Access tokens \(0\)/)).toBeInTheDocument();
  });
});
