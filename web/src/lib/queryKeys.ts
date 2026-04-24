// Centralised query-key factory. Never hand-roll TanStack Query keys in
// components — compose them here and import.
export const queryKeys = {
  authMode: ["authMode"] as const,
  me: ["me"] as const,
  assets: (filters?: Record<string, string | number | boolean>) =>
    filters ? (["assets", filters] as const) : (["assets"] as const),
  asset: (id: number) => ["asset", id] as const,
  assetLicence: (id: number) => ["asset", id, "licence"] as const,
  tags: ["tags"] as const,
  projects: ["projects"] as const,
  project: (id: number) => ["project", id] as const,
  collections: ["collections"] as const,
  collection: (id: number) => ["collection", id] as const,
  librarySettings: ["librarySettings"] as const,
} as const;
