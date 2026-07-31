import { ArrowUpDown } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/design-system";
import type { AssetSort } from "@/hooks/useDataQueries";

const SORTS: { value: AssetSort; label: string }[] = [
  { value: "created_desc", label: "Newest imported" },
  { value: "created_asc", label: "Oldest imported" },
  { value: "released_desc", label: "Newest released" },
  { value: "released_asc", label: "Oldest released" },
  { value: "name_asc", label: "Name" },
];

export const DEFAULT_ASSET_SORT: AssetSort = "created_desc";

export function AssetSortMenu({
  value,
  onChange,
}: {
  value?: AssetSort;
  onChange: (sort: AssetSort) => void;
}) {
  const active = value ?? DEFAULT_ASSET_SORT;
  const label = SORTS.find((s) => s.value === active)?.label ?? SORTS[0].label;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="secondary"
          leadingIcon={<ArrowUpDown size={12} />}
          aria-label={`Sort: ${label}`}
        >
          {label}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        <DropdownMenuRadioGroup value={active} onValueChange={(v) => onChange(v as AssetSort)}>
          {SORTS.map((s) => (
            <DropdownMenuRadioItem key={s.value} value={s.value}>
              {s.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
