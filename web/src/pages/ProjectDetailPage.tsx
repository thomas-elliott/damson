import { useState } from "react";
import { useNavigate, useParams } from "react-router";
import { ChevronLeft, Plus, Trash2 } from "lucide-react";

import { isNotFound } from "@/api";
import { AuditList } from "@/components/AuditList";
import { Shell } from "@/components/Shell";
import {
  Badge,
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbSeparator,
  Button,
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  ErrorState,
  FieldRow,
  LicenceBar,
  LicencePill,
  ProjectGlyph,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Toolbar,
  ToolbarSpacer,
  formatBytes,
  useConfirm,
} from "@/design-system";
import { useAssets, useProject, useProjectAudit, type LicenceStatus } from "@/hooks/useDataQueries";
import {
  useDeleteProject,
  usePinAssetToProject,
  useUnpinAssetFromProject,
} from "@/hooks/useMutations";

export function ProjectDetailPage() {
  const { id } = useParams();
  const projectId = id ? Number(id) : null;
  const { data: project, loading, error, failed, reload } = useProject(projectId);
  const navigate = useNavigate();
  const del = useDeleteProject();
  const [pinDialogOpen, setPinDialogOpen] = useState(false);
  const { confirm, dialog: confirmDialog } = useConfirm();

  if (!projectId) return null;

  const counts = summariseLicenceCounts(project?.pins ?? []);

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Button
              variant="ghost"
              size="sm"
              leadingIcon={<ChevronLeft size={12} />}
              onClick={() => navigate("/projects")}
            >
              Projects
            </Button>
            <Breadcrumb>
              <BreadcrumbItem>Projects</BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem current>{project?.name ?? "—"}</BreadcrumbItem>
            </Breadcrumb>
            <ToolbarSpacer />
            <Button
              variant="primary"
              leadingIcon={<Plus size={12} />}
              onClick={() => setPinDialogOpen(true)}
            >
              Pin asset
            </Button>
            <Button
              variant="destructive"
              leadingIcon={<Trash2 size={12} />}
              onClick={async () => {
                if (!project) return;
                const ok = await confirm({
                  title: `Delete project "${project.name}"?`,
                  description: "Pinned assets stay in your library — only the project is removed.",
                  confirmLabel: "Delete project",
                  tone: "destructive",
                });
                if (ok) {
                  del.mutate(project.id, {
                    onSuccess: () => navigate("/projects"),
                  });
                }
              }}
            >
              Delete
            </Button>
          </Toolbar>

          {loading ? (
            <div className="flex flex-1 items-center justify-center text-ui text-subtle-foreground">
              Loading project…
            </div>
          ) : failed && !isNotFound(error) ? (
            <ErrorState title="Couldn't load this project" onRetry={reload} />
          ) : !project ? (
            <EmptyState title="Project not found" description="It may have been deleted." />
          ) : (
            <div className="flex flex-col gap-5 p-4">
              <header className="flex items-start gap-4">
                <ProjectGlyph
                  size="lg"
                  glyph={project.glyph || project.name[0]}
                  color={project.glyph_color || "oklch(0.4 0.14 260)"}
                />
                <div className="flex-1">
                  <h1 className="text-heading-semibold text-foreground">{project.name}</h1>
                  <div className="mt-1 flex items-center gap-2 text-caption">
                    <Badge size="sm" mono>
                      {project.target_distribution}
                    </Badge>
                    <span>· {project.asset_count} assets pinned</span>
                    {project.updates_available > 0 && (
                      <span className="text-warning">
                        · {project.updates_available} updates available
                      </span>
                    )}
                  </div>
                  {project.description && (
                    <p className="mt-2 max-w-prose text-ui text-muted-foreground">
                      {project.description}
                    </p>
                  )}
                </div>
              </header>

              <section className="rounded-md border border-border bg-card p-3">
                <div className="mb-2 text-overline text-subtle-foreground">Licence mix</div>
                <LicenceBar counts={counts} size="md" />
                <div className="mt-1 flex gap-3 text-label text-subtle-foreground">
                  <span>
                    <span className="font-mono text-success">{counts.ok}</span> compliant
                  </span>
                  <span>
                    <span className="font-mono text-warning">{counts.warn}</span> review
                  </span>
                  <span>
                    <span className="font-mono text-destructive">{counts.err}</span> violation
                  </span>
                  <span>
                    <span className="font-mono text-unknown">{counts.unk}</span> unknown
                  </span>
                </div>
              </section>

              <section>
                <h2 className="mb-2 text-ui font-medium text-foreground">Pinned assets</h2>
                {project.pins.length === 0 ? (
                  <EmptyState
                    title="No assets pinned"
                    description="Pin assets from your library to track their versions for this project."
                    action={
                      <Button
                        variant="primary"
                        leadingIcon={<Plus size={12} />}
                        onClick={() => setPinDialogOpen(true)}
                      >
                        Pin asset
                      </Button>
                    }
                  />
                ) : (
                  <table className="w-full border-collapse text-table-row">
                    <thead>
                      <tr className="border-b border-border text-left text-table-header text-subtle-foreground">
                        <th className="py-2 font-normal">Asset</th>
                        <th className="py-2 font-normal">Pinned</th>
                        <th className="py-2 font-normal">Latest</th>
                        <th className="py-2 font-normal">Licence</th>
                        <th className="py-2 text-right font-normal">Size</th>
                        <th className="py-2 font-normal">Status</th>
                        <th className="py-2" />
                      </tr>
                    </thead>
                    <tbody>
                      {project.pins.map((p) => (
                        <ProjectPinRow key={p.pin_id} projectId={project.id} pin={p} />
                      ))}
                    </tbody>
                  </table>
                )}
              </section>

              <section>
                <h2 className="mb-2 text-ui font-medium text-foreground">Recent activity</h2>
                <ProjectAuditPanel projectId={project.id} />
              </section>
            </div>
          )}

          <PinAssetDialog
            open={pinDialogOpen}
            onOpenChange={setPinDialogOpen}
            projectId={projectId}
          />
          {confirmDialog}
        </>
      }
    />
  );
}

function ProjectPinRow({
  projectId,
  pin,
}: {
  projectId: number;
  pin: import("@/hooks/useDataQueries").ProjectPinDetail;
}) {
  const unpin = useUnpinAssetFromProject();
  const repin = usePinAssetToProject();
  return (
    <tr className="border-b border-border/60 hover:bg-muted">
      <td className="py-2 text-foreground">{pin.asset_name}</td>
      <td className="py-2 font-mono text-muted-foreground">
        {pin.is_reference ? "Reference" : pin.version_label}
      </td>
      <td
        className={"py-2 font-mono " + (pin.has_update ? "text-warning" : "text-subtle-foreground")}
      >
        {pin.latest_label ?? "—"} {pin.has_update ? "↑" : ""}
      </td>
      <td className="py-2">
        {pin.licence_spdx ? (
          <LicencePill status={pin.licence_status as LicenceStatus} spdx={pin.licence_spdx} />
        ) : (
          <span className="text-label text-faint-foreground">—</span>
        )}
      </td>
      <td className="py-2 text-right font-mono text-muted-foreground">
        {pin.is_reference ? "—" : formatBytes(pin.size_bytes)}
      </td>
      <td className="py-2 text-label">
        {pin.has_update ? (
          <button
            type="button"
            onClick={() =>
              pin.latest_version_id &&
              repin.mutate({
                projectId,
                asset_id: pin.asset_id,
                version_id: pin.latest_version_id,
              })
            }
            className="text-warning hover:underline"
          >
            Upgrade to {pin.latest_label}
          </button>
        ) : (
          <span className="text-faint-foreground">Up to date</span>
        )}
      </td>
      <td className="py-2 text-right">
        <Button
          size="sm"
          variant="ghost"
          onClick={() => unpin.mutate({ projectId, pinId: pin.pin_id })}
        >
          Unpin
        </Button>
      </td>
    </tr>
  );
}

function ProjectAuditPanel({ projectId }: { projectId: number }) {
  const { data: entries, loading } = useProjectAudit(projectId);
  if (loading) {
    return <div className="text-label text-faint-foreground">Loading…</div>;
  }
  return (
    <AuditList
      entries={entries.slice(0, 25)}
      emptyTitle="No activity yet"
      emptyDescription="Pinning, upgrading, and project edits will show up here."
    />
  );
}

function summariseLicenceCounts(pins: { licence_status: LicenceStatus }[]) {
  const counts = { ok: 0, warn: 0, err: 0, unk: 0 };
  for (const p of pins) {
    const s = p.licence_status ?? "unk";
    counts[s] = (counts[s] ?? 0) + 1;
  }
  return counts;
}

function PinAssetDialog({
  open,
  onOpenChange,
  projectId,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  projectId: number;
}) {
  const { data: assets } = useAssets({}, open);
  const pin = usePinAssetToProject();
  const [selected, setSelected] = useState<string>("");

  const submit = () => {
    if (!selected) return;
    pin.mutate(
      { projectId, asset_id: Number(selected) },
      {
        onSuccess: () => {
          onOpenChange(false);
          setSelected("");
        },
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Pin an asset</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <FieldRow label="Asset">
            <Select value={selected} onValueChange={setSelected}>
              <SelectTrigger aria-label="Asset">
                <SelectValue placeholder="Pick an asset" />
              </SelectTrigger>
              <SelectContent>
                {assets.map((a) => (
                  <SelectItem key={a.id} value={String(a.id)}>
                    {a.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FieldRow>
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit} disabled={!selected || pin.isPending}>
            {assets.find((a) => String(a.id) === selected)?.is_reference
              ? "Pin asset"
              : "Pin current version"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
