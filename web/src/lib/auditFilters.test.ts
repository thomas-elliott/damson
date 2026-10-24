import { describe, expect, it } from "vitest";

import ACTION_TYPES from "./auditActionTypes.json";
import { ACTION_FILTERS } from "./auditFilters";
import { VERBS } from "./auditFormat";

// Mirrors the server's action_type matching: an exact type, or a "family.*"
// prefix.
function covers(filter: string, actionType: string) {
  return filter
    .split(",")
    .some((entry) =>
      entry.endsWith(".*") ? actionType.startsWith(entry.slice(0, -1)) : entry === actionType,
    );
}

// auditActionTypes.json is the backend's list of every type it writes; a Go
// test keeps it in step with audit.ActionTypes.
describe("audit action types", () => {
  it.each(ACTION_TYPES)("has a label for %s", (action) => {
    expect(VERBS[action]).toBeDefined();
  });

  it("labels no type the backend does not write", () => {
    expect(Object.keys(VERBS).filter((a) => !ACTION_TYPES.includes(a))).toEqual([]);
  });

  // Every library action must be reachable from a chip, or it only ever shows
  // under All. admin.* events are server-level and never reach the library
  // feed.
  const libraryActions = ACTION_TYPES.filter((a) => !a.startsWith("admin."));

  it.each(libraryActions)("has a chip for %s", (action) => {
    expect(ACTION_FILTERS.some((f) => f.value && covers(f.value, action))).toBe(true);
  });
});
