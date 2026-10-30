// Centralised query-key factory. Never hand-roll TanStack Query keys in
// components — compose them here and import.
export const queryKeys = {
  authMode: ["authMode"] as const,
  me: ["me"] as const,
  assets: (filters?: Record<string, string | number | boolean | string[]>) =>
    filters ? (["assets", filters] as const) : (["assets"] as const),
  asset: (id: number) => ["asset", id] as const,
  assetLicence: (id: number) => ["asset", id, "licence"] as const,
  assetAudit: (id: number) => ["asset", id, "audit"] as const,
  assetFacets: ["assetFacets"] as const,
  tags: ["tags"] as const,
  tagIndex: ["tags", "index"] as const,
  projects: ["projects"] as const,
  project: (id: number) => ["project", id] as const,
  projectAudit: (id: number) => ["project", id, "audit"] as const,
  collections: ["collections"] as const,
  collection: (id: number) => ["collection", id] as const,
  // /api/settings answers for the current library, so its key names it.
  librarySettings: (libraryId?: number) =>
    libraryId != null ? (["librarySettings", libraryId] as const) : (["librarySettings"] as const),
  audit: (filters?: Record<string, string | number | boolean>) =>
    filters ? (["audit", filters] as const) : (["audit"] as const),
  adminAudit: (filters?: Record<string, string | number | boolean>) =>
    filters ? (["adminAudit", filters] as const) : (["adminAudit"] as const),
  imports: ["imports"] as const,
  previews: ["previews"] as const,
  libraries: ["libraries"] as const,
  libraryMembers: (libraryId: number) => ["libraries", libraryId, "members"] as const,
  apiTokens: ["apiTokens"] as const,
  adminUsers: ["adminUsers"] as const,
  adminSettings: ["adminSettings"] as const,
} as const;
