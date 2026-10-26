import { describe, expect, it } from "vitest";

import { safeReturnTo } from "./returnTo";

describe("safeReturnTo", () => {
  it("keeps an in-app path with its query and hash", () => {
    expect(safeReturnTo("/library/42?q=x&page=2#files")).toBe("/library/42?q=x&page=2#files");
  });

  it("takes a router location", () => {
    expect(safeReturnTo({ pathname: "/library/42", search: "?q=x", hash: "#h" })).toBe(
      "/library/42?q=x#h",
    );
  });

  it.each([undefined, null, "", 42, {}, { pathname: 7 }])("falls back to / for %j", (value) => {
    expect(safeReturnTo(value)).toBe("/");
  });

  it.each([
    "https://evil.example/",
    "//evil.example/library",
    "/\\evil.example",
    "javascript:alert(1)",
    "library/42",
    "/\t/evil.example",
    "/\n/evil.example",
  ])("ignores %j, which is not a page of this app", (value) => {
    expect(safeReturnTo(value)).toBe("/");
  });

  it.each(["/login", "/login/", "/login?x=1", "/auth/callback?code=1", "/bootstrap"])(
    "does not return to the sign-in page %j",
    (value) => {
      expect(safeReturnTo(value)).toBe("/");
    },
  );
});
