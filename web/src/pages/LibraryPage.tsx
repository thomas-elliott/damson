import {
  Boxes,
  FileSearch,
  FolderGit2,
  Inbox,
  Library,
  LogOut,
  Settings,
  UploadCloud,
} from "lucide-react";

import { AssetGrid } from "@/components/AssetGrid";
import { DropZone } from "@/components/DropZone";
import {
  AppShell,
  Badge,
  Breadcrumb,
  BreadcrumbItem,
  Button,
  EmptyState,
  FilterSidebar,
  NavRail,
  NavRailItem,
  NavRailSpacer,
  Section,
  Toolbar,
  ToolbarSpacer,
} from "@/design-system";
import { useAssets, useMe, type Me } from "@/hooks/useDataQueries";

export function LibraryPage() {
  const { data: me, loading: meLoading } = useMe();
  const hasLibrary = !!me?.current_library;
  const { data: assets, loading: assetsLoading, total } = useAssets(hasLibrary);

  if (meLoading) {
    return <LoadingShell />;
  }
  if (!me) return null;

  return (
    <AppShell
      rail={<DamsonRail />}
      sidebar={<LibrarySidebar me={me} />}
      main={
        hasLibrary ? (
          <>
            <Toolbar>
              <Breadcrumb count={`${total} assets`}>
                <BreadcrumbItem current>Library</BreadcrumbItem>
              </Breadcrumb>
              <ToolbarSpacer />
              <Button
                variant="primary"
                leadingIcon={<UploadCloud size={12} />}
              >
                Upload
              </Button>
            </Toolbar>
            <DropZone>
              {assetsLoading ? (
                <div className="flex flex-1 items-center justify-center text-sm-plus text-text-3">
                  Loading assets…
                </div>
              ) : (
                <AssetGrid assets={assets} />
              )}
            </DropZone>
          </>
        ) : (
          <EmptyState
            icon={<Library size={32} />}
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

function LoadingShell() {
  return (
    <div className="grid h-screen place-items-center bg-bg text-sm-plus text-text-3">
      Loading…
    </div>
  );
}

function DamsonRail() {
  return (
    <NavRail brand={<DamsonBrand />}>
      <NavRailItem
        label="Library"
        icon={<Library size={16} />}
        active
      />
      <NavRailItem
        label="Projects (M4)"
        icon={<FolderGit2 size={16} />}
        disabled
      />
      <NavRailItem
        label="Collections (M5)"
        icon={<Boxes size={16} />}
        disabled
      />
      <NavRailItem
        label="Import queue (M7)"
        icon={<Inbox size={16} />}
        disabled
      />
      <NavRailItem
        label="Audit log (M7)"
        icon={<FileSearch size={16} />}
        disabled
      />
      <NavRailSpacer />
      <NavRailItem
        label="Settings (M6)"
        icon={<Settings size={16} />}
        disabled
      />
    </NavRail>
  );
}

function DamsonBrand() {
  return (
    <div className="grid h-[30px] w-[30px] place-items-center rounded-md bg-gradient-to-br from-accent to-accent-2 font-mono text-sm-plus font-bold text-accent-fg">
      D
    </div>
  );
}

function LibrarySidebar({ me }: { me: Me }) {
  const lib = me.current_library;
  return (
    <FilterSidebar
      header={
        <div className="flex items-center gap-2 text-sm-plus">
          <span className="truncate font-medium text-text">
            {lib?.name ?? "No library"}
          </span>
          {lib && (
            <Badge size="sm" mono>
              {lib.role}
            </Badge>
          )}
        </div>
      }
      footer={
        <div className="flex items-center justify-between gap-2 text-xs">
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
      <Section title="Licence" defaultOpen={false}>
        <div className="text-xs-plus text-text-3">
          Licence filters arrive in M3 (licence detection + editor).
        </div>
      </Section>
      <Section title="File type" defaultOpen={false}>
        <div className="text-xs-plus text-text-3">
          Type filters arrive in M5 (search + filters).
        </div>
      </Section>
      <Section title="Tags" defaultOpen={false}>
        <div className="text-xs-plus text-text-3">
          Tags arrive in M3 (tags tree).
        </div>
      </Section>
    </FilterSidebar>
  );
}
