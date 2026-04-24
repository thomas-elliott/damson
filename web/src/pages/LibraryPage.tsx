import { useMemo } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { LogOut, Search, UploadCloud } from "lucide-react";

import { AssetGrid } from "@/components/AssetGrid";
import { AssetDetailPanel } from "@/components/detail/AssetDetailPanel";
import { DropZone, useDropZoneOpen } from "@/components/DropZone";
import { LibraryFilterSidebar } from "@/components/LibraryFilterSidebar";
import { Shell } from "@/components/Shell";
import {
  Breadcrumb,
  BreadcrumbItem,
  Button,
  EmptyState,
  SearchInput,
  Toolbar,
  ToolbarSpacer,
} from "@/design-system";
import { Library as LibraryIcon } from "lucide-react";
import {
  useAssets,
  useMe,
  type AssetFilters,
  type LicenceStatus,
  type Me,
} from "@/hooks/useDataQueries";

export function LibraryPage() {
  const { data: me } = useMe();
  const params = useParams();
  const navigate = useNavigate();
  const [search, setSearch] = useSearchParams();
  const hasLibrary = !!me?.current_library;

  const assetId = params.assetId ? Number(params.assetId) : null;

  const filters: AssetFilters = useMemo(() => {
    const f: AssetFilters = {};
    const q = search.get("q");
    if (q) f.q = q;
    const tag = search.get("tag");
    if (tag) f.tag_path = tag;
    const lic = search.get("licence");
    if (lic) f.licence_status = lic as LicenceStatus;
    const spdx = search.get("spdx");
    if (spdx) f.licence_spdx = spdx;
    const kind = search.get("kind");
    if (kind) f.package_kind = kind;
    const needsReview = search.get("needs_review");
    if (needsReview === "true") f.needs_review = true;
    return f;
  }, [search]);

  const { data: assets, total, loading } = useAssets(filters, hasLibrary);

  const onSearchChange = (next: string) => {
    const params = new URLSearchParams(search);
    if (next) params.set("q", next);
    else params.delete("q");
    setSearch(params, { replace: true });
  };

  const onOpenAsset = (id: number) => {
    navigate(`/library/${id}${search.toString() ? `?${search}` : ""}`);
  };
  const onCloseDetail = () => {
    navigate(`/library${search.toString() ? `?${search}` : ""}`);
  };

  if (!me) return null;

  return (
    <Shell
      sidebar={hasLibrary ? <LibraryFilterSidebar me={me} filters={filters} /> : <SignedOutSidebar me={me} />}
      main={
        hasLibrary ? (
          <DropZone>
            <Toolbar>
              <Breadcrumb count={`${total} assets`}>
                <BreadcrumbItem current>Library</BreadcrumbItem>
              </Breadcrumb>
              <div className="min-w-[220px] max-w-[360px] flex-1">
                <SearchInput
                  value={search.get("q") ?? ""}
                  onChange={(e) => onSearchChange(e.currentTarget.value)}
                  placeholder="Search assets"
                  aria-label="Search assets"
                />
              </div>
              <ToolbarSpacer />
              <UploadButton />
            </Toolbar>
            {loading ? (
              <div className="flex flex-1 items-center justify-center text-sm-plus text-text-3">
                Loading assets…
              </div>
            ) : (
              <AssetGrid assets={assets} onOpen={onOpenAsset} selectedId={assetId} />
            )}
            <AssetDetailPanel assetId={assetId} onClose={onCloseDetail} />
          </DropZone>
        ) : (
          <EmptyState
            icon={<LibraryIcon size={32} />}
            title="No library access yet"
            description={
              <>
                You're signed in as{" "}
                <span className="font-mono text-text-2">{me.email}</span> but
                you haven't been added to a library. Ask a Damson administrator
                to invite you.
              </>
            }
          />
        )
      }
    />
  );
}

function UploadButton() {
  const dz = useDropZoneOpen();
  return (
    <Button
      variant="primary"
      leadingIcon={<UploadCloud size={12} />}
      onClick={() => dz?.open()}
      disabled={!dz}
    >
      Upload
    </Button>
  );
}

function SignedOutSidebar({ me }: { me: Me }) {
  return (
    <div className="flex h-full flex-col justify-end border-r border-line bg-panel p-3">
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="truncate font-mono text-text-3" title={me.email}>
          {me.email}
        </span>
        <a href="#" aria-label="Sign out" className="text-text-3 hover:text-text">
          <LogOut size={13} />
        </a>
      </div>
      <div className="sr-only">
        <Search size={1} />
      </div>
    </div>
  );
}
