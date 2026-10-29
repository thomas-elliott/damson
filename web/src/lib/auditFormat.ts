// Human-friendly formatters for audit entries. The action_type values are
// dotted strings like "asset.update" or "library.member.add"; this module
// keeps the verb mapping in one place so new entries don't need bespoke
// rendering at every call site.
//
// A verb never names its target: formatAuditTarget supplies the noun, so
// "created project" would render as "created project project #1".

export const VERBS: Record<string, string> = {
  "asset.create": "imported",
  "asset.update": "updated",
  "asset.delete": "deleted",
  "asset.merge": "merged",
  "asset.rescan": "rescanned",
  "asset.review": "marked reviewed",
  "asset.unreview": "marked unreviewed",
  "asset.metadata.apply": "applied store metadata to",
  "asset.tag": "tagged",
  "asset.untag": "untagged",
  "asset.licence.set": "set licence on",
  "asset.licence.update": "updated licence on",
  "asset.licence.review": "marked licence reviewed on",
  "asset.reference.create": "added by reference",
  "asset.reference.update": "updated reference",
  "asset.version.create": "imported a version of",
  "asset.version.update": "updated a version of",
  "asset.version.delete": "deleted a version of",
  "asset.version.subset": "created a subset version of",
  "asset.version.reextract": "re-extracted a version of",
  "asset.import.retry": "retried the import of",
  "tag.create": "created",
  "tag.delete": "deleted",
  "project.create": "created",
  "project.update": "updated",
  "project.delete": "deleted",
  "project.pin": "pinned to",
  "project.pin.change": "changed the pinned version in",
  "project.unpin": "unpinned from",
  "collection.create": "created",
  "collection.update": "updated",
  "collection.delete": "deleted",
  "collection.add": "added to",
  "collection.remove": "removed from",
  "token.create": "created",
  "token.revoke": "revoked",
  "library.create": "created",
  "library.update": "updated",
  "library.delete": "deleted",
  "library.settings.update": "updated settings of",
  "library.member.add": "added member",
  "library.member.role.update": "changed member role",
  "library.member.remove": "removed member",
  "admin.user.update": "updated",
  "admin.server_settings.update": "updated settings of",
  "admin.storage.orphan_delete": "deleted orphaned",
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
  api_token: "token",
  storage_object: "storage object",
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
