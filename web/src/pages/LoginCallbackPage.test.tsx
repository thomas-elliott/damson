import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ErrorResponse } from "oidc-client-ts";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

// A plain function, not vi.fn: vitest tracks a vi.fn's returned promise, and
// reports a rejected one as unhandled even though the page catches it.
const oidc = vi.hoisted(() => ({
  signinRedirectCallback: (): Promise<unknown> => Promise.resolve({}),
}));

vi.mock("@/lib/authMode", () => ({ fetchAuthMode: vi.fn().mockResolvedValue({}) }));
vi.mock("@/lib/oidc", () => ({
  getUserManager: () => ({
    signinRedirectCallback: () => oidc.signinRedirectCallback(),
    signinSilentCallback: vi.fn(),
  }),
}));

function Requested() {
  const { pathname, search, hash } = useLocation();
  return <p>Requested {pathname + search + hash}</p>;
}

// The callback is memoised per module, as it is per document, so each test
// gets a fresh copy of the page.
async function renderCallback() {
  vi.resetModules();
  const { LoginCallbackPage } = await import("./LoginCallbackPage");
  render(
    <MemoryRouter initialEntries={["/auth/callback"]}>
      <Routes>
        <Route path="/auth/callback" element={<LoginCallbackPage />} />
        <Route path="/" element={<p>Home</p>} />
        <Route path="/library/:assetId" element={<Requested />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  oidc.signinRedirectCallback = () => Promise.resolve({});
});

describe("LoginCallbackPage", () => {
  it("goes home once sign-in completes", async () => {
    await renderCallback();
    expect(await screen.findByText("Home")).toBeInTheDocument();
  });

  it("returns to the page that was asked for, with its query and hash", async () => {
    oidc.signinRedirectCallback = () =>
      Promise.resolve({ state: { returnTo: "/library/42?q=x#files" } });
    await renderCallback();
    expect(await screen.findByText("Requested /library/42?q=x#files")).toBeInTheDocument();
  });

  it.each(["https://evil.example/", "//evil.example/library"])(
    "goes home instead of to %s, outside the app",
    async (returnTo) => {
      oidc.signinRedirectCallback = () => Promise.resolve({ state: { returnTo } });
      await renderCallback();
      expect(await screen.findByText("Home")).toBeInTheDocument();
    },
  );

  it("stays on a failed sign-in, shows the IdP's description, and offers to sign in again", async () => {
    oidc.signinRedirectCallback = () =>
      Promise.reject(
        new ErrorResponse({ error: "access_denied", error_description: "The user cancelled." }),
      );
    await renderCallback();
    expect(await screen.findByText("The user cancelled.")).toBeInTheDocument();
    expect(screen.queryByText("Home")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Sign in again" }));
    expect(await screen.findByText("Home")).toBeInTheDocument();
  });

  it("shows a fixed line for a failure the IdP did not describe", async () => {
    oidc.signinRedirectCallback = () =>
      Promise.reject(new Error("No matching state found in storage"));
    await renderCallback();
    expect(
      await screen.findByText("Something went wrong while signing you in. Try signing in again."),
    ).toBeInTheDocument();
    expect(screen.queryByText("No matching state found in storage")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sign in again" })).toBeInTheDocument();
  });
});
