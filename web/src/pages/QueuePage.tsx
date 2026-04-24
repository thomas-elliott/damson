import { useNavigate } from "react-router-dom";
import { Inbox } from "lucide-react";

import { AssetGrid } from "@/components/AssetGrid";
import { Shell } from "@/components/Shell";
import {
  Breadcrumb,
  BreadcrumbItem,
  EmptyState,
  Toolbar,
} from "@/design-system";
import { useAssets } from "@/hooks/useDataQueries";

/**
 * Import queue — filtered library view of `needs_review=true` assets.
 * M7 will add stage-by-stage progress and a richer review flow; for now it
 * surfaces the unreviewed set for the same one-click review that exists in
 * the detail panel.
 */
export function QueuePage() {
  const { data: assets, loading } = useAssets({ needs_review: true });
  const navigate = useNavigate();
  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={`${assets.length}`}>
              <BreadcrumbItem current>Import queue</BreadcrumbItem>
            </Breadcrumb>
          </Toolbar>
          {loading ? (
            <div className="flex flex-1 items-center justify-center text-sm-plus text-text-3">
              Loading queue…
            </div>
          ) : assets.length === 0 ? (
            <EmptyState
              icon={<Inbox size={28} />}
              title="Nothing waiting for review"
              description="Everything that arrived via import has been checked. New unreviewed assets will show up here automatically."
            />
          ) : (
            <AssetGrid assets={assets} onOpen={(id) => navigate(`/library/${id}`)} />
          )}
        </>
      }
    />
  );
}
