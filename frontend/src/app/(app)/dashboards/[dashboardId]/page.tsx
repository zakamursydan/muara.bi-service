"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { Bell, Check, ChevronLeft, Loader2, Plus, Share2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { CanvasGrid } from "@/components/canvas/grid";
import { FilterBar } from "@/components/canvas/filter-bar";
import { WidgetDrawer } from "@/components/inspector/widget-drawer";
import { DashboardAlertsDialog } from "@/components/dashboard/alerts-dialog";
import { DashboardShareDialog } from "@/components/dashboard/share-dialog";
import { useAutosaveLayout } from "@/hooks/use-autosave-layout";
import { useDashboardQuery } from "@/lib/queries";
import { useCan, useWorkspaceStore } from "@/stores/workspace-store";
import { useCanvasStore } from "@/stores/canvas-store";
import { useEnvStore } from "@/stores/env-store";
import { useProjectStore } from "@/stores/project-store";

export default function DashboardCanvasPage() {
  const params = useParams<{ dashboardId: string }>();
  const dashboardId = params.dashboardId;

  const workspaceId = useWorkspaceStore((state) => state.workspaceId) ?? "";
  const project = useProjectStore((state) => state.project);
  const envTag = useEnvStore((state) => state.envTag) ?? undefined;
  const canRead = useCan("dashboard:read");
  const canWrite = useCan("dashboard:write");

  const dashboardQuery = useDashboardQuery(canRead ? workspaceId : undefined, dashboardId);
  const detail = dashboardQuery.data;

  const load = useCanvasStore((state) => state.load);
  const reset = useCanvasStore((state) => state.reset);
  const widgets = useCanvasStore((state) => state.widgets);
  const layoutDirty = useCanvasStore((state) => state.layoutDirty);
  const startNewWidget = useCanvasStore((state) => state.startNewWidget);

  const [shareOpen, setShareOpen] = useState(false);
  const [alertsOpen, setAlertsOpen] = useState(false);

  useEffect(() => {
    if (detail) load(detail);
  }, [detail, load]);

  // Filters are per-dashboard state; leaving the canvas must not carry them over.
  useEffect(() => () => reset(), [reset]);

  useAutosaveLayout(workspaceId, dashboardId, detail);

  if (dashboardQuery.isLoading) {
    return (
      <div className="flex flex-col gap-3">
        <Skeleton className="h-9 w-72" />
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-[420px] w-full" />
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="flex flex-col items-start gap-2 py-10">
        <p className="text-sm text-muted-foreground">
          Dashboard tidak ditemukan atau Anda tidak punya akses ke project ini.
        </p>
        <Button asChild size="sm" variant="outline">
          <Link href="/catalog">Kembali ke catalog</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Button asChild size="icon" variant="ghost" className="size-8">
          <Link href="/catalog" aria-label="Kembali ke catalog">
            <ChevronLeft className="size-4" />
          </Link>
        </Button>
        <div className="min-w-0">
          <h1 className="truncate text-base font-semibold text-foreground">{detail.title}</h1>
          <p className="truncate text-xs text-muted-foreground">{detail.description}</p>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <Badge variant="secondary" className="gap-1 text-[11px]">
            {layoutDirty ? (
              <>
                <Loader2 className="size-3 animate-spin" />
                menyimpan…
              </>
            ) : (
              <>
                <Check className="size-3" />
                tersimpan
              </>
            )}
          </Badge>
          {canWrite && (
            <>
              <Button size="sm" variant="outline" className="h-8" onClick={() => setAlertsOpen(true)}>
                <Bell className="mr-1 size-3.5" />
                Alert
              </Button>
              <Button size="sm" variant="outline" className="h-8" onClick={() => setShareOpen(true)}>
                <Share2 className="mr-1 size-3.5" />
                Bagikan
              </Button>
              <Button
                size="sm"
                className="h-8"
                onClick={() => startNewWidget("builder")}
              >
                <Plus className="mr-1 size-4" />
                Widget
              </Button>
            </>
          )}
        </div>
      </div>

      {project && envTag ? (
        <>
          <FilterBar filters={detail.global_filters} widgets={widgets} />
          <CanvasGrid
            workspaceId={workspaceId}
            projectId={project.id}
            envTag={envTag}
            canEdit={canWrite}
            onAddWidget={() => startNewWidget("builder")}
          />
          <WidgetDrawer
            workspaceId={workspaceId}
            dashboardId={dashboardId}
            projectId={project.id}
            envTag={envTag}
            canEdit={canWrite}
          />
        </>
      ) : (
        <div className="rounded-xl border border-dashed border-border bg-card/50 px-6 py-16 text-center text-sm text-muted-foreground">
          Pilih project dan environment data di header untuk membuka canvas.
        </div>
      )}

      <DashboardShareDialog
        open={shareOpen}
        onOpenChange={setShareOpen}
        workspaceId={workspaceId}
        dashboardId={dashboardId}
        title={detail.title}
      />
      <DashboardAlertsDialog
        open={alertsOpen}
        onOpenChange={setAlertsOpen}
        workspaceId={workspaceId}
        dashboardId={dashboardId}
        widgets={widgets}
      />
    </div>
  );
}
