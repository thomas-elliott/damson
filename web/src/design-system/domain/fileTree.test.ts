import { describe, expect, it } from "vitest";

import {
  buildFileTree,
  countFiles,
  flattenFiles,
  folderKeysToDepth,
  isBrowsable,
  nodesAtPath,
  pathKey,
  typeCounts,
  type FolderNode,
  type ManifestEntry,
} from "./fileTree";

const MANIFEST: ManifestEntry[] = [
  { id: 1, relative_path: "Assets/Prefabs/Castle.prefab", size_bytes: 100 },
  { id: 2, relative_path: "Assets/Prefabs/House.prefab", size_bytes: 200 },
  { id: 3, relative_path: "Assets/Textures/Stone.png", size_bytes: 300 },
  { id: 4, relative_path: "Assets/README.md", size_bytes: 50 },
];

describe("buildFileTree", () => {
  it("groups a flat manifest into a folder/file tree by path", () => {
    const tree = buildFileTree(MANIFEST);
    expect(tree).toHaveLength(1);
    const assets = tree[0] as FolderNode;
    expect(assets.name).toBe("Assets");
    // Folders sort before files: Prefabs, Textures, then README.md.
    expect(assets.children.map((n) => n.name)).toEqual(["Prefabs", "Textures", "README.md"]);
  });

  it("classifies leaves and carries id/objectKey/size", () => {
    const tree = buildFileTree([
      { id: 7, relative_path: "a/b/Hit.wav", size_bytes: 42, object_key: "deadbeef" },
    ]);
    const file = flattenFiles(tree)[0];
    expect(file).toMatchObject({
      id: 7,
      name: "Hit.wav",
      ext: ".wav",
      ftype: "audio",
      size: 42,
      objectKey: "deadbeef",
      dir: ["a", "b"],
    });
  });

  it("is stable regardless of manifest order", () => {
    const a = buildFileTree(MANIFEST);
    const b = buildFileTree([...MANIFEST].reverse());
    expect(JSON.stringify(a)).toBe(JSON.stringify(b));
  });

  it("ignores entries with an empty path", () => {
    expect(buildFileTree([{ relative_path: "", size_bytes: 0 }])).toEqual([]);
  });
});

describe("nodesAtPath", () => {
  const tree = buildFileTree(MANIFEST);
  it("returns the children at a folder path", () => {
    const names = nodesAtPath(tree, ["Assets", "Prefabs"]).map((n) => n.name);
    expect(names).toEqual(["Castle.prefab", "House.prefab"]);
  });
  it("returns the root children for an empty path", () => {
    expect(nodesAtPath(tree, []).map((n) => n.name)).toEqual(["Assets"]);
  });
  it("returns an empty array for an unknown path", () => {
    expect(nodesAtPath(tree, ["Nope"])).toEqual([]);
  });
});

describe("countFiles / flattenFiles / typeCounts", () => {
  const tree = buildFileTree(MANIFEST);
  it("counts files recursively", () => {
    expect(countFiles(tree[0])).toBe(4);
  });
  it("flattens every file with its folder path", () => {
    expect(flattenFiles(tree)).toHaveLength(4);
  });
  it("counts files by display type", () => {
    expect(typeCounts(tree)).toEqual({ prefab: 2, texture: 1, doc: 1 });
  });
});

describe("folderKeysToDepth", () => {
  const DEEP: ManifestEntry[] = [
    { relative_path: "A/B/C/D/leaf.txt", size_bytes: 1 },
    { relative_path: "A/sibling.txt", size_bytes: 1 },
    { relative_path: "Z/only-a-file.txt", size_bytes: 1 },
    { relative_path: "root.txt", size_bytes: 1 },
  ];
  const tree = buildFileTree(DEEP);

  it("expands folders shallower than the given depth, leaving that depth shut", () => {
    expect(folderKeysToDepth(tree, 3).sort()).toEqual(["A", "A/B", "A/B/C", "Z"].sort());
  });

  it("never includes files", () => {
    expect(folderKeysToDepth(tree, 9)).not.toContain("root.txt");
    expect(folderKeysToDepth(tree, 9)).not.toContain("A/sibling.txt");
  });

  it("returns the whole folder set when the depth exceeds the tree", () => {
    expect(folderKeysToDepth(tree, 9).sort()).toEqual(["A", "A/B", "A/B/C", "A/B/C/D", "Z"].sort());
  });

  it("expands nothing at depth 0, and only the roots at depth 1", () => {
    expect(folderKeysToDepth(tree, 0)).toEqual([]);
    expect(folderKeysToDepth(tree, 1).sort()).toEqual(["A", "Z"]);
  });

  it("returns nothing for an empty tree", () => {
    expect(folderKeysToDepth([], 3)).toEqual([]);
  });
});

describe("pathKey / isBrowsable", () => {
  it("joins a folder path and name", () => {
    expect(pathKey(["a", "b"], "c.png")).toBe("a/b/c.png");
    expect(pathKey([], "c.png")).toBe("c.png");
  });
  it("is browsable only with more than one file", () => {
    expect(isBrowsable(2)).toBe(true);
    expect(isBrowsable(1)).toBe(false);
    expect(isBrowsable(0)).toBe(false);
  });
});
