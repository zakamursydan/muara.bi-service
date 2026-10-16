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

- **Update Terakhir:** 2026-10-16
- **Fase Aktif:** Phase 3 (Dashboard Domain, RBAC/RLS, Sharing & Alerts Engine)
- **Subtask Saat Ini:** Public Share Link & Signed JWT Embed — **kode selesai dan build hijau**, sisa unit test + `APP_PUBLIC_BASE_URL` di `.env.example`.
- **Subtask Berikutnya:** Selesaikan test share → Scheduled Digest & Threshold Alerts Dispatcher → AI Co-Pilot Service.
- **Kondisi Build Terakhir yang Terverifikasi:** Hijau (`go build ./...`, `go vet ./...`, `gofmt -l` bersih, `go test ./...`) setelah Public Sharing terpasang. Test suite: `cache`, `duckdb`, `exporter`, `middleware`, `service`.

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
- [x] Ephemeral in-memory DuckDB session factory (`internal/duckdb/session.go`, `session_cgo.go`, `session_fallback.go`).
- [x] AST SQL Parser guardrails (`internal/duckdb/guardrails.go`: Auto-LIMIT 10000, 30s timeout, anti-DDL/DML).
- [x] Smart Query Caching di Redis (`internal/cache/query_cache.go` dengan `X-Force-Refresh`).
- [x] Reusable Semantic Models / Virtual Views (`internal/duckdb/semantic_layer.go`).
- [x] NoSQL Streaming In-Memory Adapter untuk MongoDB (`internal/duckdb/nosql_adapter.go`).
- [x] Strict Parameterized Query Binding (`$1, $2`) untuk `{{param}}` & cross-filter.
- [x] Dynamic SSH Bastion Tunnel pool (`internal/ssh/bastion.go`).
- [x] Distributed Asynq Worker Pool (`cmd/worker/main.go`, `query_worker.go`, `export_worker.go`, `alert_worker.go`).
- [x] Connection, Schema, Query, dan Semantic Model CRUD Repositories & Services.

---

### Phase 3: Dashboard Domain, RBAC/RLS, Sharing & Alerts Engine
- [x] Streaming CSV & JSON exporter (`internal/exporter/csv.go`, `json.go` — baris demi baris, tidak mem-buffer seluruh hasil).
  - Catatan: memakai writer Go di atas `duckdb.QueryResult`, **bukan** `COPY ... TO`, karena `COPY TO` memang diblokir guardrail anti-file-write.
- [x] Multi-sheet Excel (`.xlsx`) exporter via `excelize` `StreamWriter` (1 widget = 1 sheet, header styled brand cyan, auto column width, sanitasi + dedup nama sheet) — `internal/exporter/xlsx.go`.
- [x] `internal/exporter/exporter_test.go` (CSV, JSON, XLSX 3 sheet, truncation nama sheet) — `go test` hijau.
- [x] Refactor worker agar memakai exporter & multi-sheet: `internal/worker/export_worker.go`, `tasks.go` (`ExportJobPayload.Sheets []ExportSheetSpec`), `server.go`, `cmd/worker/main.go`.
- [x] `internal/service/session_preparer.go` — satu jalur eksekusi bersama untuk API & worker (guardrail → `{{param}}` → `$n` → RLS → auto-LIMIT → attach koneksi per `env_tag` → mount semantic view). `query_service.go`, `query_worker.go`, `alert_worker.go` sudah dipindah ke preparer.
- [x] **Project & Dashboard CRUD dengan cache TTL settings**:
  - [x] Entity `internal/domain/entity/project.go` (Project, ProjectMember, RLSRuleSpec, Dashboard, Widget + konstanta role & query mode).
  - [x] Repository `internal/repository/pgsql/project_repository.go`, `dashboard_repository.go`, `widget_repository.go` (widget di-scope ke tenant via JOIN dashboards).
  - [x] DTO `internal/dto/request/dashboard.go`, `internal/dto/response/dashboard.go`.
  - [x] Service `project_service.go`, `dashboard_service.go` (error domain sudah pakai `pkg/customerror`; `CreateProject` memetakan unique violation Postgres 23505 → 409 via `internal/helper/dberr`).
  - [x] Helper `internal/helper/sqlident` — whitelist identifier SQL (dipakai RLS & builder; netral dari import cycle).
  - [x] Handler `project_handler.go`, `dashboard_handler.go`, `widget_handler.go`.
  - [x] `handler/errors.go` — pemetaan `customerror.AppError` → 400/401/403/404/409/500 + `DecodeJSON`; `httputil.WriteConflictResponse` ditambahkan.
  - [x] **Refactor routing selesai:** `server.go` punya SATU subrouter `/api/v1/workspaces/{wsId}`; semua handler (connection, query, semantic, project, dashboard, widget) memakai path relatif tanpa `r.Route()` bersarang, sehingga chi mount conflict hilang.
  - [x] Registrasi repo & service baru di `internal/app/server/repository.go` dan `service.go`.
  - [x] Verifikasi `go build ./... && go vet ./... && go test ./...` — hijau, `gofmt -l` bersih.
- [x] **Widget CRUD dengan query mode `SYNCED` vs `CUSTOM_SQL`**:
  - [x] Entity `internal/domain/entity/widget_builder.go` (WidgetBuilderState, BuilderMetric, BuilderFilter).
  - [x] Compiler `internal/service/widget_builder.go` — `SYNCED` selalu dikompilasi ulang dari `builder_state` (identifier divalidasi, nilai filter jadi placeholder `{{flt_<kolom>}}`, tidak ada konkatenasi nilai user); `CUSTOM_SQL` wajib lolos `duckdb.ValidateQuery`.
  - [x] `internal/service/widget_builder_test.go` — 11 kasus injeksi (stacked statement, subquery, comment, operator/alias/identifier ilegal) semuanya ditolak; nilai filter terbukti tidak pernah bocor ke teks SQL.
  - [x] Service `internal/service/widget_service.go` (whitelist chart_type, default `max_rows` 10000 / cap 100000, default TTL 300).
  - [x] Handler + routing + wiring.
- [x] **Granular RBAC & RLS per project (`project_members`)**:
  - [x] Model permission `internal/domain/entity/permission.go` — 10 permission + matriks role workspace (OWNER/ADMIN penuh, ENGINEER = EDITOR + `connection:write`, EDITOR builder, VIEWER read-only) dan role project (ADMIN/EDITOR/VIEWER). Role tak dikenal → **nol permission** (fail closed); `PermissionsFor*Role` mengembalikan salinan supaya matriks tidak bisa termutasi.
  - [x] `internal/service/access_service.go` — `Resolve(Scope)` meng-union grant workspace + project sekaligus menurunkan `RLSRules`. Aturan fail-closed: (1) bukan anggota workspace → tolak; (2) route project-scoped tapi bukan anggota project → tolak kecuali OWNER/ADMIN workspace; (3) project tidak berada di workspace pada URL → tolak (mencegah grant lintas tenant); (4) hanya OWNER/ADMIN workspace yang bebas dari RLS.
  - [x] `internal/service/rls.go` — `ParseRLSRuleSpecs` + `ResolveRLSRules`: `value_from_user` diambil dari `users.attributes` (via klaim JWT), atribut hilang / `null` / non-skalar → **403**, bukan dilewati diam-diam.
  - [x] `duckdb.ApplyRLS` **diperkeras**: signature jadi `(string, []any, error)`, kolom wajib lolos `sqlident.IsQualified`, operator dibatasi whitelist `= != > < >= <=`, nilai selalu `$n`. Rule invalid → `ErrInvalidRLSRule`, tidak lagi silently skip.
  - [x] `internal/middleware/rbac.go` — `Guard.Require(permission)` dipasang inline per route (`r.With(...)`, karena middleware inline chi berjalan **setelah** URL param terisi). Scope project diturunkan otomatis dari `{projId}`, `{dashId}` (via `GetProjectIDByDashboard`) atau `{widgetId}` (via `GetProjectIDByWidget`); `Access` hasil resolve ditaruh di context.
  - [x] Semua route connection/query/semantic/project/dashboard/widget kini dijaga Guard dengan permission yang sesuai.
  - [x] **RLS masuk ke jalur eksekusi**: `QueryService` me-resolve access, menolak tanpa `query:execute`, dan menyuntikkan `RLSRules` ke `PreparedQuery`. `service.QueryCacheScope{Parameters, RLS}` ikut di-hash ke cache key — dua user dengan filter baris berbeda tidak akan pernah saling memakai hasil cache.
  - [x] **Celah RLS di worker ditutup**: `query_worker` sebelumnya mengabaikan `UserAttributes` dan menjalankan query tanpa filter; `export_worker` juga tanpa RLS. Keduanya kini menerima `AccessServiceProvider`, me-resolve ulang rule dari `UserID` + `UserAttributes` di payload (`tasks.go` diperluas), dan menolak job bila user tidak berhak. `worker/server.go` + `cmd/worker/main.go` ikut di-wire.
  - [x] Test: `internal/service/access_service_test.go` (16 kasus: union role, penolakan non-member, penolakan project lintas workspace, RLS admin-exempt, fail-closed atribut hilang, salinan permission set) dan `internal/middleware/rbac_test.go` (6 kasus: 200/403, scope dari `{projId}`/`{dashId}`/`{widgetId}`, atribut JWT terbawa, `Access` muncul di context) — hijau.
  - Catatan: `alert_worker` sengaja **tanpa RLS** karena alert/digest dipicu sistem dan dikirim ke penerima terkonfigurasi, bukan ke satu user.
- [ ] Public Share Link & Signed JWT Embedded Analytics (`/share/:token`, `/embed`).
  - [x] Migration `internal/generated/migrations/00002_dashboard_share_env_tag.sql` — kolom `env_tag` di `dashboard_shares`. **Alasan keamanan:** tanpa ini endpoint publik harus menerima `env_tag` dari pemanggil anonim, sehingga siapa pun yang memegang share token bisa mengarahkan query ke kredensial environment lain.
  - [x] Entity `internal/domain/entity/share.go` (`DashboardShare`, `AccessModePublicLink` / `AccessModeEmbedIFrame`, `IsExpired` / `IsAccessible`).
  - [x] DTO `internal/dto/request/share.go` (`CreateShareRequest`, `PublicWidgetQueryRequest`) dan `internal/dto/response/share.go` (`ShareResponse`, `PublicWidgetResponse`, `PublicDashboardResponse`).
  - [x] Repository `internal/repository/pgsql/share_repository.go` — semua lookup tenant-scoped lewat `JOIN dashboards`; hanya `GetShareByToken` yang tanpa filter workspace karena token adalah capability-nya.
  - [x] Service `internal/service/share_service.go`:
    - Token share 256-bit dari `crypto/rand` → `base64.RawURLEncoding` (43 char).
    - Password opsional (min. 8 char) di-hash bcrypt; `expires_at` wajib di masa depan.
    - `normalizeAllowedDomains` mereduksi tiap entri jadi origin `scheme://host` absolut; wildcard, path, query, userinfo ditolak; non-localhost wajib `https`.
    - `EMBED_IFRAME` wajib punya minimal satu allowed domain.
    - Signed JWT embed HS256 (`EmbedClaims`: share_token, dashboard_id, allowed_domains, env_tag) — masa berlaku **tidak pernah melewati `expires_at` share**; `VerifyEmbedToken` menolak non-HMAC, token invalid/expired, dan issuer salah.
    - `ResolvePublic` mengembalikan `password_required: true` **tanpa konten apa pun** bila share diproteksi dan password belum dikirim; password salah → 401.
    - `QueryPublicWidget` membaca SQL dari baris widget tersimpan dan `env_tag` dari baris share — pemanggil anonim **tidak bisa mengirim SQL maupun env_tag**, hanya nilai filter yang diikat sebagai `$n`. Widget wajib milik dashboard pada share. Timeout guardrail 30s. Tanpa RLS (viewer anonim, bukan user).
  - [x] Permission baru `entity.PermSharePublish` (`share:publish`) — hanya OWNER/ADMIN workspace dan ADMIN project. **Alasan:** mempublikasikan data ke luar tenant lebih berat daripada mengedit dashboard, jadi EDITOR/ENGINEER (yang punya `dashboard:write`) tidak boleh membuat/melihat/mencabut share.
  - [x] Handler `internal/handler/share_handler.go` (terjaga Guard) — route sengaja **bersarang di bawah `{dashId}`** (`POST|GET /dashboards/{dashId}/shares`, `DELETE /dashboards/{dashId}/shares/{shareId}`) agar scope project diturunkan dari dashboard yang sudah didukung Guard; tidak perlu reader project-per-share baru. `DeleteShare` ikut di-scope `dashboard_id` supaya share dashboard lain di workspace yang sama tidak bisa terhapus.
  - [x] Handler `internal/handler/public_handler.go` (tanpa JWT) di-mount pada `/api/v1/public`: `GET /dashboards/{shareToken}`, `POST /dashboards/{shareToken}/widgets/{widgetId}/query`, `GET /embed?token=<jwt>`. Password dibaca dari header `X-Share-Password`.
    - Respons publik mengirim header `Content-Security-Policy: frame-ancestors <allowed_domains>` (atau `'none'` + `X-Frame-Options: DENY` bila share tidak punya allowlist). `PublicWidgetResponse` **sengaja tidak memuat `raw_sql` maupun `builder_state`** agar skema internal tidak bocor.
    - `/embed` memverifikasi JWT, menolak share yang bukan `EMBED_IFRAME`, dan menolak origin yang tidak ada di allowlist (defence-in-depth; kontrol utamanya tetap CSP di browser).
  - [x] Wiring: `ShareRepo` di `repository.go`, `ShareService` di `service.go`, route di `server.go`; config baru `APP_PUBLIC_BASE_URL` (`config.go`) untuk menyusun `share_url`.
  - [ ] **Sisa:** `APP_PUBLIC_BASE_URL` belum ditambahkan ke `backend/.env.example`.
  - [ ] **Sisa:** unit test share (entropi/panjang token, kasus terima/tolak `normalizeAllowedDomain`, proteksi password & expiry, sign/verify embed JWT + cek issuer).
  - [ ] **Sisa (Phase 4, frontend):** halaman Next.js `/share/[token]` dan `/embed` harus memancarkan header CSP `frame-ancestors` dari field `frame_ancestors` — backend tidak bisa men-set CSP untuk dokumen HTML yang disajikan frontend.
- [ ] Scheduled Digest & Threshold Alerts Dispatcher (Telegram, Slack, Email, Webhook).
  - Catatan: `alert_worker.go` sudah punya evaluator threshold + dispatcher inline Telegram/Slack/Webhook (EMAIL belum, belum ada config SMTP); belum ada repo/service/handler `dashboard_alerts`, belum ada cron scheduler Asynq, belum ada `digest_worker` (`TypeDigestDispatch` + `DigestDispatchPayload` sudah didefinisikan di `tasks.go`).
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
