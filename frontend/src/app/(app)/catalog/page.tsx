"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { FolderPlus, LayoutGrid, Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { CreateDashboardWizard } from "@/components/dashboard/create-wizard";
import { useDashboardsQuery } from "@/lib/queries";
import { useCreateProject } from "@/lib/mutations";
import { useCan, useWorkspaceStore } from "@/stores/workspace-store";
import { useProjectStore } from "@/stores/project-store";
import type * as T from "@/types/api";

const relativeDate = (value: string) => {
  const diff = Date.now() - new Date(value).getTime();
  const days = Math.floor(diff / 86_400_000);
  if (days <= 0) return "hari ini";
  if (days === 1) return "kemarin";
  if (days < 30) return `${days} hari lalu`;
  return new Date(value).toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" });
};

export default function CatalogPage() {
  const workspaceId = useWorkspaceStore((state) => state.workspaceId) ?? "";
  const project = useProjectStore((state) => state.project);
  const canWrite = useCan("dashboard:write");
  const canCreateProject = useCan("project:write");

  const [search, setSearch] = useState("");
  const [wizardOpen, setWizardOpen] = useState(false);
  const [projectOpen, setProjectOpen] = useState(false);

  const dashboardsQuery = useDashboardsQuery(workspaceId, project?.id);
  const dashboards = useMemo(
    () =>
      (dashboardsQuery.data ?? []).filter((item) =>
        `${item.title} ${item.description}`.toLowerCase().includes(search.toLowerCase()),
      ),
    [dashboardsQuery.data, search],
  );

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-foreground">Catalog dashboard</h1>
          <p className="text-sm text-muted-foreground">
            {project ? project.name : "Belum ada project terpilih"}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              className="h-8 w-56 pl-8"
              placeholder="Cari dashboard…"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>
          {canWrite && (
            <Button size="sm" className="h-8" onClick={() => setWizardOpen(true)} disabled={!project}>
              <Plus className="mr-1 size-4" />
              Dashboard
            </Button>
          )}
        </div>
      </div>

      {!project ? (
        <EmptyProject canWrite={canCreateProject} onCreate={() => setProjectOpen(true)} />
      ) : dashboardsQuery.isLoading ? (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-32 rounded-xl" />
          ))}
        </div>
      ) : dashboards.length === 0 ? (
        <Card className="flex flex-col items-center gap-3 border-dashed px-6 py-12 text-center">
          <LayoutGrid className="size-6 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">
            {search ? `Tidak ada dashboard dengan kata "${search}".` : "Project ini belum punya dashboard."}
          </p>
          {canWrite && !search && (
            <Button size="sm" variant="outline" onClick={() => setWizardOpen(true)}>
              Buat dashboard pertama
            </Button>
          )}
        </Card>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {dashboards.map((dashboard) => (
            <DashboardCard key={dashboard.id} dashboard={dashboard} />
          ))}
        </div>
      )}

      <CreateDashboardWizard open={wizardOpen} onOpenChange={setWizardOpen} />
      <CreateProjectDialog
        workspaceId={workspaceId}
        open={projectOpen}
        onOpenChange={setProjectOpen}
      />
    </div>
  );
}

function DashboardCard({ dashboard }: { dashboard: T.DashboardSummary }) {
  return (
    <Card className="flex flex-col gap-2 p-4 transition-shadow hover:shadow-md">
      <div className="flex items-start justify-between gap-2">
        <h2 className="line-clamp-2 text-sm font-semibold text-foreground">{dashboard.title}</h2>
        <span className="shrink-0 text-[11px] text-muted-foreground">
          {relativeDate(dashboard.updated_at)}
        </span>
      </div>
      <p className="line-clamp-2 min-h-[2.5rem] text-xs text-muted-foreground">
        {dashboard.description || "Tanpa deskripsi"}
      </p>
      <div className="mt-auto flex items-center justify-between">
        <span className="text-[11px] text-muted-foreground">
          Cache {dashboard.cache_ttl_seconds}s · {dashboard.layout_config.length} widget
        </span>
        <Button size="sm" variant="outline" className="h-7 px-2.5 text-xs" asChild>
          <Link href={`/dashboards/${dashboard.id}`}>Buka</Link>
        </Button>
      </div>
    </Card>
  );
}

function EmptyProject({ canWrite, onCreate }: { canWrite: boolean; onCreate: () => void }) {
  return (
    <Card className="flex flex-col items-center gap-3 border-dashed px-6 py-16 text-center">
      <FolderPlus className="size-6 text-muted-foreground" />
      <div>
        <p className="text-sm font-medium text-foreground">Belum ada project</p>
        <p className="text-xs text-muted-foreground">
          Dashboard selalu berada di dalam satu project workspace.
        </p>
      </div>
      {canWrite && (
        <Button size="sm" onClick={onCreate}>
          Buat project
        </Button>
      )}
    </Card>
  );
}

function CreateProjectDialog({
  workspaceId,
  open,
  onOpenChange,
}: {
  workspaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const create = useCreateProject(workspaceId);
  const [name, setName] = useState("");

  const slug = name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Buat project</DialogTitle>
          <DialogDescription>Project memisahkan dashboard, model, dan anggota dalam satu workspace.</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="project-name">Nama project</Label>
            <Input id="project-name" value={name} onChange={(event) => setName(event.target.value)} />
          </div>
          <p className="text-xs text-muted-foreground">Slug: {slug || "…"}</p>
        </div>
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={create.isPending}>
            Batal
          </Button>
          <Button
            disabled={!slug || create.isPending}
            onClick={async () => {
              await create.mutateAsync({ name, slug, description: "", icon: "folder" });
              setName("");
              onOpenChange(false);
            }}
          >
            {create.isPending ? "Menyimpan…" : "Simpan"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
