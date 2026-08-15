import { useSearchParams } from "react-router";
import { ChevronDown, ChevronRight } from "lucide-react";

import { FilterSidebar, Section, StatusDot } from "@/design-system";
import {
  useAssetFacets,
  useProjects,
  useTags,
  type AssetFilters,
  type LicenceStatus,
  type TagView,
} from "@/hooks/useDataQueries";
import { storeLabel } from "@/lib/stores";

const LICENCE_STATUSES: { value: LicenceStatus; label: string }[] = [
  { value: "ok", label: "Compliant" },
  { value: "warn", label: "Review" },
  { value: "err", label: "Violation" },
  { value: "unk", label: "Unknown" },
];

const PACKAGE_KINDS: { value: string; label: string }[] = [
  { value: "unitypackage", label: "Unity package" },
  { value: "loose", label: "Loose file" },
  { value: "zip", label: "Zip archive" },
];

// Boundaries resolve when clicked, so a long-lived session never pins a stale
// date.
const RELEASED_AFTER_PRESETS: { years: number; label: string }[] = [
  { years: 1, label: "Last year" },
  { years: 2, label: "Last 2 years" },
  { years: 5, label: "Last 5 years" },
];

function yearsAgoISODate(years: number): string {
  const d = new Date();
  d.setFullYear(d.getFullYear() - years);
  return d.toISOString().slice(0, 10);
}

export function LibraryFilterSidebar({ filters }: { filters: AssetFilters }) {
  const [search, setSearch] = useSearchParams();
  const { data: tags } = useTags();
  const { data: projects } = useProjects();
  const { data: facets } = useAssetFacets();

  const setParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(search);
    if (value && value !== "") next.set(key, value);
    else next.delete(key);
    setSearch(next, { replace: true });
  };

  return (
    <FilterSidebar>
      <Section title="Projects" defaultOpen={false}>
        {projects.length === 0 ? (
          <div className="text-label text-faint-foreground">No projects yet</div>
        ) : (
          <ul className="flex flex-col gap-0.5">
            {projects.map((p) => (
              <li key={p.id}>
                <FilterRow
                  label={p.name}
                  count={p.asset_count}
                  active={search.get("project_id") === String(p.id)}
                  onClick={() =>
                    setParam(
                      "project_id",
                      search.get("project_id") === String(p.id) ? null : String(p.id),
                    )
                  }
                />
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section title="Tags" defaultOpen>
        {tags.length === 0 ? (
          <div className="text-label text-faint-foreground">
            No tags yet — add some from an asset detail panel
          </div>
        ) : (
          <TagTree
            tags={tags}
            activePath={filters.tag_path}
            onPick={(p) => setParam("tag", filters.tag_path === p ? null : p)}
          />
        )}
      </Section>

      <Section title="File kind" defaultOpen={false}>
        <ul className="flex flex-col gap-0.5">
          {PACKAGE_KINDS.map((k) => (
            <li key={k.value}>
              <FilterRow
                label={k.label}
                active={filters.package_kind === k.value}
                onClick={() => setParam("kind", filters.package_kind === k.value ? null : k.value)}
              />
            </li>
          ))}
        </ul>
      </Section>

      <Section title="Publisher" defaultOpen={false}>
        {facets.publishers.length === 0 ? (
          <div className="text-label text-faint-foreground">
            No publishers yet — fetch storefront metadata for an asset
          </div>
        ) : (
          <ul className="flex flex-col gap-0.5">
            {facets.publishers.map((p) => (
              <li key={p.value}>
                <FilterRow
                  label={p.value}
                  count={p.count}
                  active={filters.publisher === p.value}
                  onClick={() =>
                    setParam("publisher", filters.publisher === p.value ? null : p.value)
                  }
                />
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section title="Store" defaultOpen={false}>
        {facets.stores.length === 0 ? (
          <div className="text-label text-faint-foreground">No stores yet</div>
        ) : (
          <ul className="flex flex-col gap-0.5">
            {facets.stores.map((st) => (
              <li key={st.value}>
                <FilterRow
                  label={storeLabel(st.value)}
                  count={st.count}
                  active={filters.store === st.value}
                  onClick={() => setParam("store", filters.store === st.value ? null : st.value)}
                />
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section title="Released" defaultOpen={false}>
        <ul className="flex flex-col gap-0.5">
          {RELEASED_AFTER_PRESETS.map((p) => {
            const boundary = yearsAgoISODate(p.years);
            return (
              <li key={p.years}>
                <FilterRow
                  label={p.label}
                  active={filters.released_after === boundary}
                  onClick={() =>
                    setParam(
                      "released_after",
                      filters.released_after === boundary ? null : boundary,
                    )
                  }
                />
              </li>
            );
          })}
        </ul>
      </Section>

      <Section title="Updates" defaultOpen={false}>
        <FilterRow
          label="Updates available"
          active={!!filters.updates_available}
          onClick={() => setParam("updates_available", filters.updates_available ? null : "true")}
        />
      </Section>

      <Section title="Needs review" defaultOpen={false}>
        <FilterRow
          label="Unreviewed only"
          active={!!filters.needs_review}
          onClick={() => setParam("needs_review", filters.needs_review ? null : "true")}
        />
      </Section>

      <Section title="Licence" defaultOpen>
        <ul className="flex flex-col gap-0.5">
          {LICENCE_STATUSES.map((s) => (
            <li key={s.value}>
              <FilterRow
                label={s.label}
                icon={<StatusDot tone={s.value} />}
                active={filters.licence_status === s.value}
                onClick={() =>
                  setParam("licence", filters.licence_status === s.value ? null : s.value)
                }
              />
            </li>
          ))}
        </ul>
      </Section>
    </FilterSidebar>
  );
}

function FilterRow({
  label,
  count,
  icon,
  active,
  onClick,
}: {
  label: string;
  count?: number;
  icon?: React.ReactNode;
  active?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={
        "flex w-full items-center justify-between gap-2 rounded-sm px-2 py-1 text-left text-label transition-colors duration-fast ease-standard " +
        "hover:bg-muted " +
        (active ? "bg-primary-soft text-foreground" : "text-muted-foreground")
      }
    >
      <span className="flex min-w-0 items-center gap-2 truncate">
        {icon}
        {label}
      </span>
      {count !== undefined && (
        <span className="text-mono tabular-nums text-subtle-foreground">{count}</span>
      )}
    </button>
  );
}

// TagTree — flat-to-hierarchical renderer. The `tags` API returns a flat
// list with materialised `path`; we bucket by parent path to render the
// tree client-side.
function TagTree({
  tags,
  activePath,
  onPick,
}: {
  tags: TagView[];
  activePath?: string;
  onPick: (path: string) => void;
}) {
  // Roots are tags whose parent_id is null.
  const roots = tags.filter((t) => !t.parent_id);
  return (
    <ul className="flex flex-col gap-0.5">
      {roots.map((t) => (
        <TagNode
          key={t.id}
          tag={t}
          allTags={tags}
          depth={0}
          activePath={activePath}
          onPick={onPick}
        />
      ))}
    </ul>
  );
}

function TagNode({
  tag,
  allTags,
  depth,
  activePath,
  onPick,
}: {
  tag: TagView;
  allTags: TagView[];
  depth: number;
  activePath?: string;
  onPick: (path: string) => void;
}) {
  const children = allTags.filter((t) => t.parent_id === tag.id);
  const hasChildren = children.length > 0;
  const active = activePath === tag.path;
  const label = tag.path.split("/").pop() ?? tag.path;
  return (
    <li>
      <div
        className={
          "flex items-center gap-1 rounded-sm px-1 py-1 text-label hover:bg-muted " +
          (active ? "bg-primary-soft text-foreground" : "text-muted-foreground")
        }
        style={{ paddingLeft: 6 + depth * 12 }}
      >
        {hasChildren ? (
          <ChevronDown size={10} className="text-subtle-foreground" />
        ) : (
          <ChevronRight size={10} className="text-faint-foreground" />
        )}
        <button
          type="button"
          onClick={() => onPick(tag.path)}
          className="flex flex-1 items-center justify-between gap-2"
          aria-pressed={active}
        >
          <span className="truncate">{label}</span>
          <span className="text-mono tabular-nums text-subtle-foreground">{tag.asset_count}</span>
        </button>
      </div>
      {hasChildren && (
        <ul className="flex flex-col gap-0.5">
          {children.map((c) => (
            <TagNode
              key={c.id}
              tag={c}
              allTags={allTags}
              depth={depth + 1}
              activePath={activePath}
              onPick={onPick}
            />
          ))}
        </ul>
      )}
    </li>
  );
}
