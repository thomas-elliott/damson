import { useSearchParams } from "react-router";

import { Button, Chip } from "@/design-system";
import { useProjects, type AssetFilters } from "@/hooks/useDataQueries";
import {
  LICENCE_STATUSES,
  PACKAGE_KINDS,
  RELEASED_AFTER_PRESETS,
  withoutFilters,
  withPage,
  yearsAgoISODate,
} from "@/lib/libraryFilters";
import { storeLabel } from "@/lib/stores";

interface ChipSpec {
  key: string;
  label: string;
  value: string;
  /** The URL param it removes, and the one value to remove for a repeated param. */
  param: string;
  only?: string;
}

/**
 * One removable chip per tag and filter on the library, with a Clear all.
 * Renders nothing when nothing is set; the search term has its own box.
 */
export function ActiveFilterChips({ filters }: { filters: AssetFilters }) {
  const [search, setSearch] = useSearchParams();
  const { data: projects } = useProjects(filters.project_id != null);
  const chips = describeFilters(filters, (id) => projects.find((p) => p.id === id)?.name);
  if (chips.length === 0) return null;

  const remove = (chip: ChipSpec) => {
    const next = withPage(search, 1);
    if (chip.only !== undefined) next.delete(chip.param, chip.only);
    else next.delete(chip.param);
    setSearch(next, { replace: true });
  };

  return (
    <div className="flex shrink-0 flex-wrap items-center gap-1.5 border-b border-border px-3.5 py-1.5">
      <ul aria-label="Active filters" className="flex min-w-0 flex-wrap items-center gap-1.5">
        {chips.map((chip) => (
          <li key={chip.key} className="min-w-0 max-w-full">
            <Chip
              label={chip.label}
              onRemove={() => remove(chip)}
              removeLabel={`Remove ${chip.label.toLowerCase()} filter: ${chip.value}`}
            >
              {chip.value}
            </Chip>
          </li>
        ))}
      </ul>
      <Button
        size="sm"
        variant="ghost"
        onClick={() => setSearch(withoutFilters(search), { replace: true })}
      >
        Clear all
      </Button>
    </div>
  );
}

function describeFilters(
  f: AssetFilters,
  projectName: (id: number) => string | undefined,
): ChipSpec[] {
  const out: ChipSpec[] = [];
  for (const path of f.tag_path ?? []) {
    out.push({
      key: `tag:${path}`,
      label: "Tag",
      value: path.split("/").join(" / "),
      param: "tag",
      only: path,
    });
  }
  const add = (param: string, label: string, value: string) =>
    out.push({ key: param, label, value, param });
  if (f.licence_status) {
    const status = LICENCE_STATUSES.find((s) => s.value === f.licence_status);
    add("licence", "Licence", status?.label ?? f.licence_status);
  }
  if (f.licence_spdx) add("spdx", "Licence type", f.licence_spdx);
  if (f.needs_review) add("needs_review", "Status", "Needs review");
  if (f.updates_available) add("updates_available", "Status", "Updates available");
  if (f.not_downloaded) add("not_downloaded", "Status", "Not downloaded");
  if (f.project_id != null) {
    add("project_id", "Project", projectName(f.project_id) ?? `#${f.project_id}`);
  }
  if (f.publisher) add("publisher", "Publisher", f.publisher);
  if (f.store) add("store", "Store", storeLabel(f.store));
  if (f.released_after) {
    const preset = RELEASED_AFTER_PRESETS.find(
      (p) => yearsAgoISODate(p.years) === f.released_after,
    );
    add("released_after", "Released", preset?.label ?? `Since ${f.released_after}`);
  }
  if (f.package_kind) {
    const kind = PACKAGE_KINDS.find((k) => k.value === f.package_kind);
    add("kind", "File kind", kind?.label ?? f.package_kind);
  }
  return out;
}
