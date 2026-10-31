/**
 * Mirrors the Go DTOs in backend/internal/dto/{request,response}. Field names
 * follow the JSON tags there, so a rename on the backend must be repeated here.
 */

export type Permission =
  | "connection:read"
  | "connection:write"
  | "semantic:read"
  | "semantic:write"
  | "project:read"
  | "project:write"
  | "dashboard:read"
  | "dashboard:write"
  | "query:execute"
  | "member:manage"
  | "share:publish";

export type WorkspaceRole = "OWNER" | "ADMIN" | "ENGINEER" | "EDITOR" | "VIEWER";
export type ProjectRole = "ADMIN" | "EDITOR" | "VIEWER";

export type ChartType = "line" | "bar" | "pie" | "kpi" | "table" | "scatter" | "area";
export type QueryMode = "SYNCED" | "CUSTOM_SQL";
export type ConnectionCategory = "SQL" | "NOSQL" | "FILE" | "CLOUD_DW";
export type ConnectionDriver =
  | "postgres"
  | "mysql"
  | "sqlite"
  | "mongodb"
  | "elasticsearch"
  | "redis"
  | "s3";
export type AlertType = "SCHEDULED_DIGEST" | "THRESHOLD_ALERT";
export type AlertChannel = "TELEGRAM" | "SLACK" | "EMAIL" | "WEBHOOK";
export type AccessMode = "PUBLIC_LINK" | "EMBED_IFRAME";
export type BadgeColor = "blue" | "green" | "red" | "yellow" | "purple" | "orange";
export type AIProvider = "anthropic" | "openai" | "gemini" | "azure_openai" | "ollama";

export type ColumnMeta = { name: string; type: string };

/** react-grid-layout coordinates persisted in dashboards.layout_config. */
export type LayoutItem = {
  i: string;
  x: number;
  y: number;
  w: number;
  h: number;
  minW?: number;
  minH?: number;
  static?: boolean;
};

/** Global filter definition persisted in dashboards.global_filters. */
export type GlobalFilter = {
  key: string;
  label: string;
  type: "text" | "number" | "date" | "select";
  default_value?: unknown;
  options?: string[];
};

/** One aggregated measure of a no-code builder query. */
export type BuilderMetric = {
  column: string;
  aggregation: "COUNT" | "SUM" | "AVG" | "MIN" | "MAX" | "COUNT_DISTINCT" | "NONE";
  alias?: string;
};

export type BuilderFilter = {
  column: string;
  operator: "=" | "!=" | ">" | "<" | ">=" | "<=" | "LIKE" | "ILIKE";
  value?: unknown;
  /** Placeholder name; empty means the server derives `flt_<column>`. */
  parameter?: string;
};

export type WidgetBuilderState = {
  datasource_alias: string;
  table: string;
  dimensions: string[];
  metrics: BuilderMetric[];
  filters: BuilderFilter[];
  limit?: number;
};

/**
 * Visual mapping of a widget's tabular result onto a chart. The backend stores
 * this opaquely, so the shape is owned by the frontend generator.
 */
/**
 * Presentation overrides for a table widget. Every color is an optional hex so
 * an unset field keeps following the app theme, including dark mode.
 */
export type TableStyle = {
  density?: "compact" | "cozy";
  zebra?: boolean;
  sticky_header?: boolean;
  row_numbers?: boolean;
  header_bg?: string;
  header_fg?: string;
  body_bg?: string;
  stripe_bg?: string;
  text_fg?: string;
  accent?: string;
};

export type EchartsConfig = {
  x_field?: string;
  y_fields?: string[];
  stacked?: boolean;
  smooth?: boolean;
  show_percent?: boolean;
  unit?: string;
  decimals?: number;
  /** Per-widget series colors; only literal hex values are ever applied. */
  palette?: string[];
  table?: TableStyle;
};

export type RLSRule = {
  column: string;
  operator: string;
  value_from_user?: string;
  value?: unknown;
};

export type Workspace = {
  id: string;
  name: string;
  slug: string;
  planTier: string;
  createdAt: string;
};

export type User = {
  id: string;
  email: string;
  fullName: string;
  avatarUrl: string | null;
  attributes: Record<string, unknown>;
};

export type AuthResponse = {
  token: string;
  user: User;
  workspaces: Workspace[];
};

export type AccessResponse = {
  workspace_id: string;
  project_id?: string;
  workspace_role: WorkspaceRole | "";
  project_role?: ProjectRole;
  permissions: Permission[];
  rls_columns: string[];
};

export type EnvironmentInfo = {
  id: string;
  env_tag: string;
  display_name: string;
  badge_color: BadgeColor;
  is_default: boolean;
  ssh_tunnel_enabled: boolean;
  host: string;
  port: number;
  database: string;
  user: string;
  cached_schema?: SchemaMap;
  last_synced_at?: string;
};

/** table name -> columns, as produced by the backend schema discovery job. */
export type SchemaMap = Record<string, ColumnMeta[]>;

export type Connection = {
  id: string;
  workspace_id: string;
  alias_name: string;
  category: ConnectionCategory;
  driver_type: ConnectionDriver;
  environments: EnvironmentInfo[];
  created_at: string;
};

export type TestConnectionResponse = {
  success: boolean;
  message: string;
  latency_ms: number;
};

export type Project = {
  id: string;
  workspace_id: string;
  name: string;
  slug: string;
  description: string;
  icon: string;
  created_at: string;
};

export type ProjectMember = {
  id: string;
  project_id: string;
  user_id: string;
  user_email: string;
  user_full_name: string;
  role: ProjectRole;
  rls_rules: RLSRule[];
  created_at: string;
};

export type Widget = {
  id: string;
  dashboard_id: string;
  title: string;
  chart_type: ChartType;
  query_mode: QueryMode;
  builder_state?: WidgetBuilderState;
  raw_sql: string;
  echarts_config: EchartsConfig;
  cache_ttl_seconds: number;
  max_rows: number;
  created_at: string;
};

export type DashboardSummary = {
  id: string;
  workspace_id: string;
  project_id: string;
  title: string;
  description: string;
  layout_config: LayoutItem[];
  global_filters: GlobalFilter[];
  cache_ttl_seconds: number;
  created_by?: string;
  updated_at: string;
};

export type DashboardDetail = DashboardSummary & { widgets: Widget[] };

export type SemanticModel = {
  id: string;
  workspace_id: string;
  project_id: string;
  name: string;
  description: string;
  raw_sql: string;
  cached_schema?: SchemaMap;
  created_at: string;
  updated_at: string;
};

export type ExecuteQueryResponse = {
  columns: ColumnMeta[];
  rows: Record<string, unknown>[];
  row_count: number;
  duration_ms: number;
  cached: boolean;
  env_tag: string;
};

export type Share = {
  id: string;
  dashboard_id: string;
  share_token: string;
  access_mode: AccessMode;
  env_tag: string;
  password_protected: boolean;
  allowed_domains: string[];
  is_active: boolean;
  expires_at?: string;
  created_at: string;
  share_url?: string;
  embed_token?: string;
};

export type PublicWidget = {
  id: string;
  title: string;
  chart_type: ChartType;
  echarts_config: EchartsConfig;
  max_rows: number;
};

export type PublicDashboard = {
  share_token: string;
  access_mode: AccessMode;
  password_required: boolean;
  frame_ancestors: string[];
  expires_at?: string;
  title?: string;
  description?: string;
  layout_config?: LayoutItem[];
  global_filters?: GlobalFilter[];
  widgets?: PublicWidget[];
};

export type Alert = {
  id: string;
  workspace_id: string;
  dashboard_id: string;
  widget_id?: string;
  title: string;
  alert_type: AlertType;
  cron_expression: string;
  condition_operator?: string;
  threshold_value?: number;
  channel: AlertChannel;
  recipient_config: Record<string, unknown>;
  env_tag: string;
  is_active: boolean;
  last_triggered_at?: string;
  created_at: string;
};

export type AISettings = {
  workspace_id: string;
  provider: AIProvider;
  has_api_key: boolean;
  base_url?: string;
  default_model: string;
  fast_model: string;
  temperature: number;
  max_tokens_budget: number;
  custom_instructions?: string;
  schema_masking: string[];
};

export type GeneratedSQL = {
  prompt: string;
  raw_sql: string;
  explanation?: string;
  relevant_tables: string[];
};

export type SuggestedWidget = {
  title: string;
  chart_type: ChartType;
  raw_sql: string;
  echarts_config: Record<string, unknown>;
};

export type GeneratedDashboard = {
  title: string;
  description: string;
  widgets: SuggestedWidget[];
};

/* ---------------------------------- requests ---------------------------------- */

export type RegisterRequest = {
  email: string;
  password: string;
  fullName: string;
  workspaceName: string;
};

export type LoginRequest = {
  email: string;
  password: string;
};

export type CreateConnectionRequest = {
  alias_name: string;
  category: ConnectionCategory;
  driver_type: ConnectionDriver;
};

export type UpsertEnvironmentRequest = {
  env_tag: string;
  display_name: string;
  badge_color: BadgeColor;
  is_default: boolean;
  host: string;
  port: number;
  database: string;
  user: string;
  password: string;
  ssl_mode: string;
  uri: string;
  ssh_tunnel_enabled: boolean;
  ssh_host: string;
  ssh_port: number;
  ssh_user: string;
  ssh_password: string;
  ssh_private_key: string;
};

export type ExecuteQueryRequest = {
  project_id: string;
  env_tag: string;
  raw_sql: string;
  parameters: Record<string, unknown>;
  cache_ttl?: number;
  force_refresh?: boolean;
  max_rows?: number;
};

export type CreateProjectRequest = {
  name: string;
  slug: string;
  description: string;
  icon: string;
};

export type AssignProjectMemberRequest = {
  user_id: string;
  role: ProjectRole;
  rls_rules: RLSRule[];
};

export type SaveDashboardRequest = {
  title: string;
  description: string;
  layout_config: LayoutItem[];
  global_filters: GlobalFilter[];
  cache_ttl_seconds?: number;
};

export type SaveWidgetRequest = {
  title: string;
  chart_type: ChartType;
  query_mode: QueryMode;
  builder_state?: WidgetBuilderState;
  raw_sql: string;
  echarts_config: EchartsConfig;
  cache_ttl_seconds?: number;
  max_rows?: number;
};

export type SaveSemanticModelRequest = {
  name: string;
  description: string;
  raw_sql: string;
};

export type CreateShareRequest = {
  access_mode: AccessMode;
  env_tag: string;
  password?: string;
  allowed_domains?: string[];
  expires_at?: string;
};

export type CreateAlertRequest = {
  widget_id?: string;
  title: string;
  alert_type: AlertType;
  cron_expression: string;
  condition_operator?: string;
  threshold_value?: number;
  channel: AlertChannel;
  recipient_config: Record<string, unknown>;
  env_tag: string;
  is_active?: boolean;
};

export type UpdateAISettingsRequest = {
  provider: AIProvider;
  api_key?: string;
  base_url?: string;
  default_model: string;
  fast_model: string;
  temperature?: number;
  max_tokens_budget?: number;
  custom_instructions?: string;
  schema_masking: string[];
};

export type GenerateSQLRequest = {
  project_id?: string;
  env_tag?: string;
  prompt: string;
  model_preference?: "default" | "fast";
};

export type GenerateDashboardRequest = {
  project_id?: string;
  env_tag?: string;
  prompt: string;
  title?: string;
};
