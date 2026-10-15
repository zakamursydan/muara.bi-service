# PROGRESS TRACKER & STATE CONTINUITY — `muara.bi`

> **PENTING UNTUK AGENT AI / CHAT BARU / AKUN LAIN:**  
> Selalu baca file ini (`PROGRESS.md`), `PRD.md`, dan `PLAN.md` setiap kali sesi baru dimulai untuk mengetahui titik terakhir pengembangan tanpa mengulang dari awal.

---

## 1. Project Identity & Environment Reference

- **Product Name:** `muara.bi` (Unified BI & AI Analytics Platform)
- **Backend Path:** `d:\GO\muara.bi\backend` (Go module: `muara.bi/core`)
- **Backend Architecture Reference:** `D:\GO\go-map-service` (`pkg/container`, `internal/app/server`, `repository`, `service`, `handler`, `asynq` worker queue, `duckdb` engine).
- **Frontend Path:** `d:\GO\muara.bi\frontend` (Package: `@muara/web` — Next.js 15+ App Router, Tailwind CSS, Shadcn UI, Apache ECharts, Monaco Editor, Zustand).
- **Go Toolchain Path (Windows):** `C:\Users\User\go\pkg\mod\golang.org\toolchain@v0.0.1-go1.27.1.windows-amd64\bin\go.exe`
  *(Set session PATH via: `$env:PATH = "C:\Users\User\go\pkg\mod\golang.org\toolchain@v0.0.1-go1.27.1.windows-amd64\bin;" + $env:PATH`)*
- **Docker Compose:** `d:\GO\muara.bi\docker-compose.yml` (PostgreSQL 16, Redis 7, MongoDB 7, Postgres sample).
- **Core Principles:**
  1. Dynamic Custom Environment Tags (`env_tag VARCHAR(50)`), tidak di-hardcode ke prod/staging/dev.
  2. Direct Connect DB UI sebagai default (Host/IP, Port, DB Name, User, Password); SSH Bastion Tunnel opsional & collapsible.
  3. AI Co-Pilot **hanya membaca metadata** (DDL skema, kolom, tipe data, relasi foreign key ERD); **TIDAK PERNAH membaca baris data transaksi asli**.
  4. 6 Enterprise Pillars: Smart Query Caching (Redis), Scheduled Reports & Alerts (Telegram/Slack/Email), Reusable Semantic Models (DuckDB Temp Views), RBAC & Row-Level Security (RLS AST injection), Public Sharing & Signed JWT Embed, Query Guardrails (Auto LIMIT 10000, 30s timeout).

---

## 2. Template Prompt Lanjutan (Copy-Paste untuk Chat / Akun Baru)

Jika Anda berganti model atau berganti akun, cukup copy-paste teks ini ke prompt pertama:

```text
Halo, saya melanjutkan pengembangan platform muara.bi.
Standard arsitektur backend meniru D:\GO\go-map-service.
Tolong baca @PRD.md, @PLAN.md, dan @PROGRESS.md untuk memahami arsitektur dan state terakhir.
Saat ini kita berada di [Phase X - Step Y]. Lanjutkan eksekusi task berikutnya sesuai checklist di PROGRESS.md!
```

---

## 3. Status Terkini (Current Snapshot)

- **Fase Aktif:** Phase 2 (Distributed DuckDB Engine, Smart Caching, Connectors & Multi-Worker)
- **Subtask Saat Ini:** Setup DuckDB CGO session factory, AST parser guardrails, dan Redis query caching.
- **Subtask Berikutnya:** NoSQL MongoDB in-memory adapter, SSH bastion pool, dan Asynq worker queue.

---

## 4. Phase-by-Phase Execution Checklist

### Phase 1: Go Foundation, Clean Architecture & PostgreSQL Migrations
- [x] Desain Arsitektur & Mockup Interaktif (`PRD.md`, `PLAN.md`, Mockup HTML & Visual).
- [x] Sinkronisasi Skema 14 Tabel & 6 Fitur Enterprise di `PRD.md` dan `PLAN.md`.
- [x] Setup `docker-compose.yml` (PostgreSQL metadata, Redis queue & cache, MongoDB sample, Postgres sample).
- [x] Inisialisasi File Pelacak Kontinuitas (`PROGRESS.md`).
- [x] Inisialisasi Go Module `muara.bi/core` di `/backend` (`go mod init muara.bi/core`).
- [x] Copy & Adaptasi Library `pkg/` dari `go-map-service`:
  - [x] `pkg/container` (Lifecycle management: BeforeStart, AfterStart, Shutdown).
  - [x] `pkg/logging` (Structured slog JSON logger & log levels).
  - [x] `pkg/envparser` (Env loader & struct tag parser).
  - [x] `pkg/httputil` (BaseResponse `{reqId, status, message, data}`, error handlers).
  - [x] `pkg/customerror` (Domain error parser & locator).
  - [x] `pkg/telemetry` (OpenTelemetry provider).
- [x] Implementasi `internal/config/config.go` & file `.env.example`.
- [x] Implementasi Goose Migration `internal/generated/migrations/00001_initial_schema.sql` (14 tabel lengkap).
- [x] Implementasi `internal/app/server`:
  - [x] `internalconnection.go` (PostgreSQL pool pgx/v5 & Redis client).
  - [x] `migration.go` (Goose migration runner).
  - [x] `server.go` (Chi router, middleware stack, healthcheck).
  - [x] `repository.go` dan `service.go` (Dependency injection).
- [x] Implementasi Modul Autentikasi & RLS:
  - [x] `user_repo.go` (CRUD users & workspace_members).
  - [x] `auth_service.go` (Bcrypt password hashing, JWT token generator).
  - [x] `auth_handler.go` (`/auth/register`, `/auth/login`, `/auth/me`).
  - [x] `jwt_middleware.go` (Bearer token parser & context injector).
- [x] Implementasi `internal/helper/cryptohelper` (AES-256-GCM encryption/decryption master key).
- [x] Uji coba build `cmd/api/main.go` (Kompilasi sukses tanpa error: `go build ./...` exit 0).

---

### Phase 2: Distributed DuckDB Engine, Smart Caching, Connectors & Multi-Worker
- [ ] Ephemeral in-memory DuckDB session factory (`internal/duckdb/session.go`).
- [ ] AST SQL Parser guardrails (`internal/duckdb/guardrails.go`: Auto-LIMIT 10000, 30s timeout, anti-DDL/DML).
- [ ] Smart Query Caching di Redis (`internal/cache/query_cache.go` dengan `X-Force-Refresh`).
- [ ] Reusable Semantic Models / Virtual Views (`internal/duckdb/semantic_layer.go`).
- [ ] NoSQL Streaming In-Memory Adapter untuk MongoDB (`internal/duckdb/nosql_adapter.go`).
- [ ] Strict Parameterized Query Binding (`$1, $2`) untuk `{{param}}` & cross-filter.
- [ ] Dynamic SSH Bastion Tunnel pool (`internal/ssh`).
- [ ] Distributed Asynq Worker Pool (`cmd/worker/main.go`, `query_worker.go`, `export_worker.go`, `alert_worker.go`).
- [ ] Connection, Schema, Query, dan Semantic Model CRUD Repositories & Services.

---

### Phase 3: Dashboard Domain, RBAC/RLS, Sharing & Alerts Engine
- [ ] Native DuckDB streaming CSV & JSON exporter.
- [ ] Multi-sheet Excel (`.xlsx`) exporter via `excelize` (1 widget = 1 sheet, auto styling).
- [ ] Project & Dashboard CRUD dengan cache TTL settings.
- [ ] Widget CRUD dengan query mode `SYNCED` vs `CUSTOM_SQL`.
- [ ] Granular RBAC & RLS per project (`project_members`).
- [ ] Public Share Link & Signed JWT Embedded Analytics (`/share/:token`, `/embed`).
- [ ] Scheduled Digest & Threshold Alerts Dispatcher (Telegram, Slack, Email, Webhook).
- [ ] AI Co-Pilot Service (Semantic Schema Pruning & PII Masking, Prompt-to-Dashboard).

---

### Phase 4: Next.js Frontend (`@muara/web`) - Light Off-White Theme
- [ ] Init Next.js 15+ App Router, Tailwind CSS, Shadcn UI (`#f8fafc` base theme).
- [ ] Zustand stores (`auth`, `env`, `canvas`, `filter`, `schema`, `project`).
- [ ] Global Shell: Left Navigation Rail & Header (Workspace, Project, Env Switcher, Sync Button).
- [ ] Dashboard Catalog & Create Wizard Modal (AI Co-Pilot, Template, Blank).
- [ ] 12-Column Responsive Canvas (`react-grid-layout`) + ECharts (`ResizeObserver`).
- [ ] Anti-self-loop Compound Cross-Filtering & Dynamic Parameter Filter Bar.
- [ ] 3-Layer Slide-Up Drawer (AI Prompt Bar, Visual Builder, Monaco SQL Editor).
- [ ] Semantic Models Studio & Data Studio query scratchpad.
- [ ] Data Sources UI (Direct Connect default, collapsible SSH Bastion).
- [ ] Public Share & Embedded Canvas views.
- [ ] AI Co-Pilot Settings UI.

---

### Phase 5: End-to-End Integration, Docker Packaging & Testing
- [ ] Production multi-stage `Dockerfile` with CGO and DuckDB extensions.
- [ ] E2E cross-database federated query tests (Postgres + MongoDB).
- [ ] Concurrency load tests (Redis semaphores & Asynq priority queues).
- [ ] Cache & RLS verification tests.
- [ ] Alert dispatching verification (Telegram & Slack).
