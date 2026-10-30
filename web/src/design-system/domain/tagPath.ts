// Tag paths are slash-separated store categories, such as `3d/character/humanoid`.

/** The path one level up, or null for a top-level tag. */
export function parentOf(path: string) {
  const i = path.lastIndexOf("/");
  return i < 0 ? null : path.slice(0, i);
}

export function leafOf(path: string) {
  return path.slice(path.lastIndexOf("/") + 1);
}
