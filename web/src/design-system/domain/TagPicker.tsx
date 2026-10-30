import { useMemo, useState } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { Button } from "@/design-system/primitives/Button";
import { Checkbox } from "@/design-system/primitives/Checkbox";
import { SearchInput } from "@/design-system/primitives/SearchInput";
import { Tag } from "@/design-system/primitives/Tag";
import { cn } from "@/design-system/utils";

import { leafOf, parentOf } from "./tagPath";

export interface TagPickerTag {
  path: string;
  count: number;
}

export interface TagPickerProps {
  tags: TagPickerTag[];
  /** Ticked tag paths. */
  selected: string[];
  onSelectedChange: (paths: string[]) => void;
  /** Recently used paths, offered as one-click tags above the columns. */
  recent?: string[];
  onApply?: () => void;
  className?: string;
}

const MAX_COLUMNS = 4;
const MAX_RESULTS = 50;

/**
 * Picks tags from a tree of store categories, shown as columns that step
 * through a path like a file browser's column view. The first level is too
 * broad to filter by, so only deeper levels can be ticked. Typing in the
 * filter swaps the columns for a flat list of matching paths. Below 600px it
 * shows two columns, and a column's heading steps back up a level.
 */
export function TagPicker({
  tags,
  selected,
  onSelectedChange,
  recent = [],
  onApply,
  className,
}: TagPickerProps) {
  // Every prefix of every path is a node, so a tag whose parent was never
  // created as a tag of its own still has somewhere to sit.
  const counts = useMemo(() => {
    const m = new Map<string, number | undefined>();
    for (const t of tags) {
      const steps = t.path.split("/");
      for (let i = 1; i < steps.length; i++) {
        const prefix = steps.slice(0, i).join("/");
        if (!m.has(prefix)) m.set(prefix, undefined);
      }
      m.set(t.path, t.count);
    }
    return m;
  }, [tags]);

  const childrenOf = useMemo(() => {
    const m = new Map<string | null, string[]>();
    for (const path of counts.keys()) {
      const parent = parentOf(path);
      m.set(parent, [...(m.get(parent) ?? []), path]);
    }
    for (const list of m.values()) list.sort((a, b) => leafOf(a).localeCompare(leafOf(b)));
    return m;
  }, [counts]);

  // Open on the first ticked tag's column, so the picker shows where you are.
  const [openPath, setOpenPath] = useState<string | null>(() =>
    selected[0] ? parentOf(selected[0]) : null,
  );
  const [query, setQuery] = useState("");

  const ticked = new Set(selected);
  const toggle = (path: string) =>
    onSelectedChange(ticked.has(path) ? selected.filter((p) => p !== path) : [...selected, path]);

  // The open path and each of its ancestors are the columns' parents.
  const parents: (string | null)[] = [null];
  if (openPath) {
    const steps = openPath.split("/");
    for (let i = 1; i <= steps.length; i++) parents.push(steps.slice(0, i).join("/"));
  }
  const visibleParents = parents.slice(-MAX_COLUMNS);

  const q = query.trim().toLowerCase();
  const matches = q
    ? [...counts.keys()].filter((p) => p.includes("/") && p.toLowerCase().includes(q))
    : [];
  const results = matches.slice(0, MAX_RESULTS);

  const row = (path: string, opts: { tickable: boolean; showPath?: boolean }) => {
    const isTicked = ticked.has(path);
    const onPath = openPath === path || (openPath?.startsWith(path + "/") ?? false);
    const hasChildren = childrenOf.has(path);
    const count = counts.get(path);
    return (
      <li
        key={path}
        className={cn(
          "flex items-center gap-2 rounded-md px-2 text-ui text-muted-foreground",
          onPath && "bg-control text-foreground",
          isTicked && "bg-primary text-primary-foreground",
        )}
      >
        {opts.tickable && (
          <Checkbox
            checked={isTicked}
            onCheckedChange={() => toggle(path)}
            aria-label={`Select ${path}`}
            className={cn(
              isTicked &&
                "data-[state=checked]:bg-primary-foreground data-[state=checked]:text-primary",
            )}
          />
        )}
        <button
          type="button"
          onClick={() => {
            setOpenPath(hasChildren ? path : parentOf(path));
            setQuery("");
          }}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-sm py-1.5 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
        >
          <span className="truncate">
            {opts.showPath ? path.split("/").join(" / ") : leafOf(path)}
          </span>
          {count !== undefined && <span className="ml-auto text-mono opacity-80">{count}</span>}
          {hasChildren && <ChevronRight size={12} aria-hidden className="shrink-0 opacity-70" />}
        </button>
      </li>
    );
  };

  return (
    <div className={cn("@container flex w-[760px] max-w-full flex-col text-ui", className)}>
      <div className="border-b border-border p-2">
        <SearchInput
          kbd="T"
          aria-label="Filter tags"
          placeholder="Filter tags…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>
      {recent.length > 0 && !q && (
        <div className="flex flex-wrap items-center gap-1.5 border-b border-border px-3 py-2">
          <span className="mr-1 text-overline text-subtle-foreground">Recent</span>
          {recent.map((path) => (
            <Tag key={path} onClick={() => toggle(path)} aria-pressed={ticked.has(path)}>
              {path}
            </Tag>
          ))}
        </div>
      )}
      {q ? (
        <ul aria-label="Matching tags" className="flex h-64 flex-col gap-px overflow-y-auto p-1.5">
          {results.length === 0 ? (
            <li className="px-2 py-1.5 text-subtle-foreground">No tags match “{query.trim()}”.</li>
          ) : (
            results.map((path) => row(path, { tickable: true, showPath: true }))
          )}
          {matches.length > results.length && (
            <li className="px-2 py-1.5 text-subtle-foreground">
              Showing {results.length} of {matches.length} matches. Type more to narrow them down.
            </li>
          )}
        </ul>
      ) : (
        // A narrow picker shows the last two columns; each heading steps up a
        // level, so a column out of view is one click away.
        <div className="grid h-64 grid-cols-2 @[600px]:grid-cols-4">
          {visibleParents.map((parent, i) => (
            <div
              key={parent ?? ""}
              className={cn(
                "flex min-w-0 flex-col border-r border-border p-1.5 last:border-r-0",
                i < visibleParents.length - 2 && "hidden @[600px]:flex",
              )}
            >
              {parent ? (
                <button
                  type="button"
                  onClick={() => setOpenPath(parentOf(parent))}
                  className="flex items-center gap-0.5 rounded-sm px-2 pb-1.5 pt-1 text-left text-overline text-subtle-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <ChevronLeft size={12} aria-hidden className="shrink-0" />
                  <span className="truncate">
                    <span className="sr-only">Up a level from </span>
                    {leafOf(parent)} /
                  </span>
                </button>
              ) : (
                <span className="px-2 pb-1.5 pt-1 text-overline text-subtle-foreground">
                  Top level
                </span>
              )}
              <ul
                aria-label={parent ? `Tags in ${parent}` : "Top-level tags"}
                className="flex flex-col gap-px overflow-y-auto"
              >
                {(childrenOf.get(parent) ?? []).map((path) =>
                  row(path, { tickable: parent !== null }),
                )}
              </ul>
            </div>
          ))}
        </div>
      )}
      <div className="flex items-center gap-2 border-t border-border px-3 py-2">
        <span className="text-mono text-subtle-foreground">
          {selected.length === 1 ? "1 tag ticked" : `${selected.length} tags ticked`}
        </span>
        <span className="flex-1" />
        <Button
          size="sm"
          variant="ghost"
          onClick={() => onSelectedChange([])}
          disabled={selected.length === 0}
        >
          Clear
        </Button>
        {onApply && (
          <Button size="sm" onClick={onApply}>
            Show results
          </Button>
        )}
      </div>
    </div>
  );
}
