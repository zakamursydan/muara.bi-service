import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type * as T from "@/types/api";

/** Deterministic key material: object key order must not change the cache key. */
export function stableStringify(value: unknown): string {
  if (value === null || typeof value !== "object") return JSON.stringify(value ?? null);
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(",")}]`;
  return `{${Object.keys(value as object)
    .sort()
    .map((key) => `${JSON.stringify(key)}:${stableStringify((value as Record<string, unknown>)[key])}`)
    .join(",")}}`;
}

export const queryKeys = {
  access: (wsId: string, projectId?: string) => ["access", wsId, projectId ?? ""] as const,
  projects: (wsId: string) => ["projects", wsId] as const,
  projectMembers: (wsId: string, projectId: string) =>
    ["project-members", wsId, projectId] as const,
  connections: (wsId: string) => ["connections", wsId] as const,
  dashboards: (wsId: string, projectId: string) => ["dashboards", wsId, projectId] as const,
  dashboard: (wsId: string, dashboardId: string) => ["dashboard", wsId, dashboardId] as const,
  semanticModels: (wsId: string, projectId: string) =>
    ["semantic-models", wsId, projectId] as const,
  alerts: (wsId: string, dashboardId: string) => ["alerts", wsId, dashboardId] as const,
  shares: (wsId: string, dashboardId: string) => ["shares", wsId, dashboardId] as const,
  aiSettings: (wsId: string) => ["ai-settings", wsId] as const,
  query: (input: {
    wsId: string;
    envTag: string;
    sql: string;
    parameters: Record<string, unknown>;
    nonce: number;
  }) =>
    [
      "query",
      input.wsId,
      input.envTag,
      input.sql,
      stableStringify(input.parameters),
      input.nonce,
    ] as const,
};

const enabled = (id?: string) => Boolean(id);

export const useProjectsQuery = (workspaceId?: string) =>
  useQuery({
    queryKey: queryKeys.projects(workspaceId ?? ""),
    queryFn: () => api.projects.list(workspaceId as string),
    enabled: enabled(workspaceId),
  });

export const useConnectionsQuery = (workspaceId?: string) =>
  useQuery({
    queryKey: queryKeys.connections(workspaceId ?? ""),
    queryFn: () => api.connections.list(workspaceId as string),
    enabled: enabled(workspaceId),
  });

export const useDashboardsQuery = (workspaceId?: string, projectId?: string) =>
  useQuery({
    queryKey: queryKeys.dashboards(workspaceId ?? "", projectId ?? ""),
    queryFn: () => api.dashboards.list(workspaceId as string, projectId as string),
    enabled: enabled(workspaceId) && enabled(projectId),
  });

export const useDashboardQuery = (workspaceId?: string, dashboardId?: string) =>
  useQuery({
    queryKey: queryKeys.dashboard(workspaceId ?? "", dashboardId ?? ""),
    queryFn: () => api.dashboards.get(workspaceId as string, dashboardId as string),
    enabled: enabled(workspaceId) && enabled(dashboardId),
  });

export const useSemanticModelsQuery = (workspaceId?: string, projectId?: string) =>
  useQuery({
    queryKey: queryKeys.semanticModels(workspaceId ?? "", projectId ?? ""),
    queryFn: () => api.semanticModels.list(workspaceId as string, projectId as string),
    enabled: enabled(workspaceId) && enabled(projectId),
  });

export const useAlertsQuery = (workspaceId?: string, dashboardId?: string) =>
  useQuery({
    queryKey: queryKeys.alerts(workspaceId ?? "", dashboardId ?? ""),
    queryFn: () => api.alerts.list(workspaceId as string, dashboardId as string),
    enabled: enabled(workspaceId) && enabled(dashboardId),
  });

export const useSharesQuery = (workspaceId?: string, dashboardId?: string) =>
  useQuery({
    queryKey: queryKeys.shares(workspaceId ?? "", dashboardId ?? ""),
    queryFn: () => api.shares.list(workspaceId as string, dashboardId as string),
    enabled: enabled(workspaceId) && enabled(dashboardId),
  });

export const useAISettingsQuery = (workspaceId?: string) =>
  useQuery({
    queryKey: queryKeys.aiSettings(workspaceId ?? ""),
    queryFn: () => api.ai.settings(workspaceId as string),
    enabled: enabled(workspaceId),
  });

export const useProjectMembersQuery = (workspaceId?: string, projectId?: string) =>
  useQuery({
    queryKey: queryKeys.projectMembers(workspaceId ?? "", projectId ?? ""),
    queryFn: () => api.projects.members(workspaceId as string, projectId as string),
    enabled: enabled(workspaceId) && enabled(projectId),
  });

/** Environment tags in use across this tenant's connections, for the switcher. */
export function collectEnvTags(connections: T.Connection[] | undefined): T.EnvironmentInfo[] {
  const byTag = new Map<string, T.EnvironmentInfo>();
  for (const connection of connections ?? []) {
    for (const env of connection.environments ?? []) {
      const existing = byTag.get(env.env_tag);
      // Keep the default-tagged, then alphabetically earliest profile as the label source.
      if (!existing || (env.is_default && !existing.is_default)) byTag.set(env.env_tag, env);
    }
  }
  return [...byTag.values()].sort((a, b) => a.env_tag.localeCompare(b.env_tag));
}
