// Human-friendly formatters for audit entries. The action_type values are
// dotted strings like "asset.update" or "library.member.add"; this module
// keeps the verb mapping in one place so new entries don't need bespoke
// rendering at every call site.

export const VERBS: Record<string, string> = {
  "asset.create": "imported",
  "asset.update": "updated",
  "asset.delete": "deleted",
  "asset.review": "marked reviewed",
  "asset.tag": "tagged",
  "asset.untag": "untagged",
  "asset.licence.set": "set licence on",
  "asset.licence.update": "updated licence on",
  "asset.licence.review": "marked licence reviewed on",
  "asset.version.create": "imported a version of",
  "asset.version.subset": "created a subset version of",
  "tag.create": "created tag",
  "tag.delete": "deleted tag",
  "project.create": "created project",
  "project.update": "updated project",
  "project.delete": "deleted project",
  "project.pin": "pinned to project",
  "project.pin.upgrade": "upgraded pin in project",
  "project.unpin": "unpinned from project",
  "collection.create": "created collection",
  "collection.update": "updated collection",
  "collection.delete": "deleted collection",
  "collection.add": "added to collection",
  "collection.remove": "removed from collection",
  "library.update": "updated library",
  "library.settings.update": "updated library settings",
  "library.member.add": "added member",
  "library.member.role.update": "changed member role",
  "library.member.remove": "removed member",
  "admin.user.update": "updated user",
  "admin.server_settings.update": "updated server settings",
};

export function formatAuditAction(actionType: string): string {
  return VERBS[actionType] ?? actionType;
}

export function formatAuditActor(displayName?: string, email?: string): string {
  if (displayName && displayName !== email) return displayName;
  if (email) return email;
  return "system";
}

const TARGET_LABELS: Record<string, string> = {
  asset: "asset",
  project: "project",
  collection: "collection",
  tag: "tag",
  library: "library",
  user: "user",
  server: "server",
};

export function formatAuditTarget(targetType: string, targetId?: number): string {
  const label = TARGET_LABELS[targetType] ?? targetType;
  return targetId ? `${label} #${targetId}` : label;
}

const TS = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

export function formatAuditTimestamp(iso: string): string {
  try {
    return TS.format(new Date(iso));
  } catch {
    return iso;
  }
}
