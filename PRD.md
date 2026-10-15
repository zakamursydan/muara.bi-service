# PRD.md - Product Requirements Document: muara.bi
**Product Name:** `muara.bi` (Unified BI & AI Analytics Platform)  
**Version:** 1.0.0  
**Backend:** Go 1.23+ (`muara.bi/core` in `/backend`)  
**Frontend:** Next.js 15+ App Router, TypeScript, Tailwind CSS, Shadcn UI (`@muara/web` in `/frontend`)

---

## 1. Executive Summary & Core Philosophy

`muara.bi` adalah platform *Business Intelligence* (BI) & *AI Analytics* mandiri berbasis web dengan performa tinggi yang dibangun di atas **DuckDB** dan **Golang**.

### 1.1 Core Principles
1. **Workspace & Project Multi-Tenancy:**
   - **Workspace:** Isolasi organisasi/perusahaan tingkat atas.
   - **Project:** Pengelompokan analitik spesifik aplikasi di dalam workspace (misal: *Apps Omnichannel*, *Apps Chatbot*, *Retail ERP*).
2. **Dynamic Multi-Environment Alias (Custom Tags - Pola 3 Fleksibel):**
   - Satu koneksi database memiliki satu nama alias logis (misal: `db_transaksi`), namun menyimpan profil kredensial berbasis **Custom Environment Tags** (misal: `production`, `staging`, `development`, `uat`, `sandbox`, `qa`, `dr-replica`, atau tag kustom lainnya).
   - Pengguna bebas mendefinisikan dan menambah tag environment baru sesuai kebutuhan arsitektur perusahaan tanpa batasan nama yang kaku.
   - Mengganti *Environment Switcher* di UI langsung memindahkan target koneksi DuckDB (`ATTACH`) ke tag yang dipilih tanpa perlu menulis ulang query SQL atau menduplikasi dashboard.
3. **Direct Connect as Default (Optional SSH Bastion):**
   - Form koneksi secara default langsung menerima **Host / IP, Port, DB Name, Username, dan Password** (atau mode passwordless untuk network internal).
   - Opsi SSH Bastion Jump Host bersifat toggle opsional (*collapsible*) yang hanya muncul ketika dinyalakan.
4. **Unified Canvas (Progressive Disclosure):**
   - Tidak ada saklar terpisah antara "Mode Awam" dan "Mode Expert". Seluruh persona disatukan melalui 3 layer:
     - **Lapis 1 (Default):** AI Prompt-to-Dashboard & Interactive 12-Column Canvas.
     - **Lapis 2 (Slide-Up Drawer - Tab 1):** No-Code Visual Data Builder (klik metrik/dimensi).
     - **Lapis 3 (Slide-Up Drawer - Tab 2):** Monaco SQL Editor (`</>`) dengan sinkronisasi dua arah (`SYNCED` $\leftrightarrow$ `CUSTOM_SQL`) dan dukungan *Federated Cross-Database Join* (SQL + NoSQL + Parquet/S3).

---

## 2. PostgreSQL Metadata Schema Specification

```sql
-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. Users (Authentication & Identity)
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
    alias_name VARCHAR(50) NOT NULL, -- e.g. 'db_transaksi' -> used as: SELECT * FROM db_transaksi.orders
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

## 3. Core Features & Technical Mechanics

### 3.1 Enterprise Connection Manager: Direct Connect & Optional SSH
- **Default Direct Connect (Clean & Fast):**
  - Untuk koneksi database biasa (lokal, VPC internal, atau public IP), form hanya menampilkan:
    - **Host / IP Address** (misal `192.168.1.100` atau `db.perusahaan.internal`)
    - **Port** (default auto-fill sesuai driver: 5432, 3306, 27017, dll.)
    - **Database Name**
    - **Username & Password** (disertai toggle opsi *Passwordless / Direct Network*)
- **Optional SSH Bastion Jump Host (Collapsible Toggle):**
  - Toggle *"Use SSH Bastion Tunnel"* (default: OFF).
  - Ketika dinyalakan, form menampilkan input Bastion Host, Port (22), User, dan upload Private Key `.pem` / `id_rsa`.
  - Go `internal/ssh` membuka port forwarding dinamis dan mengarahkan target DuckDB ke `127.0.0.1:<local_port>`.

### 3.2 Heterogeneous Query Mechanics: SQL vs NoSQL In-Memory Adapter
1. **SQL (PostgreSQL, MySQL, SQLite):**
   - DuckDB melakukan direct query via native scanner extension:
     ```sql
     ATTACH 'host=... port=... user=... password=... dbname=...' AS db_transaksi (TYPE postgres, READ_ONLY);
     ```
2. **NoSQL (MongoDB, Elasticsearch, Redis) -> In-Memory Arrow/JSON Adapter:**
   - Karena DuckDB tidak memiliki native MongoDB scanner, Go backend bertindak sebagai **In-Memory Streaming Adapter**:
     1. Go mengeksekusi query/pipeline ke MongoDB driver native.
     2. Go melakukan streaming dan JSON flattening terhadap dokumen (menangani nested key seperti `customer.address.city`).
     3. Go memuat buffer data ke sesi DuckDB sebagai temporary view in-memory:
        ```sql
        CREATE TEMP VIEW db_logs_sessions AS SELECT * FROM read_json_auto('/dev/shm/... or memory buffer');
        ```
     4. Query pengguna yang melakukan JOIN antara SQL (`db_transaksi.orders`) dan MongoDB (`db_logs.sessions`) dapat berjalan mulus dengan performa in-memory.

### 3.3 Strict Security & Anti-SQL Injection on Dynamic Variables (`{{param}}`)
- **Dilarang Keras String Concatenation:**
  - Token `{{param}}` dan klausa *Cross-Filtering* tidak pernah disuntikkan via string replace mentah.
  - AST Parser di Go mendeteksi variabel `{{branch}}`, menggantinya dengan parameter binding `$1`, dan mengeksekusinya via:
    ```go
    rows, err := duckdbSession.QueryContext(ctx, parameterizedSQL, branchValue)
    ```
- **AST Guardrail Validation:**
  - Query divalidasi oleh AST parser Go untuk memblokir kata kunci berbahaya (`DROP`, `DELETE`, `UPDATE`, `INSERT`, `ALTER`, `TRUNCATE`, `PRAGMA`).

### 3.4 AI Co-Pilot with Semantic Schema Pruning & Privacy Masking
- **Semantic Schema Pruner:**
  - Mencegah *token overflow* pada database besar (100+ tabel).
  - Go AI service memindai kata kunci dari prompt pengguna dan hanya menginjeksi skema tabel yang relevan ke LLM.
- **Data Privacy Masking:**
  - Kolom PII (`phone`, `email`, `salary`, `credit_card`) otomatis dihilangkan dari skema sebelum dikirim ke third-party AI.
- **Model Routing:**
  - Default Model (`Claude 3.5 Sonnet` / `GPT-4o`) untuk query kompleks & layout dashboard.
  - Fast Model (`Gemini 1.5 Flash` / `GPT-4o-mini`) untuk lookup metadata sub-second.

### 3.5 Dynamic Dashboard Designer & Unified Canvas Mechanics
- **Two-Level Scoping:** Workspace $\rightarrow$ Project $\rightarrow$ Dashboard.
- **12-Column Responsive Grid (`react-grid-layout`):**
  - ECharts rendered with `ResizeObserver` for 60fps zero-lag resizing.
  - Debounced auto-save (800ms) to prevent database hammering on card movement.
- **Anti-Self-Loop Compound Cross-Filtering:**
  - Mengklik elemen di Widget A menyaring Widget B, C, dan D tanpa merusak state visual Widget A.
- **3-Layer Unified Inspector Drawer:**
  - Layer 1: AI Prompt Bar.
  - Layer 2: Visual Builder Tab (`SYNCED`).
  - Layer 3: Monaco SQL Editor Tab (`CUSTOM_SQL`) with custom DuckDB schema autocomplete.

### 3.6 Smart Query Caching & TTL Invalidation (Point 1)
- **Hash-Keyed Redis Caching:**
  - Kunci cache dihitung secara deterministik:
    ```go
    cacheKey := fmt.Sprintf("cache:query:%s:%s:%s", wsID, envTag, sha256(rawSQL + serializedFilters))
    ```
  - Hasil tabular (`columns` dan `rows`) disimpan di Redis dengan TTL terkonfigurasi (default: 300 detik).
- **Force Refresh Mechanism:**
  - Request dengan header `X-Force-Refresh: true` atau tombol "Sync / Refresh Now" di UI mem-bypass cache, memaksa DuckDB query ulang ke target DB, dan memperbarui cache Redis.

### 3.7 Scheduled Reports & Automated Alerts Engine (Point 2)
- **Asynq Cron Scheduler:**
  - Worker membaca tabel `dashboard_alerts` setiap menit.
  - Menjalankan cron expression (misal: `0 8 * * *` untuk jam 08:00 pagi setiap hari).
- **Dual Engine Execution:**
  1. **Scheduled Digest:** Merender snapshot data dashboard/widget, mengekspor format PDF/Excel, lalu mengirim via bot Telegram, channel Slack, atau Email.
  2. **Threshold Alert:** Menjalankan query DuckDB, mengevaluasi angka agregat terhadap operator (`>`, `<`, `==`), dan langsung memicu notifikasi darurat jika ambang batas terlampaui.

### 3.8 Reusable Semantic Layer & Virtual Models (Point 3)
- **Virtual Views In-Memory:**
  - Pengguna mendefinisikan query bisnis yang rumit (misal: `dataset_performa_agen`) di tabel `semantic_models`.
  - Saat sesi DuckDB diinisialisasi, seluruh model aktif di-mount sebagai temporary view:
    ```sql
    CREATE TEMP VIEW dataset_performa_agen AS (SELECT ... 40 baris query ...);
    ```
  - Widget visual builder dan Monaco SQL editor dapat langsung melakukan query sederhana di atas virtual view tersebut: `SELECT * FROM dataset_performa_agen WHERE channel = 'whatsapp'`.

### 3.9 Granular RBAC & Row-Level Security (RLS) (Point 4)
- **Role Hierarchy:**
  - `OWNER` / `ADMIN`: Akses penuh ke koneksi database, AI settings, billing, dan manajemen user.
  - `ENGINEER` / `EDITOR`: Dapat membuat dashboard, membuat semantic model, mengedit Visual Builder & Monaco SQL.
  - `VIEWER`: Hanya dapat melihat canvas, mengubah filter interaktif, dan mengekspor data.
- **Row-Level Security (RLS) Query Injection:**
  - Middleware mencocokkan atribut user (`attributes` di tabel `users` seperti `branch_id`, `team`) dengan aturan di `project_members.rls_rules`.
  - Go AST Parser secara otomatis menyisipkan klausa pembatas ke query sebelum dieksekusi:
    ```sql
    -- Query asli:
    SELECT * FROM orders WHERE status = 'PAID'
    -- Setelah di-inject RLS untuk user cabang Surabaya:
    SELECT * FROM orders WHERE status = 'PAID' AND (branch_id = 'SUB_01')
    ```

### 3.10 Public Shareable Links & Signed Embedded Analytics (Point 5)
- **Public Read-Only URL:**
  - Dashboard dapat dibagikan dengan token acak berkriptografi tinggi (`/share/:shareToken`).
  - Opsional dilindungi proteksi password (bcrypt) dan tanggal kedaluwarsa.
- **Signed JWT Embed (iFrame):**
  - Backend menyediakan endpoint untuk men-generate signed JWT token.
  - Web eksternal (misal portal internal perusahaan) dapat menyematkan widget/dashboard via `<iframe>` dengan validasi Content Security Policy (`frame-ancestors` yang dicocokkan dengan `allowed_domains`).

### 3.11 Query Guardrails & Circuit Breaker (Point 6)
- **Automatic LIMIT Injection:**
  - Jika query dari Visual Builder atau pengguna di Monaco SQL tidak menyertakan klausa `LIMIT`, Go AST parser otomatis menyuntikkan `LIMIT 10000` untuk mencegah browser crash akibat rendering jutaan titik data ECharts.
- **Strict Query Execution Timeout:**
  - Seluruh eksekusi DuckDB diikat oleh `context.WithTimeout(ctx, 30*time.Second)`.
  - Jika query memakan waktu lebih dari batas toleransi, koneksi DuckDB dibatalkan secara elegan tanpa membuat server macet.

---

## 4. API Endpoints Contract

All responses conform to `pkg/httputil.BaseResponse`:
```json
{
  "reqId": "uuid",
  "status": "success",
  "message": "Operation successful",
  "data": {}
}
```

### 4.1 Authentication & Profile
- `POST /api/v1/auth/register` (Register new user & create initial workspace)
- `POST /api/v1/auth/login` (Login with email/password, returns JWT)
- `GET /api/v1/auth/me` (Get current user profile, attributes & workspace list)

### 4.2 Workspaces & Projects
- `POST /api/v1/workspaces` (Create workspace)
- `GET /api/v1/workspaces/:wsSlug` (Get workspace details)
- `GET /api/v1/workspaces/:wsId/projects` (List projects in workspace)
- `POST /api/v1/workspaces/:wsId/projects` (Create project: e.g. Omnichannel, Chatbot)
- `GET /api/v1/workspaces/:wsId/projects/:projId/members` (List members & RLS rules)
- `POST /api/v1/workspaces/:wsId/projects/:projId/members` (Assign member & set RLS rules)

### 4.3 Data Sources & Multi-Environment
- `POST /api/v1/workspaces/:wsId/connections/test` (Test connection with Direct Connect or SSH)
- `POST /api/v1/workspaces/:wsId/connections` (Save connection & encrypted custom env profiles)
- `GET /api/v1/workspaces/:wsId/connections` (List connections & env status)
- `GET /api/v1/workspaces/:wsId/schema?env=production` (Get unified schema tree)

### 4.4 Query, Caching & AI Engine
- `POST /api/v1/workspaces/:wsId/query/execute` (DuckDB query with parameterized bindings & Redis cache)
- `POST /api/v1/workspaces/:wsId/query/refresh` (Invalidate cache & force re-query)
- `POST /api/v1/workspaces/:wsId/ai/generate-dashboard` (Prompt-to-dashboard generator)

### 4.5 Semantic Models (Virtual Datasets)
- `GET /api/v1/workspaces/:wsId/projects/:projId/semantic-models` (List saved virtual models)
- `POST /api/v1/workspaces/:wsId/projects/:projId/semantic-models` (Create virtual model)
- `PUT /api/v1/workspaces/:wsId/semantic-models/:modelId` (Update model SQL & refresh schema)
- `DELETE /api/v1/workspaces/:wsId/semantic-models/:modelId` (Delete virtual model)

### 4.6 Dashboards & Widgets
- `GET /api/v1/workspaces/:wsId/projects/:projId/dashboards` (Catalog list)
- `POST /api/v1/workspaces/:wsId/projects/:projId/dashboards` (Create dashboard: AI, Template, Blank)
- `GET /api/v1/workspaces/:wsId/dashboards/:dashId` (Get dashboard & widgets)
- `PUT /api/v1/workspaces/:wsId/dashboards/:dashId` (Update layout with debounced auto-save & cache TTL)
- `POST /api/v1/workspaces/:wsId/dashboards/:dashId/widgets` (Add widget)
- `PUT /api/v1/workspaces/:wsId/widgets/:widgetId` (Update widget visual/SQL)
- `DELETE /api/v1/workspaces/:wsId/widgets/:widgetId` (Delete widget)

### 4.7 Scheduled Alerts & Digest Engine
- `GET /api/v1/workspaces/:wsId/dashboards/:dashId/alerts` (List alerts for dashboard)
- `POST /api/v1/workspaces/:wsId/dashboards/:dashId/alerts` (Create digest or threshold alert)
- `PUT /api/v1/workspaces/:wsId/alerts/:alertId` (Update alert parameters / schedule)
- `DELETE /api/v1/workspaces/:wsId/alerts/:alertId` (Delete alert)
- `POST /api/v1/workspaces/:wsId/alerts/:alertId/test` (Trigger test alert notification)

### 4.8 Public Sharing & Embedded Analytics
- `POST /api/v1/workspaces/:wsId/dashboards/:dashId/shares` (Generate shareable link or iframe embed token)
- `GET /api/v1/workspaces/:wsId/dashboards/:dashId/shares` (List active shares)
- `DELETE /api/v1/workspaces/:wsId/shares/:shareId` (Revoke share token)
- `GET /api/v1/public/dashboards/:shareToken` (Public unauthenticated view with optional password verify)

### 4.9 Exports & Settings
- `POST /api/v1/workspaces/:wsId/export` (Stream or enqueue CSV, JSON, XLSX)
- `GET /api/v1/workspaces/:wsId/exports` (Export audit history)
- `GET /api/v1/workspaces/:wsId/settings/ai` (Get AI provider & model settings)
- `PUT /api/v1/workspaces/:wsId/settings/ai` (Save AI settings & business glossary)
- `POST /api/v1/workspaces/:wsId/settings/ai/test` (Test LLM connectivity)
