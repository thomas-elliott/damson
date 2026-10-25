import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
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

  it("stays on a failed sign-in, shows why, and offers to sign in again", async () => {
    oidc.signinRedirectCallback = () => Promise.reject(new Error("access_denied: user cancelled"));
    await renderCallback();
    expect(await screen.findByText("access_denied: user cancelled")).toBeInTheDocument();
    expect(screen.queryByText("Home")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Sign in again" }));
    expect(await screen.findByText("Home")).toBeInTheDocument();
  });
});
