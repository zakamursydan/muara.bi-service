import axios, { AxiosError, type AxiosRequestConfig } from "axios";
import type * as T from "@/types/api";

/**
 * Single entry point for the muara.bi REST API.
 *
 * Paths are relative to `/api/v1`; next.config.ts proxies that prefix to the Go
 * server, so the browser stays same-origin and the API needs no CORS.
 */

export const TOKEN_STORAGE_KEY = "muara.bi.token";

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_STORAGE_KEY);
}

export function setToken(token: string | null) {
  if (typeof window === "undefined") return;
  if (token) window.localStorage.setItem(TOKEN_STORAGE_KEY, token);
  else window.localStorage.removeItem(TOKEN_STORAGE_KEY);
}

type BaseResponse<T> = {
  reqId?: string;
  status: string;
  message: string;
  data?: T;
  error?: unknown;
};

/** Normalised failure: every thrown API error carries a user-safe message. */
export class ApiError extends Error {
  readonly status: number;
  readonly details?: unknown;

  constructor(status: number, message: string, details?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.details = details;
  }
}

export const http = axios.create({
  baseURL: "/api/v1",
  timeout: 45_000,
});

http.interceptors.request.use((config) => {
  const token = getToken();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

function toApiError(err: unknown): ApiError {
  if (axios.isCancel(err)) return new ApiError(0, "Request cancelled");
  if (!(err instanceof AxiosError)) {
    return new ApiError(0, err instanceof Error ? err.message : "Unexpected error");
  }

  const status = err.response?.status ?? 0;
  const body = err.response?.data as BaseResponse<unknown> | undefined;
  const detail = body?.error;
  const message =
    (typeof detail === "string" && detail) ||
    body?.message ||
    (status === 0 ? "Cannot reach the muara.bi API" : err.message);

  return new ApiError(status, message, detail);
}

async function request<T>(run: () => Promise<{ data: BaseResponse<T> }>): Promise<T> {
  try {
    const response = await run();
    if (response.data.data === undefined) {
      throw new ApiError(500, "Malformed API response: missing data envelope");
    }
    return response.data.data;
  } catch (err) {
    if (err instanceof ApiError) throw err;
    throw toApiError(err);
  }
}

/** The envelope omits `data` entirely, so payload-less routes must not require it. */
async function requestVoid(run: () => Promise<{ data: BaseResponse<unknown> }>): Promise<void> {
  try {
    await run();
  } catch (err) {
    throw toApiError(err);
  }
}

const ws = (workspaceId: string) => `/workspaces/${workspaceId}`;

export const api = {
  auth: {
    register: (body: T.RegisterRequest) =>
      request<T.AuthResponse>(() => http.post("/auth/register", body)),
    login: (body: T.LoginRequest) =>
      request<T.AuthResponse>(() => http.post("/auth/login", body)),
    me: () =>
      request<{ user: T.User; workspaces: T.Workspace[] }>(() => http.get("/auth/me")),
  },

  access: {
    /** Optional project scope: pass projectId to get its role, RLS rules and merged grants. */
    get: (workspaceId: string, projectId?: string) =>
      request<T.AccessResponse>(() =>
        http.get(`${ws(workspaceId)}/access`, {
          params: projectId ? { project_id: projectId } : undefined,
        }),
      ),
  },

  projects: {
    list: (workspaceId: string) =>
      request<T.Project[]>(() => http.get(`${ws(workspaceId)}/projects`)),
    create: (workspaceId: string, body: T.CreateProjectRequest) =>
      request<T.Project>(() => http.post(`${ws(workspaceId)}/projects`, body)),
    get: (workspaceId: string, projectId: string) =>
      request<T.Project>(() => http.get(`${ws(workspaceId)}/projects/${projectId}`)),
    members: (workspaceId: string, projectId: string) =>
      request<T.ProjectMember[]>(() =>
        http.get(`${ws(workspaceId)}/projects/${projectId}/members`),
      ),
    assignMember: (
      workspaceId: string,
      projectId: string,
      body: T.AssignProjectMemberRequest,
    ) =>
      request<T.ProjectMember>(() =>
        http.post(`${ws(workspaceId)}/projects/${projectId}/members`, body),
      ),
  },

  connections: {
    list: (workspaceId: string) =>
      request<T.Connection[]>(() => http.get(`${ws(workspaceId)}/connections`)),
    create: (workspaceId: string, body: T.CreateConnectionRequest) =>
      request<T.Connection>(() => http.post(`${ws(workspaceId)}/connections`, body)),
    upsertEnvironment: (
      workspaceId: string,
      connectionId: string,
      body: T.UpsertEnvironmentRequest,
    ) =>
      requestVoid(() =>
        http.post(`${ws(workspaceId)}/connections/${connectionId}/environments`, body),
      ),
    /** Tests the credentials already stored for this environment, so save first. */
    test: (workspaceId: string, connectionId: string, envTag: string) =>
      request<T.TestConnectionResponse>(() =>
        http.post(
          `${ws(workspaceId)}/connections/${connectionId}/environments/${envTag}/test`,
          {},
        ),
      ),
    syncSchema: (workspaceId: string, connectionId: string, envTag: string) =>
      requestVoid(() =>
        http.post(
          `${ws(workspaceId)}/connections/${connectionId}/environments/${envTag}/sync-schema`,
        ),
      ),
  },

  query: {
    /** Force refresh also travels as a header, which the handler trusts over the body. */
    execute: (
      workspaceId: string,
      body: T.ExecuteQueryRequest,
      config?: AxiosRequestConfig,
    ) =>
      request<T.ExecuteQueryResponse>(() =>
        http.post(`${ws(workspaceId)}/query/execute`, body, {
          ...config,
          headers: {
            ...config?.headers,
            ...(body.force_refresh ? { "X-Force-Refresh": "true" } : {}),
          },
        }),
      ),
  },

  semanticModels: {
    list: (workspaceId: string, projectId: string) =>
      request<T.SemanticModel[]>(() =>
        http.get(`${ws(workspaceId)}/projects/${projectId}/semantic-models`),
      ),
    create: (workspaceId: string, projectId: string, body: T.SaveSemanticModelRequest) =>
      request<T.SemanticModel>(() =>
        http.post(`${ws(workspaceId)}/projects/${projectId}/semantic-models`, body),
      ),
    get: (workspaceId: string, modelId: string) =>
      request<T.SemanticModel>(() =>
        http.get(`${ws(workspaceId)}/semantic-models/${modelId}`),
      ),
    update: (workspaceId: string, modelId: string, body: T.SaveSemanticModelRequest) =>
      request<T.SemanticModel>(() =>
        http.put(`${ws(workspaceId)}/semantic-models/${modelId}`, body),
      ),
    remove: (workspaceId: string, modelId: string) =>
      requestVoid(() => http.delete(`${ws(workspaceId)}/semantic-models/${modelId}`)),
  },

  dashboards: {
    list: (workspaceId: string, projectId: string) =>
      request<T.DashboardSummary[]>(() =>
        http.get(`${ws(workspaceId)}/projects/${projectId}/dashboards`),
      ),
    create: (workspaceId: string, projectId: string, body: T.SaveDashboardRequest) =>
      request<T.DashboardSummary>(() =>
        http.post(`${ws(workspaceId)}/projects/${projectId}/dashboards`, body),
      ),
    get: (workspaceId: string, dashboardId: string) =>
      request<T.DashboardDetail>(() => http.get(`${ws(workspaceId)}/dashboards/${dashboardId}`)),
    update: (workspaceId: string, dashboardId: string, body: T.SaveDashboardRequest) =>
      request<T.DashboardSummary>(() =>
        http.put(`${ws(workspaceId)}/dashboards/${dashboardId}`, body),
      ),
  },

  widgets: {
    create: (workspaceId: string, dashboardId: string, body: T.SaveWidgetRequest) =>
      request<T.Widget>(() =>
        http.post(`${ws(workspaceId)}/dashboards/${dashboardId}/widgets`, body),
      ),
    get: (workspaceId: string, widgetId: string) =>
      request<T.Widget>(() => http.get(`${ws(workspaceId)}/widgets/${widgetId}`)),
    update: (workspaceId: string, widgetId: string, body: T.SaveWidgetRequest) =>
      request<T.Widget>(() => http.put(`${ws(workspaceId)}/widgets/${widgetId}`, body)),
    remove: (workspaceId: string, widgetId: string) =>
      requestVoid(() => http.delete(`${ws(workspaceId)}/widgets/${widgetId}`)),
  },

  shares: {
    list: (workspaceId: string, dashboardId: string) =>
      request<T.Share[]>(() =>
        http.get(`${ws(workspaceId)}/dashboards/${dashboardId}/shares`),
      ),
    create: (workspaceId: string, dashboardId: string, body: T.CreateShareRequest) =>
      request<T.Share>(() =>
        http.post(`${ws(workspaceId)}/dashboards/${dashboardId}/shares`, body),
      ),
    revoke: (workspaceId: string, dashboardId: string, shareId: string) =>
      requestVoid(() =>
        http.delete(`${ws(workspaceId)}/dashboards/${dashboardId}/shares/${shareId}`),
      ),
  },

  alerts: {
    list: (workspaceId: string, dashboardId: string) =>
      request<T.Alert[]>(() => http.get(`${ws(workspaceId)}/dashboards/${dashboardId}/alerts`)),
    create: (workspaceId: string, dashboardId: string, body: T.CreateAlertRequest) =>
      request<T.Alert>(() =>
        http.post(`${ws(workspaceId)}/dashboards/${dashboardId}/alerts`, body),
      ),
    update: (
      workspaceId: string,
      dashboardId: string,
      alertId: string,
      body: T.CreateAlertRequest & { is_active: boolean },
    ) =>
      request<T.Alert>(() =>
        http.put(`${ws(workspaceId)}/dashboards/${dashboardId}/alerts/${alertId}`, body),
      ),
    remove: (workspaceId: string, dashboardId: string, alertId: string) =>
      requestVoid(() =>
        http.delete(`${ws(workspaceId)}/dashboards/${dashboardId}/alerts/${alertId}`),
      ),
    test: (workspaceId: string, dashboardId: string, alertId: string) =>
      requestVoid(() =>
        http.post(
          `${ws(workspaceId)}/dashboards/${dashboardId}/alerts/${alertId}/test`,
        ),
      ),
  },

  ai: {
    settings: (workspaceId: string) =>
      request<T.AISettings>(() => http.get(`${ws(workspaceId)}/ai/settings`)),
    updateSettings: (workspaceId: string, body: T.UpdateAISettingsRequest) =>
      request<T.AISettings>(() => http.put(`${ws(workspaceId)}/ai/settings`, body)),
    generateSQL: (workspaceId: string, body: T.GenerateSQLRequest) =>
      request<T.GeneratedSQL>(() => http.post(`${ws(workspaceId)}/ai/generate-sql`, body)),
    generateDashboard: (workspaceId: string, body: T.GenerateDashboardRequest) =>
      request<T.GeneratedDashboard>(() =>
        http.post(`${ws(workspaceId)}/ai/generate-dashboard`, body),
      ),
  },

  public: {
    /** Unauthenticated surface; the share token is the credential. */
    dashboard: (shareToken: string, password?: string) =>
      request<T.PublicDashboard>(() =>
        http.get(`/public/dashboards/${shareToken}`, {
          headers: password ? { "X-Share-Password": password } : undefined,
        }),
      ),
    queryWidget: (
      shareToken: string,
      widgetId: string,
      parameters: Record<string, unknown>,
      password?: string,
    ) =>
      request<T.ExecuteQueryResponse>(() =>
        http.post(
          `/public/dashboards/${shareToken}/widgets/${widgetId}/query`,
          { parameters },
          { headers: password ? { "X-Share-Password": password } : undefined },
        ),
      ),
    /** Signed embed JWT exchanges for the same payload as the public link. */
    embed: (token: string, password?: string) =>
      request<T.PublicDashboard>(() =>
        http.get("/public/embed", {
          params: { token },
          headers: password ? { "X-Share-Password": password } : undefined,
        }),
      ),
  },
};
