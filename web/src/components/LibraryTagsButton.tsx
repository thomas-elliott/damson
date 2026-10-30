import { useSearchParams } from "react-router";
import { Tags } from "lucide-react";

import {
  Badge,
  Button,
  EmptyState,
  ErrorState,
  LoadingState,
  Popover,
  PopoverContent,
  PopoverTrigger,
  TagPicker,
} from "@/design-system";
import { useTags } from "@/hooks/useDataQueries";
import { withPage } from "@/lib/libraryFilters";

/**
 * The toolbar's Tags button: the tag picker in a popover. Ticking a tag
 * filters the library straight away; an asset under any ticked tag matches.
 */
export function LibraryTagsButton({
  selected,
  open,
  onOpenChange,
}: {
  /** The ticked tag paths, from the URL. */
  selected: string[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [search, setSearch] = useSearchParams();
  const setSelected = (paths: string[]) => {
    const next = withPage(search, 1);
    next.delete("tag");
    for (const p of paths) next.append("tag", p);
    setSearch(next, { replace: true });
  };

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>
        <Button
          variant="secondary"
          pressed={open}
          leadingIcon={<Tags size={12} />}
          aria-keyshortcuts="t"
        >
          Tags
          {selected.length > 0 && (
            <Badge tone="accent" shape="pill">
              {selected.length}
            </Badge>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        aria-label="Tags"
        className="w-[760px] max-w-[calc(100vw-1rem)] p-0"
        collisionPadding={8}
      >
        <TagPickerBody
          selected={selected}
          onSelectedChange={setSelected}
          onApply={() => onOpenChange(false)}
        />
      </PopoverContent>
    </Popover>
  );
}

function TagPickerBody({
  selected,
  onSelectedChange,
  onApply,
}: {
  selected: string[];
  onSelectedChange: (paths: string[]) => void;
  onApply: () => void;
}) {
  const { data: tags, loading, failed, reload } = useTags();
  if (loading) return <LoadingState className="h-40" />;
  if (failed) return <ErrorState title="Couldn't load tags" onRetry={reload} />;
  if (tags.length === 0) {
    return (
      <EmptyState
        title="No tags yet"
        description="Tag an asset from its details panel, or fetch its store listing."
      />
    );
  }
  return (
    <TagPicker
      tags={tags.map((t) => ({ path: t.path, count: t.asset_count }))}
      selected={selected}
      onSelectedChange={onSelectedChange}
      onApply={onApply}
    />
  );
}
