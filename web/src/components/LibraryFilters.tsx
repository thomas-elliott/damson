import { useSearchParams } from "react-router";
import { SlidersHorizontal } from "lucide-react";

import {
  Badge,
  Button,
  ErrorState,
  FilterRow,
  LoadingState,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Section,
  StatusDot,
} from "@/design-system";
import { useAssetFacets, useProjects, type AssetFilters } from "@/hooks/useDataQueries";
import {
  LICENCE_STATUSES,
  PACKAGE_KINDS,
  RELEASED_AFTER_PRESETS,
  activeFilterCount,
  withPage,
  yearsAgoISODate,
} from "@/lib/libraryFilters";
import { storeLabel } from "@/lib/stores";

/**
 * The toolbar's Filters button: every filter but the tags, in a popover. The
 * badge counts the filters set.
 */
export function LibraryFiltersButton({ filters }: { filters: AssetFilters }) {
  const count = activeFilterCount(filters);
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="secondary" leadingIcon={<SlidersHorizontal size={12} />}>
          Filters
          {count > 0 && (
            <Badge tone="accent" shape="pill">
              {count}
            </Badge>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        aria-label="Filters"
        className="max-h-(--radix-popover-content-available-height) w-80 max-w-[calc(100vw-1rem)] overflow-y-auto p-0"
        collisionPadding={8}
      >
        <LibraryFilterList filters={filters} />
      </PopoverContent>
    </Popover>
  );
}

/** The Filters popover's body: one section per filter. */
export function LibraryFilterList({ filters }: { filters: AssetFilters }) {
  const [search, setSearch] = useSearchParams();
  const projectsQuery = useProjects();
  const facetsQuery = useAssetFacets();
  const { data: projects } = projectsQuery;
  const { data: facets } = facetsQuery;

  // A new filter starts again from the first page.
  const setParam = (key: string, value: string | null) => {
    const next = withPage(search, 1);
    if (value && value !== "") next.set(key, value);
    else next.delete(key);
    setSearch(next, { replace: true });
  };
  const toggle = (key: string, current: string | undefined, value: string) =>
    setParam(key, current === value ? null : value);

  return (
    <div className="flex flex-col">
      <Section title="Licence" defaultOpen>
        <ul className="flex flex-col gap-0.5">
          {LICENCE_STATUSES.map((s) => (
            <li key={s.value}>
              <FilterRow
                label={s.label}
                dot={<StatusDot tone={s.value} />}
                active={filters.licence_status === s.value}
                onClick={() => toggle("licence", filters.licence_status, s.value)}
              />
            </li>
          ))}
        </ul>
      </Section>

      <Section title="Status" defaultOpen>
        <ul className="flex flex-col gap-0.5">
          <li>
            <FilterRow
              label="Needs review"
              active={!!filters.needs_review}
              onClick={() => setParam("needs_review", filters.needs_review ? null : "true")}
            />
          </li>
          <li>
            <FilterRow
              label="Updates available"
              active={!!filters.updates_available}
              onClick={() =>
                setParam("updates_available", filters.updates_available ? null : "true")
              }
            />
          </li>
          <li>
            <FilterRow
              label="Purchased, not downloaded"
              active={!!filters.not_downloaded}
              onClick={() => setParam("not_downloaded", filters.not_downloaded ? null : "true")}
            />
          </li>
        </ul>
      </Section>

      <Section title="Projects" defaultOpen={filters.project_id != null}>
        {loadStatus(projectsQuery, "Couldn't load projects") ??
          (projects.length === 0 ? (
            <div className="text-label text-subtle-foreground">No projects yet</div>
          ) : (
            <ul className="flex flex-col gap-0.5">
              {projects.map((p) => (
                <li key={p.id}>
                  <FilterRow
                    label={p.name}
                    count={p.asset_count}
                    active={filters.project_id === p.id}
                    onClick={() =>
                      toggle("project_id", search.get("project_id") ?? undefined, String(p.id))
                    }
                  />
                </li>
              ))}
            </ul>
          ))}
      </Section>

      <Section title="Publisher" defaultOpen={!!filters.publisher}>
        {loadStatus(facetsQuery, "Couldn't load publishers") ??
          (facets.publishers.length === 0 ? (
            <div className="text-label text-subtle-foreground">
              No publishers yet. Fetch storefront metadata for an asset.
            </div>
          ) : (
            <ul className="flex flex-col gap-0.5">
              {facets.publishers.map((p) => (
                <li key={p.value}>
                  <FilterRow
                    label={p.value}
                    count={p.count}
                    active={filters.publisher === p.value}
                    onClick={() => toggle("publisher", filters.publisher, p.value)}
                  />
                </li>
              ))}
            </ul>
          ))}
      </Section>

      <Section title="Store" defaultOpen={!!filters.store}>
        {loadStatus(facetsQuery, "Couldn't load stores") ??
          (facets.stores.length === 0 ? (
            <div className="text-label text-subtle-foreground">No stores yet</div>
          ) : (
            <ul className="flex flex-col gap-0.5">
              {facets.stores.map((st) => (
                <li key={st.value}>
                  <FilterRow
                    label={storeLabel(st.value)}
                    count={st.count}
                    active={filters.store === st.value}
                    onClick={() => toggle("store", filters.store, st.value)}
                  />
                </li>
              ))}
            </ul>
          ))}
      </Section>

      <Section title="Released" defaultOpen={!!filters.released_after}>
        <ul className="flex flex-col gap-0.5">
          {RELEASED_AFTER_PRESETS.map((p) => {
            const boundary = yearsAgoISODate(p.years);
            return (
              <li key={p.years}>
                <FilterRow
                  label={p.label}
                  active={filters.released_after === boundary}
                  onClick={() => toggle("released_after", filters.released_after, boundary)}
                />
              </li>
            );
          })}
        </ul>
      </Section>

      <Section title="File kind" defaultOpen={!!filters.package_kind}>
        <ul className="flex flex-col gap-0.5">
          {PACKAGE_KINDS.map((k) => (
            <li key={k.value}>
              <FilterRow
                label={k.label}
                active={filters.package_kind === k.value}
                onClick={() => toggle("kind", filters.package_kind, k.value)}
              />
            </li>
          ))}
        </ul>
      </Section>
    </div>
  );
}

// A list's place while it loads or after it failed to load; null once there
// is data, or an empty list, to show.
function loadStatus(
  query: { loading: boolean; failed: boolean; reload: () => unknown },
  errorTitle: string,
) {
  if (query.loading) return <LoadingState variant="inline" />;
  if (query.failed) return <ErrorState title={errorTitle} onRetry={query.reload} />;
  return null;
}
