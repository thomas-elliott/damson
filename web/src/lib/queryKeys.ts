// Centralised query-key factory. Never hand-roll TanStack Query keys in
// components — compose them here and import.
export const queryKeys = {
  authMode: ["authMode"] as const,
  me: ["me"] as const,
  assets: (filters?: Record<string, string | number>) =>
    filters ? (["assets", filters] as const) : (["assets"] as const),
} as const;
