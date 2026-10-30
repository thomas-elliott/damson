import { useNavigate } from "react-router";
import { Tags } from "lucide-react";

import { Shell } from "@/components/Shell";
import { useBlobUrl } from "@/components/filebrowser/useFileObjectUrl";
import {
  Breadcrumb,
  BreadcrumbItem,
  EmptyState,
  ErrorState,
  LoadingState,
  PageContent,
  TagIndexCard,
  Toolbar,
  tidyAssetName,
  type TagIndexCover,
} from "@/design-system";
import {
  useTagIndex,
  type TagIndexCoverView,
  type TagIndexEntryView,
} from "@/hooks/useDataQueries";

// Top-level tags with nothing under them are the library's own; they gather
// at the end rather than each heading a group of one.
const OWN_TAGS = "Other tags";

/**
 * The tag index: a card per second-level tag, grouped under its top-level
 * store category, each showing covers from inside it. A card opens the
 * library filtered to that tag.
 */
export function TagIndexPage() {
  const { data: entries, loading, failed, reload } = useTagIndex();
  const navigate = useNavigate();
  const groups = groupByTopLevel(entries);

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={loading ? undefined : `${entries.length}`}>
              <BreadcrumbItem current heading>
                Tags
              </BreadcrumbItem>
            </Breadcrumb>
          </Toolbar>
          {loading ? (
            <LoadingState />
          ) : failed ? (
            <ErrorState title="Couldn't load tags" onRetry={reload} />
          ) : entries.length === 0 ? (
            <EmptyState
              icon={<Tags size={28} />}
              title="No tags yet"
              description="Tag an asset from its details panel, or fetch its store listing, and its tags show here."
            />
          ) : (
            <PageContent className="gap-6">
              {groups.map(([group, items]) => (
                <section key={group} aria-labelledby={`tag-group-${group}`}>
                  <h2
                    id={`tag-group-${group}`}
                    className="mb-2.5 text-overline text-subtle-foreground"
                  >
                    {group}
                  </h2>
                  <ul className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-x-3 gap-y-5">
                    {items.map((entry) => (
                      <li key={entry.path}>
                        <TagIndexEntry
                          entry={entry}
                          onOpen={() =>
                            navigate(`/library?${new URLSearchParams({ tag: entry.path })}`)
                          }
                        />
                      </li>
                    ))}
                  </ul>
                </section>
              ))}
            </PageContent>
          )}
        </>
      }
    />
  );
}

function groupByTopLevel(entries: TagIndexEntryView[]): [string, TagIndexEntryView[]][] {
  const groups = new Map<string, TagIndexEntryView[]>();
  for (const e of entries) {
    const slash = e.path.indexOf("/");
    const group = slash < 0 ? OWN_TAGS : e.path.slice(0, slash);
    groups.set(group, [...(groups.get(group) ?? []), e]);
  }
  return [...groups].sort(([a], [b]) =>
    a === OWN_TAGS ? 1 : b === OWN_TAGS ? -1 : a.localeCompare(b),
  );
}

// One card, with its covers' thumbnails fetched behind the bearer token.
function TagIndexEntry({ entry, onOpen }: { entry: TagIndexEntryView; onOpen: () => void }) {
  const [a, b, c] = entry.covers;
  const covers = [useCover(a), useCover(b), useCover(c)].filter(
    (cover): cover is TagIndexCover => cover != null,
  );
  return (
    <TagIndexCard
      path={entry.path}
      count={entry.count}
      topChildren={entry.top_children}
      covers={covers}
      onOpen={onOpen}
    />
  );
}

function useCover(cover: TagIndexCoverView | undefined): TagIndexCover | undefined {
  const imageUrl = useBlobUrl(
    cover?.thumbnail_object_key ? `/api/thumbs/${cover.thumbnail_object_key}` : undefined,
  );
  if (!cover) return undefined;
  return { name: tidyAssetName(cover.name), publisher: cover.publisher, imageUrl };
}
