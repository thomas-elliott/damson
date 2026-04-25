import { useEffect, useMemo, useRef } from "react";
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
  type AssetRow,
  type LicenceStatus,
  type Me,
} from "@/hooks/useDataQueries";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";

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

  const searchInputRef = useRef<HTMLInputElement>(null);

  // Per UI §5.3: `/` and `cmd+k` focus the library search; arrow keys move
  // selection in the grid; enter opens the highlighted asset; escape closes
  // the panel. We keep the bindings array stable across renders by reading
  // current state through refs inside the handlers — re-creating the array
  // every time `assets` refetches caused useKeyboardShortcuts to rebind on
  // every paint.
  const stateRef = useRef({ assets, assetId, onOpenAsset, onCloseDetail });
  useEffect(() => {
    stateRef.current = { assets, assetId, onOpenAsset, onCloseDetail };
  });

  const focusSearch = (e: KeyboardEvent) => {
    e.preventDefault();
    searchInputRef.current?.focus();
  };
  const moveSelection = (delta: number) => {
    const s = stateRef.current;
    moveByOffset(s.assets, s.assetId, delta, s.onOpenAsset);
  };
  useKeyboardShortcuts(
    useMemo(
      () => [
        { key: "/", handler: focusSearch },
        { key: "k", meta: true, handler: focusSearch },
        { key: "k", ctrl: true, handler: focusSearch },
        {
          key: "Escape",
          handler: () => {
            const s = stateRef.current;
            if (s.assetId != null) s.onCloseDetail();
          },
        },
        { key: "ArrowRight", handler: (e) => { e.preventDefault(); moveSelection(1); } },
        { key: "ArrowLeft", handler: (e) => { e.preventDefault(); moveSelection(-1); } },
        {
          key: "ArrowDown",
          handler: (e) => {
            e.preventDefault();
            moveSelection(estimateColumnCount());
          },
        },
        {
          key: "ArrowUp",
          handler: (e) => {
            e.preventDefault();
            moveSelection(-estimateColumnCount());
          },
        },
        {
          key: "Enter",
          handler: () => {
            const s = stateRef.current;
            if (s.assetId == null && s.assets[0]) s.onOpenAsset(s.assets[0].id);
          },
        },
      ],
      [],
    ),
  );

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
                  ref={searchInputRef}
                  value={search.get("q") ?? ""}
                  onChange={(e) => onSearchChange(e.currentTarget.value)}
                  placeholder="Search assets ( / )"
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

// moveByOffset shifts the highlighted asset by `delta` and pushes the new id
// through the supplied open handler so the URL routing stays the source of
// truth. Bounded so wrap-around doesn't accidentally jump from the start
// to the end.
function moveByOffset(
  assets: AssetRow[],
  selected: number | null,
  delta: number,
  open: (id: number) => void,
) {
  if (assets.length === 0) return;
  const i = selected != null ? assets.findIndex((a) => a.id === selected) : -1;
  let next = i < 0 ? 0 : i + delta;
  if (next < 0) next = 0;
  if (next >= assets.length) next = assets.length - 1;
  const target = assets[next];
  if (target) open(target.id);
}

// estimateColumnCount infers how many cards fit per row from the grid's
// declared min cell width (180px) plus gap (12px). Cheap heuristic that
// avoids a layout-effect read; close enough for arrow-down semantics.
function estimateColumnCount(): number {
  if (typeof window === "undefined") return 4;
  const cellWithGap = 180 + 12;
  const main = window.document.querySelector("main");
  const width = main?.clientWidth ?? window.innerWidth;
  return Math.max(1, Math.floor(width / cellWithGap));
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
