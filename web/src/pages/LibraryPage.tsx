import { AssetGrid } from "@/components/AssetGrid";
import { DropZone } from "@/components/DropZone";
import { LibraryHeader } from "@/components/LibraryHeader";
import { useAssets, useMe } from "@/hooks/useDataQueries";

export function LibraryPage() {
  const { data: me, loading: meLoading } = useMe();
  const hasLibrary = !!me?.current_library;
  const { data: assets, loading: assetsLoading } = useAssets(hasLibrary);

  if (meLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center text-sm text-muted">
        Loading…
      </div>
    );
  }

  if (!me) return null;

  if (!hasLibrary) {
    return (
      <div className="flex min-h-screen flex-col">
        <LibraryHeader me={me} />
        <div className="flex flex-1 items-center justify-center">
          <div className="max-w-md rounded-md border border-border bg-panel p-6 text-sm">
            <p className="mb-2 font-medium text-foreground">No library access yet</p>
            <p className="text-muted">
              You're signed in as <span className="font-mono">{me.email}</span>, but
              you haven't been added to a library. Ask a Damson administrator to
              invite you.
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col">
      <LibraryHeader me={me} />
      <DropZone>
        {assetsLoading ? (
          <div className="flex flex-1 items-center justify-center text-sm text-muted">
            Loading assets…
          </div>
        ) : (
          <AssetGrid assets={assets} />
        )}
      </DropZone>
    </div>
  );
}
