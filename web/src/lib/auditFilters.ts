// Each value is the server's action_type filter: a comma-separated list where
// an entry ending in ".*" matches every action under that prefix.
export const ACTION_FILTERS = [
  { label: "All", value: "" },
  { label: "Imports", value: "asset.create,asset.version.create,asset.reference.create" },
  {
    label: "Updates",
    value:
      "asset.update,asset.reference.update,asset.version.update,asset.version.subset,asset.metadata.apply,asset.rescan",
  },
  { label: "Reviews", value: "asset.review,asset.unreview" },
  { label: "Licences", value: "asset.licence.*" },
  { label: "Tags", value: "asset.tag,asset.untag,tag.*" },
  { label: "Merges", value: "asset.merge" },
  { label: "Deletions", value: "asset.delete,asset.version.delete" },
  { label: "Collections", value: "collection.*" },
  { label: "Projects", value: "project.*" },
  { label: "Library", value: "library.*,token.*" },
];
