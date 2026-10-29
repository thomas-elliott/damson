import { describe, expect, it } from "vitest";

import { formatAuditAction, formatAuditTarget } from "./auditFormat";

// The keys must be the action types the server writes, or the feed shows the
// raw dotted string instead of a label.
describe("formatAuditAction", () => {
  it.each([
    ["asset.create", "imported"],
    ["asset.version.create", "imported a version of"],
    ["asset.tag", "tagged"],
    ["asset.untag", "untagged"],
    ["collection.add", "added to"],
    ["collection.remove", "removed from"],
  ])("labels %s", (action, label) => {
    expect(formatAuditAction(action)).toBe(label);
  });
});

// The feed renders the verb then the target, so a verb that names its own
// target reads "created project project #1". Each pair is an action type and
// the target_type the server writes it with.
describe("formatAuditAction with formatAuditTarget", () => {
  it.each([
    ["tag.create", "tag", "created tag #5"],
    ["tag.delete", "tag", "deleted tag #5"],
    ["project.create", "project", "created project #5"],
    ["project.update", "project", "updated project #5"],
    ["project.delete", "project", "deleted project #5"],
    ["project.pin", "project", "pinned to project #5"],
    ["project.pin.change", "project", "changed the pinned version in project #5"],
    ["project.unpin", "project", "unpinned from project #5"],
    ["collection.create", "collection", "created collection #5"],
    ["collection.update", "collection", "updated collection #5"],
    ["collection.delete", "collection", "deleted collection #5"],
    ["collection.add", "collection", "added to collection #5"],
    ["collection.remove", "collection", "removed from collection #5"],
    ["token.create", "api_token", "created token #5"],
    ["token.revoke", "api_token", "revoked token #5"],
    ["library.create", "library", "created library #5"],
    ["library.update", "library", "updated library #5"],
    ["library.delete", "library", "deleted library #5"],
    ["library.settings.update", "library", "updated settings of library #5"],
    ["admin.user.update", "user", "updated user #5"],
    ["admin.storage.orphan_delete", "storage_object", "deleted orphaned storage object #5"],
  ])("reads %s on a %s as one phrase", (action, targetType, phrase) => {
    expect(`${formatAuditAction(action)} ${formatAuditTarget(targetType, 5)}`).toBe(phrase);
  });
});

describe("formatAuditTarget", () => {
  it("names a target by its name when it has one", () => {
    expect(formatAuditTarget("project", 1, "Ashfall")).toBe("project Ashfall");
  });

  it("falls back to the id when the target has no name", () => {
    expect(formatAuditTarget("asset", 297)).toBe("asset #297");
    expect(formatAuditTarget("asset", 297, "")).toBe("asset #297");
  });
});
