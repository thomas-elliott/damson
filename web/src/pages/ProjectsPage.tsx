import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Plus } from "lucide-react";

import { Shell } from "@/components/Shell";
import {
  Breadcrumb,
  BreadcrumbItem,
  Button,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  FieldRow,
  Input,
  LicenceBar,
  ProjectCard,
  ProjectCardNew,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea,
  Toolbar,
  ToolbarSpacer,
} from "@/design-system";
import { useProjects } from "@/hooks/useDataQueries";
import { useCreateProject } from "@/hooks/useMutations";

export function ProjectsPage() {
  const { data: projects, loading } = useProjects();
  const navigate = useNavigate();
  const [dialogOpen, setDialogOpen] = useState(false);

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb count={`${projects.length}`}>
              <BreadcrumbItem current>Projects</BreadcrumbItem>
            </Breadcrumb>
            <ToolbarSpacer />
            <Button
              variant="primary"
              leadingIcon={<Plus size={12} />}
              onClick={() => setDialogOpen(true)}
            >
              New project
            </Button>
          </Toolbar>

          {loading ? (
            <div className="flex flex-1 items-center justify-center text-ui text-text-3">
              Loading projects…
            </div>
          ) : projects.length === 0 ? (
            <EmptyState
              icon={<Plus size={28} />}
              title="No projects yet"
              description="A project pins specific asset versions from your library. Add a project when you want to lock asset versions to a particular game build."
              action={
                <Button
                  variant="primary"
                  leadingIcon={<Plus size={12} />}
                  onClick={() => setDialogOpen(true)}
                >
                  New project
                </Button>
              }
            />
          ) : (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-3 p-3.5">
              {projects.map((p) => (
                <ProjectCard
                  key={p.id}
                  name={p.name}
                  glyph={p.glyph || p.name.slice(0, 1).toUpperCase()}
                  color={p.glyph_color || "oklch(0.4 0.14 260)"}
                  engine={p.target_distribution || undefined}
                  onOpen={() => navigate(`/projects/${p.id}`)}
                >
                  <div className="flex items-center justify-between gap-2 text-small text-text-3">
                    <span>
                      <span className="font-mono text-text-2">{p.asset_count}</span> assets
                    </span>
                    {p.updates_available > 0 && (
                      <span className="text-warn">
                        <span className="font-mono">{p.updates_available}</span> updates
                      </span>
                    )}
                  </div>
                  <LicenceBar
                    counts={{ ok: 0, warn: 0, err: 0, unk: p.asset_count }}
                  />
                </ProjectCard>
              ))}
              <ProjectCardNew onClick={() => setDialogOpen(true)} />
            </div>
          )}

          <NewProjectDialog
            open={dialogOpen}
            onOpenChange={setDialogOpen}
          />
        </>
      }
    />
  );
}

function NewProjectDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [dist, setDist] = useState("unspecified");
  const [error, setError] = useState<string | null>(null);
  const create = useCreateProject();
  const navigate = useNavigate();

  const submit = () => {
    setError(null);
    create.mutate(
      { name: name.trim(), description, target_distribution: dist },
      {
        onSuccess: (p) => {
          onOpenChange(false);
          setName("");
          setDescription("");
          setDist("unspecified");
          navigate(`/projects/${p.id}`);
        },
        onError: (err) => setError(err instanceof Error ? err.message : "failed"),
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New project</DialogTitle>
          <DialogDescription>
            Projects pin specific asset versions and track licence compatibility with a distribution target.
          </DialogDescription>
        </DialogHeader>
        <DialogBody>
          <FieldRow label="Name">
            <Input
              value={name}
              autoFocus
              onChange={(e) => setName(e.currentTarget.value)}
              placeholder="Ashfall"
            />
          </FieldRow>
          <FieldRow label="Description">
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.currentTarget.value)}
              placeholder="Short note about the project"
              rows={3}
            />
          </FieldRow>
          <FieldRow
            label="Target distribution"
            description="Used later for licence compliance checks."
          >
            <Select value={dist} onValueChange={setDist}>
              <SelectTrigger aria-label="Target distribution">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="unspecified">Unspecified</SelectItem>
                <SelectItem value="commercial">Commercial</SelectItem>
                <SelectItem value="open-source">Open source</SelectItem>
                <SelectItem value="personal">Personal</SelectItem>
              </SelectContent>
            </Select>
          </FieldRow>
          {error && <div className="text-small text-err">{error}</div>}
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={submit}
            disabled={!name.trim() || create.isPending}
          >
            Create
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

