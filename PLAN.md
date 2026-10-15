# PLAN.md - Implementation Master Plan: muara.bi
**Product Name:** `muara.bi` (Unified BI & AI Analytics Platform)  
**Go Module Path:** `muara.bi/core` (inside `/backend`)  
**Frontend Package:** `@muara/web` (inside `/frontend`)  
**Architecture Reference:** Standard project structure and patterns adapted from `D:\GO\go-map-service`

---

## 1. Executive Summary & Architectural Vision

`muara.bi` is a high-performance, self-hosted Business Intelligence (BI) and AI Analytics platform engineered with **DuckDB** and **Golang** on the backend and **Next.js (App Router, Tailwind CSS, Shadcn UI, Apache ECharts, Monaco Editor)** on the frontend.

### Core Architectural Pillars
1. **Two-Level Multi-Tenancy (Workspace & Project):**
   - **Workspace:** Tenant isolation unit at the company/organization boundary.
   - **Project:** App-scoped analytical container (e.g., `Apps Omnichannel`, `Apps Chatbot`, `Retail ERP`) with dedicated dashboards, custom metrics, and access controls.
2. **Dynamic Multi-Environment Alias (Custom Tags - Flexible Pattern 3):**
   - A single logical connection alias (e.g., `db_transaksi`) stores encrypted credential profiles keyed by **Custom Environment Tags** (e.g., `production`, `staging`, `development`, `uat`, `sandbox`, `qa`, `dr-replica`, or arbitrary custom tags).
   - Switching the environment switcher in the UI dynamically binds the DuckDB `ATTACH` target to the selected custom tag without altering dashboard queries or widget definitions.
3. **Direct Connect by Default (Optional SSH Bastion):**
   - Data source setup is straightforward: simply enter **Host / IP Address, Port, Database Name, Username, and Password** (with optional passwordless mode for internal networks).
   - SSH Bastion Jump Host tunneling is an optional collapsible toggle that is only displayed when activated.
4. **Unified Canvas (Progressive Disclosure):**
   - **Layer 1 (Default):** AI Prompt-to-Dashboard bar & interactive 12-column canvas with compound cross-filtering.
   - **Layer 2 (Slide-Up Drawer - Tab 1):** No-Code Visual Data Builder (metrics, dimensions, filters).
   - **Layer 3 (Slide-Up Drawer - Tab 2):** Monaco SQL Editor (`</>`) supporting federated cross-database joins (PostgreSQL + MongoDB + Parquet/S3) with two-way synchronization (`SYNCED` vs `CUSTOM_SQL`).
5. **Standard Go Service Pattern (`go-map-service`):**
   - Lifecycle management with `pkg/container` (`BeforeStart`, `AfterStart`, graceful `Shutdown`).
   - Centralized dependency injection via `internal/app/server` (`server.go`, `internalconnection.go`, `repository.go`, `service.go`, `migration.go`).
   - Strict separation: `handler` $\rightarrow$ `service` (interface) $\rightarrow$ `repository` (interface).
   - Standardized HTTP responses via `pkg/httputil` and custom domain errors via `pkg/customerror`.
   - Asynchronous heavy workload offloading via `internal/worker` using Redis (`hibiken/asynq`).
6. **Distributed Multi-Worker & Multi-Tenant Engine:**
   - Distributed worker concurrency with `asynq`.
   - Ephemeral in-memory DuckDB sessions per query/job to ensure 100% tenant isolation and prevent alias collisions.
   - Weighted priority queues and per-tenant concurrency semaphores to eliminate noisy neighbors.
   - NoSQL in-memory adapter (MongoDB streaming $\rightarrow$ in-memory Arrow/JSON virtual tables).
   - Strict parameterized query binding (`$1, $2`) preventing SQL injection.
7. **Smart Query Caching & TTL Invalidation (Point 1):**
   - Redis hash-keyed cache (`SHA256(ws_id + env_tag + query_sql + params)`).
   - Configurable TTL per widget/dashboard (default 300s) + instant force refresh bypass (`X-Force-Refresh`).
8. **Scheduled Reports & Automated Alerts (Point 2):**
   - Distributed Asynq cron scheduler.
   - Daily/weekly digest (PDF/Excel) and threshold anomaly alerts dispatching to Telegram, Slack, Email, and Webhook.
9. **Reusable Semantic Layer (Point 3 - Saved Datasets / Virtual Views):**
   - Centralized complex business models mounted into DuckDB as temporary virtual views (`CREATE TEMP VIEW AS ...`).
10. **Granular RBAC & Row-Level Security (Point 4 - RLS):**
   - Workspace & project roles (`OWNER`, `ADMIN`, `ENGINEER`, `EDITOR`, `VIEWER`).
   - Dynamic user attribute injection (`branch_id`, `team`) directly into SQL WHERE clauses via Go AST parser.
11. **Public Sharing & Embedded Analytics (Point 5):**
   - Public read-only URLs with optional password/expiry and Signed JWT tokens for seamless `<iframe>` embedding.
12. **Query Guardrails & Circuit Breaker (Point 6):**
   - Automatic `LIMIT 10000` injection for visual previews and strict 30-second context timeout cancellation.

---

## 2. Standard Project Structure (Mirroring `go-map-service`)

```text
d:\GO\muara.bi\
├── PRD.md                             # Canonical product requirements document
├── PLAN.md                            # Comprehensive master implementation plan
├── docker-compose.yml                 # PostgreSQL, Redis (Asynq queue), MongoDB & Sample Postgres
├── Dockerfile                         # Turnkey CGO multi-stage build with pre-bundled DuckDB extensions
├── Makefile                           # Unified build, test, migration, and run targets
│
├── backend/                           # Go Core Backend Service (muara.bi/core)
│   ├── go.mod
│   ├── go.sum
│   ├── .env.example
│   ├── cmd/
│   │   ├── api/
│   │   │   └── main.go                # API entrypoint (loads cfg, logger, otel, migrations, container)
│   │   └── worker/
│   │       └── main.go                # Asynq Background Worker entrypoint (heavy query, export, sampling)
│   │
│   ├── docs/                          # Swag / Apidog OpenAPI specifications
│   │   ├── docs.go
│   │   ├── swagger.json
│   │   └── swagger.yaml
│   │
│   ├── internal/
│   │   ├── app/
│   │   │   └── server/
│   │   │       ├── server.go              # Chi router, global middleware, route mounts, lifecycle
│   │   │       ├── internalconnection.go  # PG metadata (*sql.DB), DuckDB pool, Redis client
│   │   │       ├── repository.go          # Repository provider aggregation struct
│   │   │       ├── service.go             # Service provider aggregation struct
│   │   │       └── migration.go           # Goose database migration runner
│   │   │
│   │   ├── config/
│   │   │   └── config.go              # App, DB, Redis, DuckDB, AI, SSH, and KMS configs
│   │   │
│   │   ├── generated/
│   │   │   └── migrations/            # Goose SQL migration files (00001_initial_schema.sql)
│   │   │
│   │   ├── dto/
│   │   │   ├── request/               # Strongly-typed request payloads with validator tags
│   │   │   │   ├── auth_request.go
│   │   │   │   ├── workspace_request.go
│   │   │   │   ├── project_request.go
│   │   │   │   ├── connection_request.go
│   │   │   │   ├── query_request.go
│   │   │   │   ├── dashboard_request.go
│   │   │   │   ├── ai_request.go
│   │   │   │   └── export_request.go
│   │   │   └── response/              # Strongly-typed API responses
│   │   │       ├── auth_response.go
│   │   │       ├── workspace_response.go
│   │   │       ├── project_response.go
│   │   │       ├── connection_response.go
│   │   │       ├── schema_response.go
│   │   │       ├── query_response.go
│   │   │       ├── dashboard_response.go
│   │   │       ├── ai_response.go
│   │   │       └── export_response.go
│   │   │
│   │   ├── handler/                   # HTTP handlers with Chi routing & Swagger annotations
│   │   │   ├── auth_handler.go
│   │   │   ├── workspace_handler.go
│   │   │   ├── project_handler.go
│   │   │   ├── connection_handler.go
│   │   │   ├── schema_handler.go
│   │   │   ├── query_handler.go
│   │   │   ├── dashboard_handler.go
│   │   │   ├── ai_handler.go
│   │   │   └── export_handler.go
│   │   │
│   │   ├── service/                   # Business logic provider interfaces & implementations
│   │   │   ├── auth_service.go
│   │   │   ├── workspace_service.go
│   │   │   ├── project_service.go
│   │   │   ├── connection_service.go
│   │   │   ├── schema_service.go
│   │   │   ├── query_service.go
│   │   │   ├── dashboard_service.go
│   │   │   ├── ai_service.go
│   │   │   └── export_service.go
│   │   │
│   │   ├── repository/                # Data persistence interfaces & implementations
│   │   │   └── pgsql/
│   │   │       ├── user_repo.go
│   │   │       ├── workspace_repo.go
│   │   │       ├── project_repo.go
│   │   │       ├── connection_repo.go
│   │   │       ├── dashboard_repo.go
│   │   │       ├── widget_repo.go
│   │   │       ├── export_job_repo.go
│   │   │       └── ai_settings_repo.go
│   │   │
│   │   ├── worker/                    # Asynq task definitions, client, and task handlers
│   │   │   ├── tasks.go               # Task types & payload definitions
│   │   │   ├── concurrency.go         # Per-tenant semaphore & fair-share rate limiter
│   │   │   ├── query_worker.go        # Async heavy query worker (ephemeral DuckDB session)
│   │   │   ├── export_worker.go       # Multi-sheet Excel & large CSV/JSON export worker
│   │   │   └── discovery_worker.go    # Background NoSQL sampling & schema refresher
│   │   │
│   │   ├── middleware/                # HTTP interceptors
│   │   │   ├── auth_middleware.go     # JWT token verification & user claims injection
│   │   │   ├── tenant_middleware.go   # Workspace resolution & context injection
│   │   │   └── rbac_middleware.go     # Role permission guard (OWNER, ADMIN, ENGINEER, etc.)
│   │   │
│   │   ├── helper/
│   │   │   ├── cryptohelper/          # AES-256-GCM encryption/decryption with Master Key
│   │   │   └── sqlparser/             # Strict AST validation & parameterized variable transformer
│   │   │
│   │   ├── duckdb/                    # DuckDB in-process manager & connection multiplexer
│   │   │   ├── session.go             # Ephemeral isolated session factory per tenant/worker
│   │   │   ├── attach.go              # Dynamic ATTACH (READ_ONLY) per alias and environment
│   │   │   ├── nosql_adapter.go       # MongoDB streaming -> in-memory Arrow/JSON virtual tables
│   │   │   └── guard.go               # Query timeout, memory limit, and read-only enforcement
│   │   │
│   │   ├── ssh/                       # SSH Jump Host Tunnel Manager
│   │   │   ├── tunnel.go              # Dynamic local port forwarding via golang.org/x/crypto/ssh
│   │   │   └── pool.go                # Active tunnel lifecycle registry & keep-alive
│   │   │
│   │   └── connectors/                # Heterogeneous source connectors & schema discovery
│   │       ├── connector.go           # Common Driver interface
│   │       ├── sql/                   # Postgres, MySQL, MSSQL, Oracle, ClickHouse
│   │       ├── nosql/                 # MongoDB, Elasticsearch, Redis (sampling & JSON unnesting)
│   │       └── storage/               # AWS S3, GCS, MinIO, Local Parquet/CSV
│   │
│   └── pkg/                           # Reusable framework utilities (mirroring go-map-service)
│       ├── container/                 # Lifecycle container (BeforeStart, AfterStart, Shutdown)
│       │   ├── app.go
│       │   └── option.go
│       ├── logging/                   # Slog/Uber-Zap structured logging wrapper
│       │   ├── logging.go
│       │   ├── handler.go
│       │   └── handler_option.go
│       ├── envparser/                 # Environment variable struct tag parser with defaults
│       │   ├── config.go
│       │   └── types.go
│       ├── httputil/                  # Standardized response envelopes & error writers
│       │   ├── response.go            # BaseResponse, Pagination, ResponseWriter
│       │   ├── header.go
│       │   └── client.go
│       ├── customerror/               # Centralized domain error codes and locators
│       │   ├── code.go
│       │   ├── error.go
│       │   └── locator.go
│       ├── swaggerutil/               # Swagger & Apidog extension handlers
│       │   └── apidog.go
│       └── telemetry/                 # OpenTelemetry Tracer and Metric provider
│           ├── otel.go
│           └── transport.go
│
└── frontend/                          # Next.js App Router Frontend (@muara/web)
    ├── package.json
    ├── tsconfig.json
    ├── tailwind.config.ts
    ├── components.json                # Shadcn UI configuration
    ├── public/
    └── src/
        ├── app/
        │   ├── layout.tsx             # Root layout with providers (Theme, QueryClient, Toaster)
        │   ├── page.tsx               # Redirect to default workspace or dashboard
        │   ├── (auth)/
        │   │   ├── login/page.tsx     # Clean login page
        │   │   └── register/page.tsx  # Register & create initial workspace
        │   └── [wsSlug]/
        │       ├── layout.tsx         # Workspace App Shell (Left Rail + Project Header Switcher)
        │       ├── projects/
        │       │   └── [projectSlug]/
        │       │       ├── dashboards/
        │       │       │   ├── page.tsx       # Dashboard Catalog & +Create Modal
        │       │       │   └── [dashId]/
        │       │       │       └── page.tsx   # Unified Canvas (12-col grid, drawer, cross-filters)
        │       │       └── studio/
        │       │           └── page.tsx       # Data Studio (Scratchpad federated query editor)
        │       ├── sources/
        │       │   └── page.tsx       # Data Sources (Direct IP connection form + optional SSH)
        │       ├── exports/
        │       │   └── page.tsx       # Exports & Job audit table
        │       └── settings/
        │           ├── page.tsx       # Workspace settings & members
        │           └── ai/page.tsx    # AI Co-Pilot settings (provider, models, glossary, PII masking)
        │
        ├── components/
        │   ├── ui/                    # Shadcn UI primitives (button, dialog, dropdown, tabs, etc.)
        │   ├── layout/
        │   │   ├── left-rail.tsx      # Collapsible navigation rail
        │   │   ├── env-switcher.tsx   # Global Environment Switcher (Prod, Staging, Dev)
        │   │   ├── project-switcher.tsx # Two-level Workspace > Project Switcher
        │   │   └── header.tsx
        │   ├── canvas/
        │   │   ├── grid.tsx           # react-grid-layout 12-column responsive canvas
        │   │   ├── widget-card.tsx    # Card container with ResizeObserver & menu
        │   │   ├── filter-bar.tsx     # Dynamic global filters & dynamic {{param}} inputs
        │   │   └── chart-renderer.tsx # Apache ECharts dynamic wrapper
        │   ├── inspector/
        │   │   ├── slide-drawer.tsx   # Unified 3-Layer Slide-Up Inspector Drawer
        │   │   ├── ai-prompt-bar.tsx  # Layer 1: Natural language AI Prompt-to-Dashboard input
        │   │   ├── visual-builder.tsx # Layer 2: Metric/Dimension drag-and-drop builder
        │   │   └── sql-editor.tsx     # Layer 3: Monaco Editor with live schema autocompletion
        │   ├── catalog/
        │   │   ├── dashboard-grid.tsx # Project dashboard cards
        │   │   └── create-modal.tsx   # Modal with 3 creation methods (AI, Template, Blank)
        │   ├── sources/
        │   │   ├── direct-connect-form.tsx # Default direct IP, port, user, pass form
        │   │   └── ssh-bastion-toggle.tsx  # Collapsible optional SSH Jump Host section
        │   └── tree/
        │       └── schema-tree.tsx    # Collapsible tree inspector (SQL + NoSQL)
        │
        ├── stores/
        │   ├── auth-store.ts          # Authenticated user session & JWT
        │   ├── workspace-store.ts     # Active workspace, members, role
        │   ├── project-store.ts       # Active project (Omnichannel, Chatbot, etc.)
        │   ├── env-store.ts           # Active environment ('production' | 'staging' | 'development')
        │   ├── canvas-store.ts        # Grid layout state, selected widget, drawer mode, debounced auto-save
        │   ├── filter-store.ts        # Cross-filtering selections & global dynamic params
        │   └── schema-store.ts        # Cached schema tree per environment with quick search
        │
        ├── hooks/
        │   ├── use-query-executor.ts  # React Query hook for /query/execute
        │   ├── use-schema-tree.ts     # Cached schema tree provider
        │   ├── use-cross-filter.ts    # Anti-self-loop compound cross-filter handler
        │   └── use-variable-parser.ts # Regex dynamic variable parser for {{param}}
        │
        ├── lib/
        │   ├── api.ts                 # Axios client with JWT bearer interceptor
        │   ├── echarts-options.ts     # Deterministic tabular data -> ECharts options generator
        │   ├── monaco-completion.ts   # Custom schema completion provider for Monaco
        │   └── utils.ts               # Tailwind class merge helper
        └── types/                     # Shared TypeScript interfaces matching Go DTOs
```

---

## 3. PostgreSQL Metadata Database Schema (Goose Migration)

File path: `backend/internal/generated/migrations/00001_initial_schema.sql`

```sql
-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. Users (Authentication & User Profiles)
CREATE TABLE users (
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
CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    plan_tier VARCHAR(30) DEFAULT 'pro',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 3. Workspace Members & RBAC
CREATE TABLE workspace_members (
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('OWNER', 'ADMIN', 'ENGINEER', 'EDITOR', 'VIEWER')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (workspace_id, user_id)
);

-- 4. Logical Data Connections (Alias used in SQL & Schema Inspector)
CREATE TABLE data_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    alias_name VARCHAR(50) NOT NULL, -- e.g. 'db_transaksi' -> SELECT * FROM db_transaksi.orders
    category VARCHAR(20) NOT NULL CHECK (category IN ('SQL', 'NOSQL', 'FILE', 'CLOUD_DW')),
    driver_type VARCHAR(30) NOT NULL, -- 'postgres', 'mysql', 'mongodb', 'elasticsearch', 'redis', 's3'
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (workspace_id, alias_name)
);

-- 5. Dynamic Custom Environment Profiles per Connection (Flexible Pattern 3)
CREATE TABLE connection_environments (
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
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    icon VARCHAR(30) DEFAULT 'folder',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (workspace_id, slug)
);

-- 7. Dashboards (Grid Layout, Global Filters & Caching)
CREATE TABLE dashboards (
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
CREATE TABLE widgets (
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
CREATE TABLE export_jobs (
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
CREATE TABLE ai_settings (
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
CREATE TABLE project_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('ADMIN', 'EDITOR', 'VIEWER')),
    rls_rules JSONB DEFAULT '[]', -- Row-level rules: [{"column": "branch_id", "operator": "=", "value_from_user": "branch_id"}]
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (project_id, user_id)
);

-- 12. Reusable Semantic Models / Virtual Views (Saved Datasets)
CREATE TABLE semantic_models (
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
CREATE TABLE dashboard_alerts (
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
CREATE TABLE dashboard_shares (
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

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_data_connections_ws ON data_connections(workspace_id);
CREATE INDEX idx_connection_environments_conn ON connection_environments(connection_id);
CREATE INDEX idx_projects_ws ON projects(workspace_id);
CREATE INDEX idx_dashboards_proj ON dashboards(project_id);
CREATE INDEX idx_widgets_dash ON widgets(dashboard_id);
CREATE INDEX idx_export_jobs_ws ON export_jobs(workspace_id);
CREATE INDEX idx_ai_settings_ws ON ai_settings(workspace_id);
CREATE INDEX idx_project_members_user ON project_members(user_id);
CREATE INDEX idx_semantic_models_proj ON semantic_models(project_id);
CREATE INDEX idx_dashboard_alerts_dash ON dashboard_alerts(dashboard_id);
CREATE INDEX idx_dashboard_shares_token ON dashboard_shares(share_token);

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
```

---

## 4. Backend Technical Mechanics & Distributed Query Engine

### 4.1. Security & Encryption (`internal/helper/cryptohelper`)
- **Algorithm:** AES-256-GCM.
- **Key Source:** 32-byte master key loaded from environment variable `APP_ENCRYPTION_MASTER_KEY` via `internal/config`.
- **Payload:** Serializes database credentials (including sensitive SSH private keys, DB passwords, and AI API keys) into JSON, then encrypts with a randomized 12-byte nonce, prepending the nonce to the ciphertext before persisting as `BYTEA` in PostgreSQL.

### 4.2. Connection Manager: Direct Connect vs Optional SSH Bastion
- **Default Direct Connect (Clean & Fast):**
  - Database connection modal directly exposes:
    - **Host / IP Address** (`127.0.0.1` or `10.0.0.5` or `db.internal.net`)
    - **Port** (defaulted by driver: 5432 for Postgres, 3306 for MySQL, 27017 for MongoDB)
    - **Database Name**
    - **Username & Password** (with optional passwordless toggle for local/internal DBs)
- **Optional SSH Bastion Jump Host (Collapsible Toggle):**
  - Toggle *"Use SSH Bastion Tunnel"* (default: OFF).
  - When enabled, fields expand for Bastion Host, Port (22), User, and private key (`.pem` / `id_rsa`).
  - `internal/ssh` dynamically dials the jump host and forwards local port to the remote DB.

### 4.3. DuckDB Embedded Engine & Dynamic Multi-Environment ATTACH
- Uses `github.com/marcboeker/go-duckdb` in CGO mode.
- Instantiates an in-process, memory-bounded DuckDB database.
- **Safety Enforcement:**
  - Installs strict execution bounds: `SET max_memory='2GB'`, `SET threads=2`, `SET statement_timeout='30s'`.
  - Parses AST before execution: rejects any query containing DDL or DML (`DROP`, `DELETE`, `UPDATE`, `INSERT`, `ALTER`, `TRUNCATE`, `PRAGMA`).
  - All SQL targets are attached in `READ_ONLY` mode:
    ```sql
    ATTACH 'host=127.0.0.1 port=5432 dbname=prod_db user=readonly password=secret' AS db_transaksi (TYPE postgres, READ_ONLY);
    ```

### 4.4. Heterogeneous Query Mechanics: SQL vs NoSQL In-Memory Adapter
1. **SQL (PostgreSQL, MySQL, SQLite):**
   - DuckDB performs direct queries via pre-installed extensions (`postgres_scanner`, etc.).
2. **NoSQL (MongoDB, Elasticsearch, Redis) -> In-Memory Streaming Adapter (`internal/duckdb/nosql_adapter.go`):**
   - Because DuckDB lacks a native MongoDB driver, Go acts as an in-memory streaming bridge:
     1. Go executes query/aggregation on MongoDB driver.
     2. Go un-nests and flattens JSON documents (e.g., `customer.address.city`).
     3. Go registers the memory buffer into DuckDB as an in-memory temporary view:
        ```sql
        CREATE TEMP VIEW db_logs_sessions AS SELECT * FROM read_json_auto('/dev/shm/... or in-memory arrow buffer');
        ```
     4. Federated DuckDB queries joining PostgreSQL `orders` and MongoDB `sessions` execute seamlessly in-memory.

### 4.5. Strict Parameterized Query Binding (`$1, $2`) for `{{param}}`
- **Zero Raw String Concatenation:**
  - `{{param}}` variables and cross-filters are **never** injected via `strings.Replace`.
  - AST Transformer in Go converts `WHERE branch = {{branch}}` into parameterized placeholder `WHERE branch = $1`.
  - Values are bound through Go's database driver:
    ```go
    rows, err := duckdbSession.QueryContext(ctx, parameterizedSQL, branchVal)
    ```
  - Eliminates SQL injection vulnerabilities entirely.

### 4.6. Multi-Format Exporter Engine (`internal/exporter` & `internal/worker/export_worker.go`)
- **CSV / JSON:** Native DuckDB `COPY (...) TO '...' WITH (FORMAT CSV, HEADER)` or `(FORMAT JSON)` for maximum streaming throughput.
- **Multi-Sheet Excel (.xlsx):** Utilizes `github.com/xuri/excelize/v2` with `NewStreamWriter()`. Supports **Full Dashboard Export**: iterates over every widget in the dashboard, executes its query, and writes each widget's tabular output to a dedicated named sheet with bold styled headers, numeric formatting, and auto-adjusted column widths.

### 4.7. Multi-Worker & Multi-Tenant Heavy Query Engine (`internal/worker`)
1. **Ephemeral Isolated DuckDB Sessions (`internal/duckdb/session.go`):**
   - Each heavy query or background worker job executes inside an isolated in-memory DuckDB session (`duckdb.Open(":memory:")`).
   - Wipes memory upon completion, ensuring zero alias collisions between tenants and full OS memory reclaim.
2. **Distributed Asynq Worker Pool (`cmd/worker/main.go`):**
   - Weighted queues: `critical` (weight: 6), `default` (weight: 3), `heavy` (weight: 1).
   - Redis per-tenant semaphores enforce concurrent execution quotas (Pro: 5 slots, Enterprise: 10 slots) to prevent noisy neighbor starvation.

### 4.8. AI Co-Pilot with Semantic Schema Pruning & Privacy Masking
- **Semantic Schema Pruner:**
  - Scans prompt keywords to select only the top 5-10 relevant tables/columns before injecting into LLM prompt, preventing token overflow and reducing cost.
- **Data Privacy Masking:**
  - Automatically scrubs sensitive columns (`phone`, `email`, `salary`, `credit_card_hash`) from prompts.
- **Model Routing:**
  - Default Model (`Claude 3.5 Sonnet` / `GPT-4o`) for complex SQL AST and dashboard layout generation.
  - Fast Model (`Gemini 1.5 Flash` / `GPT-4o-mini`) for instant metadata lookups.

### 4.9. Smart Query Caching & Invalidation (`internal/cache/query_cache.go`)
- **Deterministic Cache Key:**
  - `cache:query:<ws_id>:<env_tag>:<sha256(raw_sql + serialized_filters)>`
  - Encoded in msgpack or gzip JSON in Redis.
- **TTL Strategy:**
  - Dashboard default TTL: 300 seconds (configurable 0-86400s).
  - Widget-level override for slow aggregates.
- **Bypass & Force Refresh:**
  - Header `X-Force-Refresh: true` purges key and executes live DuckDB query.

### 4.10. Scheduled Reports & Automated Alerts (`internal/worker/alert_worker.go`)
- **Asynq Cron Runner:**
  - Periodic task every 60s checks active `dashboard_alerts`.
- **Digest Generator:**
  - Generates dashboard image / multi-sheet XLSX report.
  - Dispatches to Telegram Bot API, Slack Webhook, SMTP Email, or custom webhook.
- **Threshold Evaluator:**
  - Runs DuckDB aggregate query. If `result > threshold_value`, fires immediate alert with chart snapshot.

### 4.11. Reusable Semantic Models / Virtual Views (`internal/duckdb/semantic_layer.go`)
- **Temporary View Mounting:**
  - In each DuckDB session, active `semantic_models` for the project are registered:
    ```sql
    CREATE TEMP VIEW dataset_performa_agen AS (SELECT ... complex query ...);
    ```
  - Widgets can query virtual views as if they were physical tables.

### 4.12. Granular RBAC & Row-Level Security (`internal/middleware/rls.go`)
- **Project Roles:** `ADMIN`, `EDITOR`, `VIEWER`.
- **Dynamic RLS AST Injection:**
  - Evaluates user's `attributes` against `project_members.rls_rules`.
  - Appends parameterized conditions (e.g., `AND branch_id = $N`) into DuckDB SQL query AST before execution.

### 4.13. Public Sharing & Embedded Analytics (`internal/service/share_service.go`)
- **Public Share Links:** Cryptographically random tokens (`/share/:token`) with optional bcrypt password validation.
- **Signed JWT Embeds:** IFrame embedding protected by CSP `frame-ancestors` verified against `allowed_domains`.

### 4.14. Query Guardrails & Circuit Breaker (`internal/duckdb/guardrails.go`)
- **Automatic Limit Enforcement:** Automatically appends `LIMIT 10000` to ad-hoc / preview queries if missing.
- **Strict Cancellation Context:** `context.WithTimeout(ctx, 30*time.Second)` gracefully closes DuckDB query channel if timeout occurs.

---

## 5. Frontend Comprehensive Architecture & Mechanics Specification (`@muara/web`)

### 5.1. Design System & Global App Shell (Base Color: Terang Semi-Putih / Off-White)
- **Base Color Palette (Modern Clean Off-White):**
  - **Background Canvas:** `#f8fafc` (Tailwind Slate-50) / `#f1f5f9` (Slate-100)
  - **Cards & Modal Surfaces:** `#ffffff` with subtle slate borders (`#e2e8f0`) and soft ambient shadows (`shadow-sm`, `shadow-md`)
  - **Typography:** `#0f172a` (Slate-900) for primary headers, `#475569` (Slate-600) for body/descriptions
  - **Accent Colors:** Primary Cyan (`#0891b2`), Indigo (`#4f46e5`), Emerald Green (`#059669`) for positive trends, Amber (`#d97706`) for warnings
  - **Dark Mode Support:** Clean toggling via `next-themes` (persisted in local storage)
- **Two-Level Workspace & Project Hierarchy:**
  - `Workspace Selector` (e.g. `🏢 Nusantara Corp`) $\rightarrow$ `Project Selector` (e.g. `📱 Apps Omnichannel`, `🤖 Apps Chatbot`, `🛒 E-Commerce & Retail`)
- **Dashboard Catalog & Create Wizard:**
  - Dedicated Catalog View (`/[wsSlug]/projects/[projectSlug]/dashboards`) displaying project dashboards with search, tags, and creator info.
  - **Create New Dashboard Modal:** Offers 3 creation pathways:
    1. *✨ Generate with AI Co-Pilot* (Natural language prompt-to-dashboard)
    2. *📋 Start from Project Template* (Pre-built metrics for Chatbot, Omnichannel, Retail)
    3. *▦ Blank 12-Column Canvas* (Custom design from scratch)
- **Left Navigation Rail (Collapsible, 64px to 220px):**
  1. `📁 Catalog`: Project dashboard inventory and template launcher.
  2. `📊 Dashboards`: Unified canvas view, widget designer, 12-col grid.
  3. `⚡ Data Studio`: Ad-hoc multi-database query scratchpad.
  4. `🔌 Data Sources`: Connection manager with multi-env tabs (Direct IP by default, optional SSH).
  5. `📥 Exports & Jobs`: Download history and asynchronous export tracker.
  6. `⚙️ Workspace Settings`: Member invitations, RBAC roles, audit logs, and AI Co-Pilot configuration.
- **Top Application Header:**
  - Workspace selector dropdown (`/ws-slug/...`).
  - Project switcher dropdown (`/projects/:projectSlug/...`).
  - **Global Environment Switcher:** Dropdown toggling `production` (Red badge), `staging` (Yellow badge), or `development` (Green badge).

### 5.2. Zustand State Management Architecture
- `useAuthStore`: Authenticated user, JWT token, login/logout.
- `useEnvStore`: Active environment (`production` | `staging` | `development`).
- `useCanvasStore`: 12-column grid coordinates `{i, x, y, w, h}`, selected widget, drawer state, and debounced auto-save (800ms).
- `useFilterStore`: Global filters, dynamic `{{param}}` values, and interactive cross-filtering.
- `useSchemaStore`: Cached schema tree per environment with quick search.
- `useProjectStore`: Active project metadata.

### 5.3. Unified Canvas & ECharts Dynamic Option Generator
- **12-Column Responsive Grid (`react-grid-layout`):**
  - ECharts rendered with `ResizeObserver` for zero-lag scaling.
- **Deterministic ECharts Generator (`lib/echarts-options.ts`):**
  - Automatically converts tabular output (`columns`, `rows`, `types`) + `builder_state` into typed ECharts option JSON.
- **Anti-Self-Loop Compound Cross-Filtering:**
  - When clicking Widget A, Widget A retains full visibility (dimming other categories), while Widget B, C, D re-query with bound parameter clauses.

### 5.4. Data Sources UI: Direct Connect by Default
- Form default:
  - `Host / IP`: Input field (e.g. `192.168.1.100` or `db.internal.net`)
  - `Port`: Pre-filled (5432, 3306, 27017)
  - `Database Name`: Input field
  - `Username & Password`: Input fields (with toggle for passwordless)
- Collapsible section:
  - `[ ] Use SSH Bastion Tunnel` (off by default; expands Bastion Host, Port 22, User, PEM Key upload when checked).

---

## 6. Implementation Milestones & Task Breakdown

### Phase 1: Go Foundation, Clean Architecture & PostgreSQL Migrations (Backend)
- [ ] Setup `docker-compose.yml` with PostgreSQL (metadata), Redis (Asynq queue & cache), and sample target databases.
- [ ] Initialize Go module `muara.bi/core` in `/backend`.
- [ ] Implement `pkg/` framework packages identical to `go-map-service`:
  - `pkg/container`, `pkg/logging`, `pkg/envparser`, `pkg/httputil`, `pkg/customerror`, `pkg/telemetry`.
- [ ] Implement `internal/config/config.go` with full environment mappings.
- [ ] Setup `internal/app/server`:
  - `internalconnection.go`: PostgreSQL pool (`jackc/pgx/v5`), Redis client (`asynq` + caching).
  - `migration.go`: Goose migration runner for `00001_initial_schema.sql` (14 enterprise tables).
  - `server.go`: Chi router, middleware chain (`RequestID`, `RealIP`, `Logger`, `Recoverer`, `JWT`, `RLS`), route mounting.
  - `repository.go` and `service.go` dependency injection structs.
- [ ] Implement Authentication & RLS Attributes Module:
  - `user_repo.go`, `auth_service.go`, `auth_handler.go` (`/auth/register`, `/auth/login`, `/auth/me`).
  - JWT generation and claims validation middleware.
- [ ] Implement `internal/helper/cryptohelper` (AES-256-GCM encryption/decryption with Master Key).
- [ ] Verify clean startup, database migration, and auth flow via `cmd/api/main.go`.

### Phase 2: Distributed DuckDB Engine, Smart Caching, Connectors & Multi-Worker (Backend)
- [ ] Implement `internal/duckdb/session.go`:
  - Ephemeral in-memory DuckDB session factory.
  - Dynamic `ATTACH ... (READ_ONLY)` resolver mapped to logical aliases and active environments (via custom tags e.g. `production`, `staging`, `development`, `uat`, etc.).
  - AST SQL Parser guardrails (`internal/duckdb/guardrails.go`): blocks write/DDL operations, auto-appends `LIMIT 10000` to ad-hoc previews, and enforces 30s context cancellation.
  - Strict resource limits (`max_memory = '2GB'`, `threads = 2`, `statement_timeout = '30s'`).
- [ ] Implement Smart Query Caching (`internal/cache/query_cache.go`):
  - Deterministic Redis key `cache:query:<ws_id>:<env_tag>:<sha256(sql+filters)>`.
  - Configurable TTL (300s default) with `X-Force-Refresh` bypass support.
- [ ] Implement Reusable Semantic Models / Virtual Views (`internal/duckdb/semantic_layer.go`):
  - Injects active `semantic_models` into DuckDB sessions as temporary views (`CREATE TEMP VIEW ... AS ...`).
- [ ] Implement `internal/duckdb/nosql_adapter.go`:
  - MongoDB streaming -> in-memory Arrow/JSON virtual tables.
- [ ] Implement strict parameterized query transformer for `{{param}}` and cross-filters (`$1, $2`).
- [ ] Implement `internal/ssh`: Dynamic SSH Jump Host tunnel listener and lifecycle pool.
- [ ] Implement Redis-backed `hibiken/asynq` multi-worker queue:
  - `cmd/worker/main.go` entrypoint with configurable concurrency.
  - `internal/worker/concurrency.go` per-tenant fair-share semaphore and priority queues (`critical`, `default`, `heavy`).
  - `internal/worker/query_worker.go` for asynchronous heavy queries.
  - `internal/worker/export_worker.go` for multi-sheet Excel and CSV streaming.
  - `internal/worker/alert_worker.go` for scheduled digests (PDF/Excel) & threshold alerts (Telegram, Slack, Email, Webhook).
- [ ] Implement Repositories and Services:
  - `ConnectionRepositoryProvider` and `ConnectionService` (CRUD with encrypted configs, Direct IP & optional SSH).
  - `SchemaRepositoryProvider` and `SchemaService` (cached schema and auto-discovery trigger).
  - `QueryService`: Parameterized execution of DuckDB queries with Redis caching and dynamic bindings.
  - `SemanticModelRepositoryProvider` and `SemanticModelService`.
- [ ] Implement Handlers:
  - `connection_handler.go` (`/connections/test`, `/connections`).
  - `schema_handler.go` (`/schema?env=...`).
  - `query_handler.go` (`/query/execute`, `/query/refresh`).
  - `semantic_model_handler.go` (`/semantic-models`).

### Phase 3: Dashboard Domain, RBAC/RLS, Sharing & Alerts (Backend)
- [ ] Implement `internal/exporter`:
  - Native DuckDB streaming CSV and JSON export.
  - Multi-sheet Excel (`.xlsx`) generator using `excelize` (1 widget = 1 sheet, auto column widths, styled header).
- [ ] Implement Project, Dashboard, and Widget Repositories & Services:
  - `ProjectRepositoryProvider` and `ProjectService` with granular RBAC & RLS rules (`project_members`).
  - `DashboardRepositoryProvider` and `DashboardService` (with cache TTL settings).
  - `WidgetRepositoryProvider` and `WidgetService` (with max rows and TTL override).
- [ ] Implement Public Share & Embedded Analytics:
  - `ShareRepositoryProvider` and `ShareService` (Public token URL & Signed JWT for iFrame with CSP check).
- [ ] Implement Scheduled Alerts & Digest Service:
  - `AlertRepositoryProvider` and `AlertService` (Telegram, Slack, Email, Webhook dispatchers).
- [ ] Implement AI Co-Pilot Service:
  - `ai_settings_repo.go`, `ai_settings_service.go`, `ai_settings_handler.go`.
  - Semantic Schema Pruner and PII privacy masking.
  - Structured prompt-to-dashboard generator (`/ai/generate-dashboard`).
- [ ] Implement Handlers:
  - `project_handler.go`, `project_member_handler.go` (RBAC & RLS management).
  - `dashboard_handler.go`, `widget_handler.go` (Catalog CRUD, layout updates).
  - `share_handler.go` (Public link, JWT embed).
  - `alert_handler.go` (Digest & threshold alerts CRUD and test triggers).
  - `export_handler.go` (Instant stream or async job trigger).

### Phase 4: Next.js Frontend (`@muara/web`) - Light Off-White Theme
- [ ] Initialize Next.js 15+ App Router in `/frontend` with TypeScript and Tailwind CSS.
- [ ] Configure light off-white palette (`#f8fafc`, `#ffffff`, `#e2e8f0`, `#0891b2`).
- [ ] Install and configure Shadcn UI components.
- [ ] Setup Zustand stores (`auth-store`, `env-store`, `canvas-store`, `filter-store`, `schema-store`, `project-store`).
- [ ] Build Global Shell:
  - Left Navigation Rail (Catalog, Dashboards, Studio, Semantic Models, Sources, Alerts, Exports, Settings).
  - Header with Workspace Switcher, Project Switcher, Environment Switcher, and "Sync / Force Refresh" button.
- [ ] Build Dashboard Catalog Page (`/[wsSlug]/projects/[projectSlug]/dashboards`):
  - Card inventory of project dashboards with role-based badges.
  - **Create New Dashboard Modal** with 3 options: AI Co-Pilot, Project Template, Blank Canvas.
- [ ] Build Unified Canvas (`/dashboards/[dashId]`):
  - 12-column grid using `react-grid-layout` with debounced auto-save (800ms).
  - Responsive `Apache ECharts` widget renderer with `ResizeObserver` and deterministic option generator.
  - Anti-self-loop compound cross-filtering.
  - Dynamic Filter Bar for auto-detected `{{param}}` inputs.
  - "Share / Embed" modal (generate public link or iframe code).
  - "Set Alert / Digest" modal (Telegram/Slack/Email threshold triggers).
- [ ] Build 3-Layer Progressive Slide-Up Drawer:
  - Layer 1: AI Prompt Bar with streaming feedback.
  - Layer 2: Visual Builder Tab (`SYNCED` mode).
  - Layer 3: Monaco SQL Editor Tab (`CUSTOM_SQL` mode) with custom schema autocomplete provider (`monaco-completion.ts`).
- [ ] Build Semantic Models Studio (`/semantic-models`):
  - Create, test, and save reusable virtual views.
- [ ] Build Data Studio Page (`/studio`):
  - Multi-tab federated query scratchpad with virtualized results table.
  - "Save as Widget to Dashboard" modal.
- [ ] Build Data Sources Page (`/sources`):
  - **Default Direct Connect form (Host/IP, Port, DB Name, User, Password).**
  - **Collapsible optional SSH Bastion toggle.**
  - "Test Connection" button with real-time ping latency.
- [ ] Build Public & Embedded Views (`/share/[token]` & `/embed`):
  - Clean, unbranded, high-performance read-only dashboard canvas.
- [ ] Build AI Co-Pilot Settings Page (`/settings/ai`):
  - Provider selector (OpenAI, Gemini, Claude, Ollama).
  - Model routing (Default vs Fast model).
  - Business glossary and PII schema masking rules.

### Phase 5: End-to-End Integration, Docker Packaging & Testing
- [ ] Create production multi-stage `Dockerfile` with CGO and pre-installed DuckDB extensions (`postgres_scanner`, etc.).
- [ ] Run full federated query tests in DuckDB across attached PostgreSQL and MongoDB containers.
- [ ] Concurrency load test: Execute simultaneous heavy queries; verify tenant fair-share semaphores prevent starvation.
- [ ] Cache verification: Verify identical queries hit Redis in <5ms without querying target DB.
- [ ] RLS verification: Verify user attributes automatically filter row data without leaks.
- [ ] Alert verification: Test firing test alert to Telegram and Slack channels.
- [ ] Verify instant environment switching in the UI without modifying dashboard configs.
- [ ] Verify multi-sheet Excel export for a multi-widget dashboard.
- [ ] Audit SQL execution paths to ensure read-only guardrails and parameterized bindings are impenetrable.

---

## 7. Immediate Next Steps
1. Create `docker-compose.yml` (PostgreSQL metadata, Redis queue & cache, MongoDB sample, Postgres sample).
2. Initialize backend directory structure matching `go-map-service`.
3. Create `backend/go.mod` (`muara.bi/core`) and `.env.example`.
4. Implement `pkg/` utility packages and configuration loader.
5. Create Goose database migration `00001_initial_schema.sql` (14 enterprise tables) and run server startup test.
