import { describe, expect, it } from "vitest";

import { formatAuditAction } from "./auditFormat";

// The keys must be the action types the server writes, or the feed shows the
// raw dotted string instead of a label.
describe("formatAuditAction", () => {
  it.each([
    ["asset.create", "imported"],
    ["asset.version.create", "imported a version of"],
    ["asset.tag", "tagged"],
    ["asset.untag", "untagged"],
    ["collection.add", "added to collection"],
    ["collection.remove", "removed from collection"],
  ])("labels %s", (action, label) => {
    expect(formatAuditAction(action)).toBe(label);
  });
});
