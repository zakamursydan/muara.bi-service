"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { queryKeys } from "@/lib/queries";
import type * as T from "@/types/api";

function useWrite() {
  const client = useQueryClient();
  return {
    /** Invalidate only the keys a write can affect, then toast the outcome. */
    done: (success: string, keys: readonly (readonly unknown[])[]) => {
      for (const key of keys) void client.invalidateQueries({ queryKey: [...key] });
      toast.success(success);
    },
    fail: (message: string) => (err: unknown) =>
      toast.error(err instanceof Error ? err.message : message),
  };
}

/** 12-column grid, two widgets per row, used when the AI builds a canvas. */
export function autoLayout(ids: string[], w = 6, h = 6): T.LayoutItem[] {
  return ids.map((i, index) => ({
    i,
    x: (index % 2) * w,
    y: Math.floor(index / 2) * h,
    w,
    h,
  }));
}

export function useCreateProject(workspaceId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: (body: T.CreateProjectRequest) => api.projects.create(workspaceId, body),
    onSuccess: () => done("Project dibuat", [queryKeys.projects(workspaceId)]),
    onError: fail("Project gagal dibuat"),
  });
}

export type CreateDashboardInput = {
  projectId: string;
  body: T.SaveDashboardRequest;
  /** Attached before the canvas opens; used by the AI and template paths. */
  widgets?: T.SaveWidgetRequest[];
};

export function useCreateDashboard(workspaceId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: async ({ projectId, body, widgets }: CreateDashboardInput) => {
      const dashboard = await api.dashboards.create(workspaceId, projectId, body);
      if (!widgets?.length) return dashboard;

      const created: T.Widget[] = [];
      for (const widget of widgets) {
        created.push(await api.widgets.create(workspaceId, dashboard.id, widget));
      }
      const saved = await api.dashboards.get(workspaceId, dashboard.id);
      // The layout can only be written once the widgets have real ids.
      return api.dashboards.update(workspaceId, dashboard.id, {
        title: saved.title,
        description: saved.description,
        global_filters: saved.global_filters,
        cache_ttl_seconds: saved.cache_ttl_seconds,
        layout_config: autoLayout(created.map((widget) => widget.id)),
      });
    },
    onSuccess: (dashboard) =>
      done("Dashboard dibuat", [
        queryKeys.dashboards(workspaceId, dashboard.project_id),
        queryKeys.dashboard(workspaceId, dashboard.id),
      ]),
    onError: fail("Dashboard gagal dibuat"),
  });
}

export function useSaveDashboard(workspaceId: string, dashboardId: string) {
  const { fail } = useWrite();
  return useMutation({
    mutationFn: (body: T.SaveDashboardRequest) =>
      api.dashboards.update(workspaceId, dashboardId, body),
    onError: fail("Autosave gagal disimpan"),
  });
}

export function useSaveWidget(workspaceId: string, dashboardId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: ({ id, body }: { id?: string; body: T.SaveWidgetRequest }) =>
      id ? api.widgets.update(workspaceId, id, body) : api.widgets.create(workspaceId, dashboardId, body),
    onSuccess: () => done("Widget tersimpan", [queryKeys.dashboard(workspaceId, dashboardId)]),
    onError: fail("Widget gagal disimpan"),
  });
}

export function useDeleteWidget(workspaceId: string, dashboardId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: (widgetId: string) => api.widgets.remove(workspaceId, widgetId),
    onSuccess: () => done("Widget dihapus", [queryKeys.dashboard(workspaceId, dashboardId)]),
    onError: fail("Widget gagal dihapus"),
  });
}

export function useSaveSemanticModel(workspaceId: string, projectId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: ({ id, body }: { id?: string; body: T.SaveSemanticModelRequest }) =>
      id
        ? api.semanticModels.update(workspaceId, id, body)
        : api.semanticModels.create(workspaceId, projectId, body),
    onSuccess: () => done("Semantic model tersimpan", [queryKeys.semanticModels(workspaceId, projectId)]),
    onError: fail("Model gagal disimpan"),
  });
}

export function useDeleteSemanticModel(workspaceId: string, projectId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: (modelId: string) => api.semanticModels.remove(workspaceId, modelId),
    onSuccess: () => done("Model dihapus", [queryKeys.semanticModels(workspaceId, projectId)]),
    onError: fail("Model gagal dihapus"),
  });
}

export function useCreateConnection(workspaceId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: (body: T.CreateConnectionRequest) => api.connections.create(workspaceId, body),
    onSuccess: () => done("Sumber data dibuat", [queryKeys.connections(workspaceId)]),
    onError: fail("Sumber data gagal dibuat"),
  });
}

export function useUpsertEnvironment(workspaceId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: ({ connectionId, body }: { connectionId: string; body: T.UpsertEnvironmentRequest }) =>
      api.connections.upsertEnvironment(workspaceId, connectionId, body),
    onSuccess: () => done("Environment tersimpan", [queryKeys.connections(workspaceId)]),
    onError: fail("Environment gagal disimpan"),
  });
}

export function useTestConnection(workspaceId: string) {
  return useMutation({
    mutationFn: ({
      connectionId,
      envTag,
    }: {
      connectionId: string;
      envTag: string;
    }) => api.connections.test(workspaceId, connectionId, envTag),
  });
}

export function useSyncSchema(workspaceId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: ({ connectionId, envTag }: { connectionId: string; envTag: string }) =>
      api.connections.syncSchema(workspaceId, connectionId, envTag),
    onSuccess: () => done("Sinkronisasi schema dikirim ke worker", [queryKeys.connections(workspaceId)]),
    onError: fail("Sinkronisasi gagal"),
  });
}

export function useCreateShare(workspaceId: string, dashboardId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: (body: T.CreateShareRequest) => api.shares.create(workspaceId, dashboardId, body),
    onSuccess: () => done("Share aktif", [queryKeys.shares(workspaceId, dashboardId)]),
    onError: fail("Share gagal dibuat"),
  });
}

export function useRevokeShare(workspaceId: string, dashboardId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: (shareId: string) => api.shares.revoke(workspaceId, dashboardId, shareId),
    onSuccess: () => done("Share dicabut", [queryKeys.shares(workspaceId, dashboardId)]),
    onError: fail("Share gagal dicabut"),
  });
}

export function useSaveAlert(workspaceId: string, dashboardId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id?: string;
      body: T.CreateAlertRequest & { is_active?: boolean };
    }) =>
      id
        ? api.alerts.update(workspaceId, dashboardId, id, { ...body, is_active: body.is_active ?? true })
        : api.alerts.create(workspaceId, dashboardId, body),
    onSuccess: () => done("Alert tersimpan", [queryKeys.alerts(workspaceId, dashboardId)]),
    onError: fail("Alert gagal disimpan"),
  });
}

export function useDeleteAlert(workspaceId: string, dashboardId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: (alertId: string) => api.alerts.remove(workspaceId, dashboardId, alertId),
    onSuccess: () => done("Alert dihapus", [queryKeys.alerts(workspaceId, dashboardId)]),
    onError: fail("Alert gagal dihapus"),
  });
}

export function useTestAlert(workspaceId: string, dashboardId: string) {
  return useMutation({
    mutationFn: (alertId: string) => api.alerts.test(workspaceId, dashboardId, alertId),
    onSuccess: () => toast.success("Notifikasi uji dikirim"),
    onError: (err: unknown) =>
      toast.error(err instanceof Error ? err.message : "Notifikasi uji gagal"),
  });
}

export function useGenerateSQL(workspaceId: string) {
  return useMutation({
    mutationFn: (body: T.GenerateSQLRequest) => api.ai.generateSQL(workspaceId, body),
  });
}

export function useGenerateDashboard(workspaceId: string) {
  return useMutation({
    mutationFn: (body: T.GenerateDashboardRequest) => api.ai.generateDashboard(workspaceId, body),
  });
}

export function useUpdateAISettings(workspaceId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: (body: T.UpdateAISettingsRequest) => api.ai.updateSettings(workspaceId, body),
    onSuccess: () => done("Konfigurasi AI tersimpan", [queryKeys.aiSettings(workspaceId)]),
    onError: fail("Konfigurasi AI gagal disimpan"),
  });
}

export function useAssignMember(workspaceId: string, projectId: string) {
  const { done, fail } = useWrite();
  return useMutation({
    mutationFn: (body: T.AssignProjectMemberRequest) =>
      api.projects.assignMember(workspaceId, projectId, body),
    onSuccess: (member) =>
      done("Anggota tersimpan", [queryKeys.projectMembers(workspaceId, member.project_id)]),
    onError: fail("Anggota gagal disimpan"),
  });
}
