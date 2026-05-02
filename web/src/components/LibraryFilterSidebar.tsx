import { useSearchParams } from "react-router";
import { ChevronDown, ChevronRight, LogOut } from "lucide-react";

import { FilterSidebar, Section, StatusDot } from "@/design-system";
import { LibraryPicker } from "@/components/LibraryPicker";
import {
  useProjects,
  useTags,
  type AssetFilters,
  type LicenceStatus,
  type Me,
  type TagView,
} from "@/hooks/useDataQueries";

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

export function LibraryFilterSidebar({ me, filters }: { me: Me; filters: AssetFilters }) {
  const [search, setSearch] = useSearchParams();
  const { data: tags } = useTags();
  const { data: projects } = useProjects();
  const lib = me.current_library;

  const setParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(search);
    if (value && value !== "") next.set(key, value);
    else next.delete(key);
    setSearch(next, { replace: true });
  };

  return (
    <FilterSidebar
      header={
        lib ? (
          <LibraryPicker current={lib} />
        ) : (
          <div className="text-ui text-text-3">No library</div>
        )
      }
      footer={
        <div className="flex items-center justify-between gap-2 text-small">
          <span className="truncate font-mono text-text-3" title={me.email}>
            {me.email}
          </span>
          <a
            href="#"
            aria-label="Sign out"
            className="text-text-3 transition-colors duration-std ease hover:text-text"
          >
            <LogOut size={13} />
          </a>
        </div>
      }
    >
      <Section title="Projects" defaultOpen={false}>
        {projects.length === 0 ? (
          <div className="text-small text-text-4">No projects yet</div>
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
          <div className="text-small text-text-4">
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
        "flex w-full items-center justify-between gap-2 rounded-sm px-2 py-1 text-left text-small transition-colors duration-std ease " +
        "hover:bg-surface " +
        (active ? "bg-accent-tint text-text" : "text-text-2")
      }
    >
      <span className="flex min-w-0 items-center gap-2 truncate">
        {icon}
        {label}
      </span>
      {count !== undefined && <span className="text-mono tabular-nums text-text-3">{count}</span>}
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
          "flex items-center gap-1 rounded-sm px-1 py-1 text-small hover:bg-surface " +
          (active ? "bg-accent-tint text-text" : "text-text-2")
        }
        style={{ paddingLeft: 6 + depth * 12 }}
      >
        {hasChildren ? (
          <ChevronDown size={10} className="text-text-3" />
        ) : (
          <ChevronRight size={10} className="text-text-4" />
        )}
        <button
          type="button"
          onClick={() => onPick(tag.path)}
          className="flex flex-1 items-center justify-between gap-2"
          aria-pressed={active}
        >
          <span className="truncate">{label}</span>
          <span className="text-mono tabular-nums text-text-3">{tag.asset_count}</span>
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
