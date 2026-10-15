-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. Users (Authentication, Identity & Dynamic RLS Attributes)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    avatar_url TEXT,
    attributes JSONB DEFAULT '{}',   -- Dynamic RLS attributes: {team, branch_id, territory, department}
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 2. Workspaces (Tenant Isolation Unit)
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    plan_tier VARCHAR(30) DEFAULT 'pro',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 3. Workspace Members & RBAC
CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('OWNER', 'ADMIN', 'ENGINEER', 'EDITOR', 'VIEWER')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (workspace_id, user_id)
);

-- 4. Logical Data Connections (Alias used in SQL & Schema Inspector)
CREATE TABLE IF NOT EXISTS data_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    alias_name VARCHAR(50) NOT NULL, -- e.g. 'db_transaksi' -> used as: SELECT * FROM db_transaksi.orders
    category VARCHAR(20) NOT NULL CHECK (category IN ('SQL', 'NOSQL', 'FILE', 'CLOUD_DW')),
    driver_type VARCHAR(30) NOT NULL, -- 'postgres', 'mysql', 'mongodb', 'elasticsearch', 'redis', 's3'
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (workspace_id, alias_name)
);

-- 5. Dynamic Custom Environment Profiles per Connection (Flexible Pattern 3)
CREATE TABLE IF NOT EXISTS connection_environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID REFERENCES data_connections(id) ON DELETE CASCADE,
    env_tag VARCHAR(50) NOT NULL,    -- Custom tag: 'production', 'staging', 'development', 'uat', 'sandbox', 'qa', 'dr-replica', etc.
    display_name VARCHAR(100),       -- Human-readable label: 'Production Master', 'UAT Server Jakarta', 'Local Sandbox'
    badge_color VARCHAR(30) DEFAULT 'blue', -- UI pill color: 'red', 'yellow', 'green', 'purple', 'blue', 'orange'
    encrypted_config BYTEA NOT NULL, -- AES-256-GCM JSON: {host, port, database, user, password, uri, ssl, ssh_tunnel: {enabled, host, port, user, private_key}}
    cached_schema JSONB,             -- Auto-discovered tables/fields & data types
    is_default BOOLEAN DEFAULT false,
    last_synced_at TIMESTAMPTZ,
    UNIQUE (connection_id, env_tag)
);

-- 6. Projects (Project/App Scoping: e.g. 'Apps Omnichannel', 'Apps Chatbot', 'Retail ERP')
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    icon VARCHAR(30) DEFAULT 'folder',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (workspace_id, slug)
);

-- 7. Dashboards (Grid Layout, Global Filters & Smart Caching)
CREATE TABLE IF NOT EXISTS dashboards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(150) NOT NULL,
    description TEXT,
    layout_config JSONB NOT NULL DEFAULT '[]', -- react-grid-layout coordinates: [{i, x, y, w, h}]
    global_filters JSONB NOT NULL DEFAULT '[]',-- [{key, label, type, default_value}]
    cache_ttl_seconds INT DEFAULT 300,         -- Smart Query Caching TTL (default: 5 mins, 0 for no cache)
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 8. Widgets (Unified Visual Builder, Custom SQL State & Guardrails)
CREATE TABLE IF NOT EXISTS widgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dashboard_id UUID REFERENCES dashboards(id) ON DELETE CASCADE,
    title VARCHAR(150) NOT NULL,
    chart_type VARCHAR(30) NOT NULL, -- 'line', 'bar', 'pie', 'kpi', 'table', 'scatter', 'area'
    query_mode VARCHAR(20) DEFAULT 'SYNCED' CHECK (query_mode IN ('SYNCED', 'CUSTOM_SQL')),
    builder_state JSONB,             -- {datasource_alias, table, dimensions: [], metrics: [], filters: []}
    raw_sql TEXT NOT NULL,           -- DuckDB SQL query (supports {{variable}} placeholders)
    echarts_config JSONB NOT NULL,   -- X/Y mapping, series styling, number formatting
    cache_ttl_seconds INT DEFAULT 300, -- Widget-level cache TTL override
    max_rows INT DEFAULT 10000,        -- Circuit breaker limit for visual rendering
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 9. Export Audit & Background Jobs
CREATE TABLE IF NOT EXISTS export_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    env_used VARCHAR(50) NOT NULL,
    format VARCHAR(10) NOT NULL CHECK (format IN ('CSV', 'JSON', 'XLSX')),
    row_count INT,
    file_url TEXT,
    status VARCHAR(20) DEFAULT 'COMPLETED',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 10. AI Provider & Co-Pilot Settings (Workspace Level)
CREATE TABLE IF NOT EXISTS ai_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    provider VARCHAR(30) NOT NULL DEFAULT 'anthropic', -- 'openai', 'gemini', 'anthropic', 'azure_openai', 'ollama'
    encrypted_api_key BYTEA,                          -- AES-256-GCM encrypted credentials
    base_url TEXT,                                    -- custom endpoint for Ollama / Azure / Local LLM
    default_model VARCHAR(50) NOT NULL DEFAULT 'claude-3-5-sonnet',
    fast_model VARCHAR(50) DEFAULT 'gemini-1.5-flash',
    temperature NUMERIC(2,1) DEFAULT 0.2,
    max_tokens_budget INT DEFAULT 10000000,           -- monthly token spending cap
    custom_instructions TEXT,                         -- Business glossary & domain rules
    schema_masking JSONB DEFAULT '[]',                -- Sensitive columns to exclude from prompts (PII)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (workspace_id)
);

-- 11. Project Members & Granular RBAC / Row-Level Security (RLS)
CREATE TABLE IF NOT EXISTS project_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('ADMIN', 'EDITOR', 'VIEWER')),
    rls_rules JSONB DEFAULT '[]', -- Row-level rules: [{"column": "branch_id", "operator": "=", "value_from_user": "branch_id"}]
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (project_id, user_id)
);

-- 12. Reusable Semantic Models / Virtual Views (Saved Datasets)
CREATE TABLE IF NOT EXISTS semantic_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,      -- Virtual view name: e.g. 'dataset_performa_agen'
    description TEXT,
    raw_sql TEXT NOT NULL,           -- Base SQL query executed in DuckDB
    cached_schema JSONB,             -- Auto-inferred columns & types
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (workspace_id, name)
);

-- 13. Scheduled Reports & Threshold Alerts
CREATE TABLE IF NOT EXISTS dashboard_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    dashboard_id UUID REFERENCES dashboards(id) ON DELETE CASCADE,
    widget_id UUID REFERENCES widgets(id) ON DELETE SET NULL, -- NULL if whole dashboard digest
    title VARCHAR(150) NOT NULL,
    alert_type VARCHAR(20) NOT NULL CHECK (alert_type IN ('SCHEDULED_DIGEST', 'THRESHOLD_ALERT')),
    cron_expression VARCHAR(50) NOT NULL,                     -- e.g. '0 8 * * *' (Every morning at 08:00)
    condition_operator VARCHAR(10),                           -- '>', '<', '>=', '<=', '==', '!='
    threshold_value NUMERIC,                                  -- Alert trigger boundary
    channel VARCHAR(30) NOT NULL CHECK (channel IN ('TELEGRAM', 'SLACK', 'EMAIL', 'WEBHOOK')),
    recipient_config JSONB NOT NULL,                          -- {webhook_url, telegram_chat_id, slack_channel, email_list: []}
    env_tag VARCHAR(50) NOT NULL,                             -- Connection environment to query
    is_active BOOLEAN DEFAULT true,
    last_triggered_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 14. Public Shareable Links & Embedded Analytics
CREATE TABLE IF NOT EXISTS dashboard_shares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dashboard_id UUID REFERENCES dashboards(id) ON DELETE CASCADE,
    share_token VARCHAR(100) UNIQUE NOT NULL,                 -- Secure cryptographically random token
    access_mode VARCHAR(20) NOT NULL CHECK (access_mode IN ('PUBLIC_LINK', 'EMBED_IFRAME')),
    password_hash VARCHAR(255),                               -- Optional access password
    allowed_domains JSONB DEFAULT '[]',                       -- Whitelisted origins for iframe embedding: ["https://company.com"]
    is_active BOOLEAN DEFAULT true,
    expires_at TIMESTAMPTZ,                                   -- Optional expiration timestamp
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indices for Fast High-Performance Query Resolution
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_data_connections_ws ON data_connections(workspace_id);
CREATE INDEX IF NOT EXISTS idx_connection_environments_conn ON connection_environments(connection_id);
CREATE INDEX IF NOT EXISTS idx_projects_ws ON projects(workspace_id);
CREATE INDEX IF NOT EXISTS idx_dashboards_proj ON dashboards(project_id);
CREATE INDEX IF NOT EXISTS idx_widgets_dash ON widgets(dashboard_id);
CREATE INDEX IF NOT EXISTS idx_export_jobs_ws ON export_jobs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_ai_settings_ws ON ai_settings(workspace_id);
CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id);
CREATE INDEX IF NOT EXISTS idx_semantic_models_proj ON semantic_models(project_id);
CREATE INDEX IF NOT EXISTS idx_dashboard_alerts_dash ON dashboard_alerts(dashboard_id);
CREATE INDEX IF NOT EXISTS idx_dashboard_shares_token ON dashboard_shares(share_token);

-- +goose Down
DROP TABLE IF EXISTS dashboard_shares;
DROP TABLE IF EXISTS dashboard_alerts;
DROP TABLE IF EXISTS semantic_models;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS ai_settings;
DROP TABLE IF EXISTS export_jobs;
DROP TABLE IF EXISTS widgets;
DROP TABLE IF EXISTS dashboards;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS connection_environments;
DROP TABLE IF EXISTS data_connections;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS users;
