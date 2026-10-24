import { describe, expect, it } from "vitest";

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

// Every labelled library action must be reachable from a chip, or it only
// ever shows under All. admin.* events are server-level and never reach the
// library feed.
describe("ACTION_FILTERS", () => {
  const libraryActions = Object.keys(VERBS).filter((a) => !a.startsWith("admin."));

  it.each(libraryActions)("has a chip for %s", (action) => {
    expect(ACTION_FILTERS.some((f) => f.value && covers(f.value, action))).toBe(true);
  });
});
