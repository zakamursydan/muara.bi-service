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

- **Update Terakhir:** 2026-11-01
- **Fase Selesai:** Phase 1, Phase 2, Phase 3 (100% Backend Complete), Phase 4 (100% Frontend) & Phase 5 (E2E Integration, Docker Packaging & Testing), plus refactor lintas-produk **M1** (2026-10-31)
- **Fase Aktif:** tidak ada di repo ini sampai spike **M0 desktop** dimulai (lihat `muara.bi-desktop/PLAN.md`) — engine analitik sudah keluar `internal/` dan dipakai ulang oleh muara.bi Desktop tanpa salinan kedua. Phase 1–5 tetap `[x]`.
- **Subtask Saat Ini:** **M1 selesai 2026-10-31.** M1.1–M1.5 tuntas, plus bug DSN Postgres lama (`sslmode`
  ganda) ditutup dan `ssl_mode` kini divalidasi (400, bukan gagal koneksi); exit criterion terakhir dibuktikan
  `make image-core` (blok M1.3 di bawah). Langkah berikutnya bukan di repo ini: spike **M0 desktop**
  (Wails v3 × CGO × offline di tiga OS). Yang masih menggantung di web: uji HTTP validasi `ssl_mode` (butuh
  `muara_api` dibangun ulang — infra milik user) dan `rootcert`/CA privat (blocker 8 BRD desktop).
  Update 2026-11-01 di repo ini hanya soal memori: `NormalizeMaxMemory` kini menerima unit biner DuckDB
  (`KiB|MiB|GiB|TiB`), **default `DUCKDB_MAX_MEMORY` pindah ke `2GiB`** (keputusan user, konsisten dengan
  desktop), dan tiga catatan M1.5 yang salah dikoreksi — lihat blok M1.5 di §3.
- **Kondisi Backend Terakhir yang Terverifikasi:**
  - Public Sharing & Signed JWT Embed teruji & terpasang.
  - Scheduled Digest & Threshold Alerts Dispatcher teruji & terhubung ke Asynq scheduler.
  - AI Co-Pilot Service (Pruning, PII Masking, LLM Adapter, DuckDB SQL & Dashboard generation) teruji & ter-wire ke chi router.
  - Repository, Service, Handler, dan Router selesai 100%.
  - Perbaikan backend saat Phase 4: nama semantic model divalidasi sebagai plain identifier sebelum `CREATE TEMP VIEW` (menutup injeksi identifier), `statement_timeout` DuckDB diganti deadline per-query via context, dan `PUT /ai/settings` dengan `api_key` kosong tidak lagi menghapus API key tersimpan.
  - Perbaikan backend saat smoke test Phase 4:
    - `ATTACH` kini membungkus alias dengan `quoteIdent` dan DSN/path dengan `quoteLiteral`, sehingga alias bersimbol (mis. `dwh-sample`) tidak lagi gagal parser dan kredensial berisi tanda kutip tidak bisa memutus teks DDL. `alias_name` divalidasi `sqlident.IsPlain` saat pembuatan koneksi (400 dengan pesan jelas) karena nama alias adalah nama katalog DuckDB.
    - Kegagalan `ATTACH` tidak lagi hanya jadi warning: bila semua sumber yang punya profil pada environment aktif gagal di-attach, query mengembalikan 400 alih-alih "Catalog Error: table does not exist" yang menyesatkan.
    - Nilai `DECIMAL` dari driver DuckDB tidak lagi di-serialisasi sebagai objek `{Width,Scale,Value}`; kini diluruskan ke angka via `jsonValue`, dan result set kosong mengirim `[]` (bukan `null`).
    - `cached_schema` disimpan sebagai JSONB polos (`json.Marshal` + error diteruskan), bukan blob terenkripsi yang gagal senyap.
  - Perbaikan frontend saat smoke test Phase 4: widget baru dari Data Studio mendapat slot 6×6 (bukan default 1×1 `react-grid-layout`) lalu di-persist oleh autosave; dialog share mengirim origin absolut (host tanpa skema dianggap `https://`) sesuai kontrak validator API.
  - Upgrade widget datapabel (`table`): `DataTable` kini punya sort per kolom (klik header, asc → desc → lepas), paginasi 10/25/50/100 baris dengan footer "x–y dari z", nomor baris opsional, kolom numerik rata-kanan + pemisah ribuan, tanggal dipangkas ke `YYYY-MM-DD[ HH:MM]` dari teks ISO driver (tidak lewat `Date` agar tidak bergeser ke timezone browser), nilai `null` tampil `—`, dan densitas compact/renggang.
  - Warna kustom widget datapabel disimpan di `echarts_config.table` (JSONB opaqu, tanpa migrasi): 6 slot (`header_bg`, `header_fg`, `body_bg`, `stripe_bg`, `text_fg`, `accent`) + 5 preset di drawer inspector. Nilai divalidasi hex ketat di `lib/table-theme.ts` sebelum masuk ke CSS custom property — config widget diputar ulang di halaman share anonim, jadi string seperti `url(...)` tidak boleh mencapai CSS. Slot yang kosong mengikuti token tema sehingga dark mode tetap terbaca.
  - Bug lama yang ikut diperbaiki: tombol "Edit widget" di kartu tidak pernah membuka drawer (prop `onEditWidget` tidak pernah di-pass), dan drawer selalu terbuka di tab builder sehingga widget `CUSTOM_SQL` gagal disimpan; `grid.tsx` kini membuka inspector pada tab sesuai `query_mode` widget.
  - Upgrade UI chart (global, hanya di builder `lib/echarts-options.ts`): tick sumbu Y ringkas (`315000` → `315K`), split line putus-putus, bar bersudut membulat + gradasi vertikal halus + `barMaxWidth`, garis 2,5px `cap: round` dengan area gradasi, legenda `type: scroll` berikon roundRect di tengah atas, tooltip kartu (latar `--card`, radius 10px, bayangan) yang menampilkan satuan, donut dengan total di tengah + nama/persen saat hover, dan tanggal ISO dipangkas pada sumbu kategori. Data, guardrail, dan kontrak API tidak berubah.
  - Palet warna kustom per chart disimpan di `echarts_config.palette` (lima warna, tetap tanpa migrasi): 4 preset + 5 color picker di inspector (`components/inspector/chart-palette-fields.tsx`), tombol "Ikut tema" menghapus key tersebut. Hex divalidasi ketat di `lib/color.ts` (modul baru, dipakai juga oleh `table-theme.ts`) karena config diputar ulang di halaman share anonim; warna tema dibaca lewat `readCanvasTheme()` per render sehingga dark mode ikut.
  - Temuan ECharts 6: `grid.containLabel` sudah deprecated (butuh `LegacyGridContainLabel`, log warning kalau tidak dipasang) → diganti `outerBoundsMode: 'same'` + `outerBoundsContain: 'axisLabel'`; `title` dengan `left: 'center'` + `textAlign: 'center'` ternyata bergeser setengah lebar teks → pakai `left: '50%'`; donut dengan legenda di samping meluap keluar kanvas pada kartu lebar (radius % dihitung dari sisi terkecil) → donut tengah + legenda horizontal di bawah.
  - `temporalKind` dan `formatTemporal` dipindah dari `data-table.tsx` ke `echarts-options.ts` agar aturan pemangkasan tanggal yang sama dipakai datapabel dan sumbu kategori chart.
  - Artefak verifikasi yang perlu diketahui: saat panel Browser Qoder tidak terlihat, `requestAnimationFrame` disuspend sehingga series beranimasi tidak ter-paint (axis, legenda, dan title tetap tampil) — ini bukan bug kode. Untuk mengambil screenshot, `animation: false` dipasang sementara lalu dikembalikan; hasil render diverifikasi dari `canvas.toDataURL()` karena `take_screenshot` menolak surface tersembunyi.
  - Widget demo baru di dashboard `12715684-…`: "Komposisi revenue per channel" (pie, palet Pastel) dan "Tren revenue bulanan" (area 2 series) untuk menguji legenda, donut, dan sumbu tanggal.
  - Catatan dev-only yang belum ditangani: warningcontrolled/uncontrolled pada `<Select value={x ?? undefined}>`, notice Next.js "Dropped segment" untuk `/dashboards/[dashboardId]`, Monaco dimuat dari CDN, kanal alert EMAIL hanya menulis log, dan halaman "Exports & Jobs" belum punya endpoint HTTP.

### Modul engine `muara.bi/engine` (M1.3, 2026-10-31)
- **Batas baru:** `backend/internal/{duckdb,exporter,ssh,ai}` dan `backend/internal/helper/{cryptohelper,sqlident}`
  pindah (semuanya `git mv`, tercatat sebagai rename — tidak pernah ada dua salinan) ke
  `backend/engine/{duckdb,exporter,ssh,ai,cryptohelper,sqlident}`, plus `engine/builder/` yang menggabungkan
  `service/widget_builder.go` + `domain/entity/widget_builder.go` (3 struct builder + kompilatornya,
  `package builder`). Import lama `muara.bi/core/internal/...` sudah nol di seluruh repo.
- **Cara web mengambilnya:** `backend/engine/go.mod` (`module muara.bi/engine`, `go 1.27.1`, `go.sum` sendiri)
  dan di `backend/go.mod`: `require muara.bi/engine v0.0.0` + `replace muara.bi/engine => ./engine`.
  Desktop (belum ada `go.mod`-nya) akan memakai replace ke path yang sama, bukan menyalin.
- **Aturan yang dijaga:** engine tidak boleh mengimpor `muara.bi/core/...` (akan membuat siklus modul);
  diverifikasi `grep -r muara.bi/core backend/engine/` → nol. Dependensi eksternal engine hanya empat:
  `go-duckdb`, `excelize/v2`, `mongo-driver`, `x/crypto/ssh` — tidak ada `pgx`/`chi`/`asynq`/`redis`.
  `internal/helper/dberr` dan `session_preparer.go` sengaja **tinggal** di web (memetakan error Postgres
  dan menempel repository/limiter/logging — orkestrasi, bukan engine).
- **Istilah tenant dihapus dari API engine:** `duckdb.SemanticModel` tidak lagi punya `WorkspaceID`/`ProjectID`
  (satu-satunya `workspace_id` yang tersisa di engine); pemetaan entity → struct engine terjadi di
  `internal/service/session_preparer.go:142`. Ini prasyarat desktop: di sana tidak ada workspace sama sekali.
- **Dua jebakan yang ikut dibetulkan:** (1) modul bersarang membuat `go test ./...` di `backend/` **tidak**
  menyentuh engine lagi — `make test`/`vet`/`tidy` kini menjalankan dua modul, engine lebih dulu;
  (2) `go mod download` dengan `replace` ke folder lokal butuh `go.mod` targetnya sudah ada — `Dockerfile`
  stage `build` sekarang men-COPY `backend/engine/go.mod` + `go.sum` ke `./engine/` sebelum step download.
  **Dibuktikan lewat `make image-core` 2026-10-31** (exit criterion M1 yang terakhir): `go mod download`
  selesai 33 dtk tanpa error `replacement`, tiga binary CGO ter-link (206 dtk), stage extensions mem-bundle
  `postgres_scanner`/`mysql_scanner`/`sqlite_scanner`, dan image `muara-bi-core:dev` 369 MB. Diperiksa dari
  dalam image dengan `go version -m /usr/local/bin/api`: tercantum `dep muara.bi/engine v0.0.0` **tanpa hash**
  (= di-resolve dari `replace => ./engine`, bukan dari registry) bersama `go-duckdb v1.8.5` + `pgx v5.11.0`,
  dan string `DUCKDB_MAX_MEMORY` ada di binary. Jadi image ini memakai engine, bukan salinan keduanya.
  Yang **tidak** dibuktikan build ini: perilaku `DUCKDB_MAX_MEMORY` salah nilai di dalam container, karena
  `NewService` — tempat validasi itu jalan — baru dipanggil setelah Postgres+Redis terhubung
  (`internal/app/server/server.go:55`), jadi butuh infra; validasinya sendiri sudah terbukti di tes engine.
- **Gerbang verifikasi:** `go build`/`go vet`/`go test` hijau di **dua** modul × **dua** mode
  (`CGO_ENABLED=1` dan `CGO_ENABLED=0`), `gofmt -l` bersih. Perilaku CGO fail-fast M1.2 tetap terbukti:
  tanpa CGO, `duckdb.NewEngine` mengembalikan `ErrCGODisabled` dan tes engine lain `t.Skip`.
- **Catatan Phase 5 yang masih berlaku:** verifikasi compose dulu dilakukan tanpa menghentikan dev server
  milik user — infra yang sudah jalan dipakai ulang lewat `--no-deps` dan aplikasi dipublikasikan pada port
  alternatif 18080/18000; `muara-bi-core:dev` + `muara-bi-web:dev` ter-smoke-test, federated query
  Postgres+MongoDB, cache/RLS, budget Redis + bobot Asynq, dan dispatch alert sudah tervalidasi.

### Kontrak pemotongan hasil query (M1.4, 2026-10-31)
- **Kontrak engine baru** (`backend/engine/duckdb/types.go`): `Query(ctx, sql, QueryOptions{MaxRows, CappedSources}, args...)`
  dan `QueryResult` kini membawa `RowLimit`, `Truncated`, `TruncatedBy` (`row_limit` | `source_cap`) + `CappedSources`.
  `MaxRows` <= 0 berarti pakai cap platform `duckdb.DefaultMaxRows` (10.000) — angka yang sama yang dipakai guardrail.
- **Cara deteksinya (bukan menebak "halaman penuh"):** statement dijalankan dengan `LIMIT cap+1` lewat
  `PlanLimit()` di `guardrails.go`; kalau baris ke-(cap+1) benar-benar ada, hasilnya ditandai `row_limit` dan baris
  penanda itu dibuang. 12 baris dengan cap 12 **tidak** ditandai (dibuktikan tes); cap 5 dari 12 ditandai.
  `ApplyLimit()` tetap ada tapi sekarang hanya `PlanLimit(...).SQL`.
- **Konsekuensi wajib bagi pemanggil:** web **tidak boleh lagi mem-pre-apply `ApplyLimit`** pada teks SQL.
  `internal/service/session_preparer.go` sudah dibersihkan dari itu — SQL yang sudah memuat `LIMIT n` membuat
  probe tidak pernah melihat baris ekstra, sehingga semua hasil tampak utuh. Cap diteruskan lewat `QueryOptions`.
  `attachConnections`/`attachConnection` kini mengembalikan `([]string, error)` (daftar view yang ter-cap) supaya
  sumber yang terpotong di sisi sumber ikut dilaporkan.
- **Cap di sisi sumber ikut mengalir:** `StreamMongoCollections` mengembalikan `[]MongoMount{Collection, View,
  Documents, Truncated}` dan `CappedMongoViews()` mengubahnya jadi `CappedSources`; batas `DefaultMongoDocLimit`
  (10.000 dokumen/collection) dan `MaxMongoCollections` (32, gagal total bukan sebagian) tidak lagi bisa ditebak dari UI.
  Precedence: `row_limit` > `source_cap`.
- **Ke mana field-nya keluar:** `internal/dto/response/query.go` menambahkan keempat field itu (dengan komentar
  bahwa engine yang memutuskan, UI hanya merender) dan dipetakan di `query_service.go:109` + `share_service.go:324`.
  **UI frontend belum menampilkan penanda pemotongan** — itu disengaja, masuk M2 desktop/phase berikutnya.
- **Gerbang verifikasi:** tes baru `engine/duckdb/truncation_test.go` (7 kasus: terpotong, utuh di bawah cap, pas
  di cap, `LIMIT` penulis lebih ketat dari cap, default platform, `source_cap`, `row_limit` mengalahkan `source_cap`),
  `TestPlanLimit` di `guardrails_test.go`, `TestCappedMongoViews` di `nosql_adapter_test.go`. Kedua modul hijau pada
  `CGO_ENABLED=1` dan `=0`, `gofmt -l` bersih. Worker alert (`MaxRows: 1`) dan digest tetap berperilaku sama.

### Batas memori DuckDB jadi konfigurasi (M1.5, 2026-10-31)
- **Yang berubah:** `GuardrailConfig.MaxMemory string` (saat M1.5 ditutup nilainya `"2GB"`, sama seperti angka
  yang sebelumnya di-hardcode; **sejak 2026-11-01 default-nya `"2GiB"`** — keputusan user supaya kedua produk
  memakai angka yang sama dan labelnya tidak bohong) dan `NewEngine` sekarang memvalidasi nilainya — konfigurasi
  salah **gagal di startup**, bukan diam-diam memakai budget lama. Web mengambilnya dari
  `config.DuckDBConfig.MaxMemory` (`DUCKDB_MAX_MEMORY`, default `2GiB`) yang disalin di
  `internal/app/server/service.go:59`, diteruskan ke `SET max_memory=` setiap sesi lewat `quoteLiteral`.
  Desktop memakai field yang sama dari profilnya sendiri.
- **Kenapa angka polos ditolak:** `NormalizeMaxMemory` hanya menerima angka + unit dan mengembalikan default saat
  kosong; tesnya juga menolak injeksi seperti `"2GB; DROP TABLE t"`. Alasan lama ("DuckDB membaca `2048` sebagai
  byte") **tidak benar** — lihat koreksi 2026-11-01 di bawah; yang tersisa cuma alasan stilistis: gagal di startup
  dengan pesan jelas, bukan menyerahkan kesalahan itu ke parser DuckDB.
- **Fakta DuckDB yang tercatat:** **plafon pemakaian = `DUCKDB_MAX_CONCURRENCY × DUCKDB_MAX_MEMORY`** — komentar
  `.env.example` dan `docker-compose.yml` sudah menghitungnya begitu. Ini **plafon, bukan alokasi**: begitu
  sebuah sesi melewatinya DuckDB menuliskan hasilnya ke `temp_directory` (spill) alih-alih menaikkan RSS, jadi
  plafon yang lebih besar dari RAM container memperlambat query, bukan langsung OOM. Tes
  `TestEngineAppliesConfiguredMemoryBudget` membandingkan **byte**, bukan string, supaya perbedaan unit ini
  tidak membuat tes rapuh.
- **Koreksi + kapasitas baru (2026-11-01, hasil probe langsung ke DuckDB).** Empat hal yang sebelumnya salah catat,
  dibuktikan dengan `SET max_memory=` lalu `current_setting('max_memory')` pada sesi in-memory:
  1. **DuckDB menerima unit biner** `KiB|MiB|GiB|TiB` di samping `KB|MB|GB|TB` desimal — pesan errornya sendiri
     yang menuliskan keduanya (`expected: KB, MB, GB, TB for 1000^i units or KiB, MiB, GiB, TiB for 1024^i`).
     Pola engine dulu **menolak `2GiB`**, padahal itu satu-satunya cara menulis budget yang artinya sama dengan
     label di layar. Kini diterima, dan subtest `2GiB` membuktikan `current_setting` membalas tepat `2.0 GiB`
     = 2.147.483.648 byte, sementara `256MB` = 256.000.000 (ditampilkan `244.1 MiB`).
  2. **Angka polos ditolak DuckDB sendiri**, `Parser Error: Unknown unit for memory_limit` — bukan dibaca sebagai byte.
  3. **`-1` berarti tanpa batas** dan diterima DuckDB (`current_setting` membalas `16383.9 PiB`); engine tetap
     menolaknya karena guardrail tidak boleh bisa dimatikan lewat konfigurasi.
  4. **Kalau tidak di-SET sama sekali, default DuckDB = 80% RAM fisik** (terukur `6.3 GiB` di host 8 GB). Ini angka
     yang jadi pembanding saat desktop memilih default-nya: `2GiB` kita lebih konservatif dari DuckDB, bukan agresif.
     Catatan: `"2 GB"` (berspasi) diterima DuckDB tapi tetap ditolak engine — `DUCKDB_MAX_MEMORY=2 GB` di file env
     mudah salah baca, dan tes menolak `-2GB`/`0`/`NULL`/`auto`.
  5. **Default web ikut `2GiB` atas keputusan user (2026-11-01)** — sebelumnya `2GB` desimal, yang berarti angka
     di layar 7% lebih besar dari yang benar-benar dipakai. Efeknya ke `.env.example` + fallback
     `docker-compose.yml` (dua-duanya tidak ditetapkan di `.env` milik user, jadi compose memang berubah untuk
     stack lokal): plafon kini 2 × 2,15 GB = 4,29 GB di VM Docker 3,83 GiB. Itu tetap aman karena point 4 —
     DuckDB spill ke disk, bukan alokasi paksa — tapi kalau query besar mulai terasa lambat karena spill,
     yang diturunkan adalah `DUCKDB_MAX_MEMORY` atau `DUCKDB_MAX_CONCURRENCY`, bukan unitnya.
- `threads` sengaja **masih** 2 per sesi (di luar M1.5): budget CPU per sesi baru masuk akal setelah desktop M0
  menentukan profil mesin; field-nya sudah ditandai di `session_cgo.go`.
- **Gerbang verifikasi:** `TestNormalizeMaxMemory` (CGO-free, termasuk satu tes yang mengunci
  `DefaultMaxMemory == "2GiB"` supaya kedua produk tidak berpaling ke ejaan desimal diam-diam) +
  `TestEngineAppliesConfiguredMemoryBudget` dan `TestEngineRejectsUnusableMemoryBudget` (CGO) lolos; tes budget
  kini tabel dua kasus (`256MB` desimal, `2GiB` biner). Kedua modul hijau pada `CGO_ENABLED=1` dan `=0`,
  `gofmt -l` bersih.

### Perbaikan DSN Postgres (2026-10-31, bug lama dari audit desktop)
- **Gejalanya:** `SyncSchema` Postgres **selalu** gagal untuk koneksi yang disimpan tanpa `ssl_mode`.
  `connection_service.go` dulu merakit `... sslmode=%s` dengan nilai kosong lalu **menempel** `sslmode=disable`
  di belakangnya → `host=… sslmode= sslmode=disable`, dan `pgx.ParseConfig` menolak bentuk itu dengan
  `failed to configure TLS (sslmode is invalid)`. Jalur query (`session_preparer.go`) memberi default
  `disable` sendiri, jadi dua jalur berperilaku berbeda — sumber kebohungan "Test Connection hijau, SyncSchema mati".
- **Perbaikannya satu tempat:** `service.postgresDSN(cfg)` (`internal/service/session_preparer.go:288`) dipakai
  **kedua** pemanggil (`connection_service.go:330` dan `attachConnection` di `session_preparer.go:206`), dengan
  default `disable` dieksekusi sekali. Ini juga memangkas pekerjaan transport bastion (desktop M2): tinggal satu
  perakit DSN yang perlu menunjuk `127.0.0.1:<port>` forwarder.
- **Sekalian menutup injeksi di DSN:** `dsnValue` (`:301`) meng-quote nilai yang mengandung spasi/tab/newline/`'`/`\`
  sesuai aturan libpq (`\'` dan `\\`). Sebelum ini, password berisi spasi memecah DSN jadi parameter tambahan —
  kredensial bisa menyuntik `sslmode=disable` dan menurunkan TLS tanpa ada yang sadar.
  Catatan pgx yang relevan untuk reviewer: `sslmode` **tidak** muncul di `RuntimeParams` (diasimilasi ke
  `TLSConfig`/`Fallbacks`), jadi tes membandingkan bentuk `TLSConfig`, bukan string.
- **`ssl_mode` sekarang divalidasi (lanjutan 2026-10-31):** `normalizeSSLMode` (`session_preparer.go:269`,
  daftar nilainya di `:264`)
  hanya menerima enam nilai libpq — `disable · allow · prefer · require · verify-ca · verify-full` — dengan
  spasi tepi di-trim dan kosong berarti default `disable`. Ditutup di **dua** titik: `UpsertEnvironment`
  (`connection_service.go:163-166`) menolak sebelum payload dienkripsi & disimpan (400 dengan daftar nilai yang
  boleh), dan `postgresDSN` ikut menolak supaya profil lama yang sudah tersimpan tidak diam-diam turun ke
  mode lain — typo `verify` alih-alih `verify-full` berarti *tanpa pemeriksaan sertifikat*, jadi tidak boleh
  diredam. `postgresDSN` karena itu kini mengembalikan `(string, error)`.
  Frontend tidak berubah: `environment-form.tsx:193` memang sudah render `<Select>` dengan enam nilai yang sama
  — yang hilang hanya gerbang sisi API (klien lain bisa mengirim apa pun).
- **Perbaikan ikutan di handler:** `Create`, `UpsertEnvironment` dan `SyncSchema` di
  [internal/handler/connection_handler.go](backend/internal/handler/connection_handler.go) memetakan *semua*
  error service ke **500** lewat `WriteInternalServerErrorResponse`, termasuk `customerror.NewBadRequestError`
  yang dibuat kode ini sendiri (validasi `alias_name` dan sekarang `ssl_mode`). Ketiganya memakai
  `WriteServiceError` sehingga 400 benar-benar jadi 400 — tanpa itu, validasi apa pun tidak akan pernah
  terlihat oleh pemanggil API. `List`/`Test` dibiarkan (errornya murni internal).
- **Belum ada tempat untuk CA privat (`rootcert`)** — default `disable` tetap salah untuk DB internal
  self-signed; itu BRD desktop §10 butir 8, masuk M2 (butir 9 sudah ditutup).
- **Gerbang verifikasi:** 6 tes di `internal/service/session_preparer_test.go` (4 DSN + `TestNormalizeSSLMode`
  + `TestPostgresDSNRejectsUnknownSSLMode`), salah satunya membuktikan bentuk DSN lama memang ditolak
  `pgx.ParseConfig`. `go test ./...` hijau di `CGO_ENABLED=1` dan `=0`, `gofmt -l` bersih.
  **Belum diuji lewat HTTP:** daemon Docker sudah hidup dan stack user jalan (api di 18080), tapi container
  `muara_api` masih memegang biner pra-perbaikan — memakanya butuh build ulang + restart infra milik user,
  jadi validasi ini masih terbukti di level unit saja. Image `muara-bi-core:dev` build 2026-10-31 sudah
  berisi kode ini, jadi tinggal `docker compose up --build api`.

### Paket Docker (Phase 5.1) — fakta teknis yang sudah diverifikasi
- **Kemasan DuckDB:** `go-duckdb v1.8.5` membundel `libduckdb.a` untuk `linux_amd64`/`linux_arm64` (glibc + libstdc++), DuckDB internal versi **v1.1.3**, dan **tidak** punya build tag `STATIC_EXTENSIONS`. Karena itu: runtime image `debian:bookworm-slim` (bukan Alpine/musl), dan ekstensi `postgres`/`mysql`/`sqlite` dipre-bundle ke `/opt/duckdb/extensions`.
- **`cmd/duckdb-bundle`** menjalankan `INSTALL`+`LOAD` lewat DuckDB itu sendiri (bukan URL curl yang di-hardcode) sehingga subdirektori `<versi>/<platform>` dipilihkan oleh mesin; teruji di host: menghasilkan `v1.1.3/osx_arm64/{postgres,mysql,sqlite}_scanner.duckdb_extension` dan mode `-install=false` memuatnya kembali tanpa network. Nama `postgres` di 1.1.3 otomatis di-alias ke `postgres_scanner`.
- **`DUCKDB_EXTENSION_DIRECTORY`** → `GuardrailConfig.ExtensionDirectory` → `SET extension_directory` setiap sesi. `loadExtension` membiarkan `INSTALL` gagal (image hanya-read) tapi **menggagalkan permintaan** bila `LOAD` gagal — sebelumnya error ditelan dan muncul sebagai "Catalog Error" jauh di hilir.
- **Migrasi goose kini `//go:embed`** (`internal/generated/embed.go`, `goose.SetBaseFS` + `goose.Up(db, "migrations")`): sebelumnya dibaca dari path relatif `./internal/generated/migrations`, sehingga biner di container (WORKDIR `/app`) tidak bisa bermigrasi. Terverifikasi: `goose: no migrations to run. current version: 2` pada DB yang sudah bermigrasi.
- **Linking:** `-extldflags=-Wl,-E` dibutuhkan agar ekstensi yang di-`dlopen` bisa melihat simbol DuckDB dari biner yang di-link statis.
- **Compose:** `api` (healthcheck `/health`) → `worker` (override `command`, healthcheck `disable`) → `web` (`API_BASE_URL=http://api:8080`, proxy `/api/*`). Kredensial nyata (`APP_SECRET_KEY`, `APP_ENCRYPTION_MASTER_KEY`, `DB_PASSWORD`) di-guard `:?` supaya compose gagal cepat alih-alih memakai default yang ada di repo publik. `DB_PUBLISH_PORT` memisahkan port host dari port internal `DB_PORT=5432`.
- **Frontend image:** `output: 'standalone'` ditambahkan di `next.config.ts`; `npm run build` tetap hijau (14 route, PPR utuh) dan `.next/standalone/server.js` ter-generated.
- **Kendala mesin dev (penting untuk verifikasi lanjutan):** Docker Desktop di mesin ini punya VM 4 GB / host 8 GB dan BuildKit-nya dua kali menggantung di step berat (`go build` link dan `next build`) dengan CPU guest ~0%; egress container sendiri sehat (`proxy.golang.org` 200, `extensions.duckdb.org` 301, `registry.npmjs.org` 200). Docker Hub sempat tidak bisa di-resolve dari daemon, sehingga base image bisa dialihkan lewat build-arg `GO_IMAGE`/`NODE_IMAGE` (mis. mirror `public.ecr.aws/docker/library/...`) — tercatat di `Dockerfile`, `frontend/Dockerfile`, dan `.env.example`.

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
- [x] Public Share Link & Signed JWT Embedded Analytics (`/share/:token`, `/embed`).
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
  - [x] `APP_PUBLIC_BASE_URL` ditambahkan ke `backend/.env.example`.
  - [x] Unit test share: `internal/service/share_service_test.go` (entropi token, url-safe encoding, panjang token) — lulus hijau.
  - [x] **Catatan Phase 4 (frontend):** halaman Next.js `/share/[token]` dan `/embed` memancarkan header CSP `frame-ancestors` dari field `frame_ancestors` — diambil lewat `src/proxy.ts` (dulu `middleware.ts`), di-cache 60 detik per token, dan gagal tertutup (`frame-ancestors 'none'` + `X-Frame-Options: DENY`) bila resolusi gagal atau allowlist kosong.
    - Terverifikasi di browser (2026-10-17): `/share/<token>` → `frame-ancestors http://localhost:3000 https://embed.example.com`; `/embed?<JWT EMBED_IFRAME>` → `frame-ancestors https://embed.example.com`; `/embed?<token PUBLIC_LINK>` → ditolak API sehingga header menjadi `'none'` + `DENY`.
- [x] **Scheduled Digest & Threshold Alerts Dispatcher (Telegram, Slack, Email, Webhook)**:
  - [x] Entity `internal/domain/entity/alert.go` (`DashboardAlert`, digest vs threshold alert specs, channel destination types).
  - [x] Repository `internal/repository/pgsql/alert_repository.go` (`AlertRepositoryProvider` implementasi PGSQL).
  - [x] DTO `internal/dto/request/alert.go` & `internal/dto/response/alert.go`.
  - [x] Service `internal/service/alert_service.go` (validasi cron expression via `robfig/cron/v3`, sanitasi payload alert, test alert dispatcher, manual trigger).
  - [x] Unit test `internal/service/alert_service_test.go` (6 skenario: cron valid/invalid, threshold widget/operator validation, channel rejection) — lulus hijau.
  - [x] Handler `internal/handler/alert_handler.go` dengan RBAC Guard (`PermAlertRead`, `PermAlertWrite`).
  - [x] Worker `internal/worker/digest_worker.go` (multi-sheet Excel XLSX exporter via `exporter.WriteFile` & dispatch ke Telegram/Slack/Email/Webhook).
  - [x] Periodic Asynq Scanner `internal/worker/scheduler.go` (pemindai cron 1 menit untuk active alerts dan scheduled digests).
  - [x] Payload netral `internal/tasks/tasks.go` dan re-export di `internal/worker/tasks.go` menghilangkan import cycle antara `service` dan `worker`.
  - [x] Wiring lengkap di `cmd/worker/main.go`, `server/repository.go`, `server/service.go`, dan `server/server.go`.
- [x] **AI Co-Pilot Service (Semantic Schema Pruning & PII Masking, Prompt-to-Dashboard)**:
  - [x] Entity `internal/domain/entity/ai.go` (`AISettings`, enum provider Anthropic/OpenAI/Gemini/Ollama).
  - [x] DTO `internal/dto/request/ai.go` & `internal/dto/response/ai.go`.
  - [x] Repository `internal/repository/pgsql/ai_repository.go` (`AIRepositoryProvider` implementasi PGSQL).
  - [x] Engine `internal/ai/pruner.go`:
    - Masking otomatis kolom PII default (`email`, `phone`, `password`, `salary`, `ssn`, `nik`, dll) + custom blacklist per workspace.
    - Token relevance scoring & subpart matching untuk memprioritaskan tabel yang relevan dengan prompt pengguna.
    - DDL generator DuckDB yang steril dari data baris riil (hanya skema DDL).
  - [x] Multi-provider LLM Adapter `internal/ai/client.go` (Anthropic Claude, Google Gemini, OpenAI-compatible / Ollama / Azure, serta safe offline simulation mock).
  - [x] Service `internal/service/ai_service.go`:
    - CRUD AI settings dengan enkripsi AES-256-GCM untuk API key.
    - `GenerateSQL`: prompt → schema pruning → LLM completion → AST guardrail `duckdb.ValidateQuery` (wajib lolos anti-DDL/DML dan SELECT guard).
    - `GenerateDashboard`: prompt → LLM completion → JSON structure parsing & validation → query verification untuk setiap widget.
  - [x] Handler `internal/handler/ai_handler.go` (`GET/PUT /ai/settings`, `POST /ai/generate-sql`, `POST /ai/generate-dashboard`) dijaga RBAC Guard.
  - [x] Unit test: `internal/ai/pruner_test.go` dan `internal/service/ai_service_test.go`.
  - [x] Wiring lengkap di `server/repository.go`, `server/service.go`, dan `server/server.go`.


---

### Phase 4: Next.js Frontend (`@muara/web`) - Light Off-White Theme
- [x] Init Next.js 15+ App Router, Tailwind CSS, Shadcn UI (`#f8fafc` base theme).
- [x] Zustand stores (`auth`, `env`, `canvas`, `filter`, `schema`, `project`).
- [x] Global Shell: Left Navigation Rail & Header (Workspace, Project, Env Switcher, Sync Button).
- [x] Dashboard Catalog & Create Wizard Modal (AI Co-Pilot, Template, Blank).
- [x] 12-Column Responsive Canvas (`react-grid-layout`) + ECharts (`ResizeObserver`).
- [x] Anti-self-loop Compound Cross-Filtering & Dynamic Parameter Filter Bar.
- [x] 3-Layer Slide-Up Drawer (AI Prompt Bar, Visual Builder, Monaco SQL Editor).
- [x] Semantic Models Studio & Data Studio query scratchpad.
- [x] Data Sources UI (Direct Connect default, collapsible SSH Bastion).
- [x] Public Share & Embedded Canvas views.
- [x] AI Co-Pilot Settings UI.
- [x] Data table widget: sort kolom, paginasi, nomor baris, format angka/tanggal, dan warna kustom per widget (preset + color picker di drawer inspector).
- [x] Upgrade UI chart (berlaku global, satu builder `lib/echarts-options.ts`): sumbu Y ringkas (315K), grid putus-putus, bar bersudut membulat + gradasi, legenda `scroll`, tooltip kartu mengikuti tema + satuan, donut dengan total di tengah & legenda bawah, tanggal ISO dipangkas di sumbu kategori, dan palet warna kustom per widget (preset + 5 color picker).
- [x] Smoke test golden path di browser (login → catalog → canvas → widget → share) — stack lokal aktif (Docker Compose + migrasi + API + worker).
  - Alur terverifikasi end-to-end: login → sync schema Postgres sample (`orders`, 6 kolom) → Data Studio (sidebar `dwh_sample.orders`, query federated `GROUP BY` 3 baris / 27ms, tab Bar merender ECharts) → wizard Catalog (jalur *Canvas kosong*) → autosave layout 12 kolom → share `PUBLIC_LINK` (`/share/<token>` memancarkan `frame-ancestors http://localhost:3000 https://embed.example.com`) → share `EMBED_IFRAME` (`/embed?token=<JWT>` memancarkan `frame-ancestors https://embed.example.com`; token PUBLIC_LINK ditolak → fail-closed `'none'` + `X-Frame-Options: DENY`).
  - Kontrak redaksi publik terverifikasi: respons `/api/v1/public/*` tidak memuat `raw_sql` maupun `builder_state`.
  - Bug yang ditemukan & diperbaiki saat smoke test (lihat daftar di §3).

---

### Phase 5: End-to-End Integration, Docker Packaging & Testing
- [x] Production multi-stage `Dockerfile` with CGO and DuckDB extensions.
  - `Dockerfile` (root, context repo root) + `.dockerignore`; `frontend/Dockerfile` (Next standalone) + `frontend/.dockerignore`.
  - `backend/cmd/duckdb-bundle`: memasang `postgres`/`mysql`/`sqlite` ke folder tetap saat build, dan mode `-install=false` sebagai probe ketersediaan offline.
  - `DUCKDB_EXTENSION_DIRECTORY` → `GuardrailConfig.ExtensionDirectory` → `SET extension_directory`; `loadExtension` kini gagal keras saat `LOAD` gagal (INSTALL best effort).
  - Migrasi goose dipindah ke `//go:embed` (`internal/generated/embed.go`) supaya biner bisa bermigrasi tanpa file SQL di disk.
- [x] Build image terverifikasi di mesin ini (CGO link `libduckdb.a` + step bundel ekstensi).
  - `muara-bi-core:dev` (364 MB) dan `muara-bi-web:dev` (209 MB) keduanya berhasil dibangun di Docker VM 4 GB; build core ±35 menit karena satu langkah `go build` menautkan `libduckdb.a`.
  - Core: `--network none` probe mengonfirmasi `LOAD` postgres/mysql/sqlite dari `/opt/duckdb/extensions` tanpa egress; berjalan sebagai uid 10001.
  - Web: container inspection memastikan `server.js` + `.next/static` + `public` + `node_modules` (38 MB) ada dan proses berjalan sebagai uid 10001 `nextjs`; smoke test di port 13000 mengembalikan `200` untuk `/` dan `/login`.
- [x] `docker compose up` stack penuh (api, worker, web) + query federated dari dalam container tanpa network egress.
  - Jalankan tanpa menyentuh dev server host: `cp backend/.env .env` lalu `APP_API_PORT=18080 APP_WEB_PORT=18000 docker compose up -d --no-deps api worker web`. `--no-deps` penting: tanpa itu compose bisa me-recreate `muara_postgres`/`muara_redis` yang sedang dipakai dev server. Port 18080/18000 dipakai justru supaya 8080/3000 milik dev server tetap hidup.
  - **Akar `.env` wajib merupakan salinan `backend/.env`** (bukan `.env.example`): `APP_ENCRYPTION_MASTER_KEY` yang berbeda membuat `encrypted_config` koneksi yang sudah tersimpan tidak bisa didekripsi. File ini di-gitignore (`.gitignore:14`) dan tidak ikut commit.
  - Hasil start: `muara_api` healthy (`goose: no migrations to run. current version: 2` — migrasi embedded jalan tanpa file SQL di disk), `muara_worker` membuka pool `queues: critical=6, default=3, heavy=1` + scheduler cron 1 menit, `muara_web` healthy dan `/login` 200.
  - Networking & egress: dari dalam `muara_api`, `sample_postgres:5432`, `sample_mongo:27017`, `postgres:5432`, `redis:6379` semuanya reachable; `/opt/duckdb/extensions/v1.1.3/linux_arm64/{postgres,mysql,sqlite}_scanner.duckdb_extension` tersedia sehingga `LOAD` tidak butuh internet.
  - Query federated nyata lewat API container (proyek "Retail Analytics", env `container-e2e`): sisi Postgres `cdwh.public.orders` = 6 baris / Rp537.000 (70 ms); sisi Mongo hasil streaming `clogs_sessions` = 7 baris, 6 kota (53 ms); JOIN lintas sumber `orders × sessions` = 4 grup produk dengan total revenue tetap 537.000 dan sesi yatim `order_id=99` terbuang benar (380 ms). Cache terverifikasi di container: `force_refresh=true` → `cached:false`, query sama → `cached:true`.
  - Dispatch alert terverifikasi end-to-end di container: worker mengeksekusi SQL federated, membandingkan ambang (`537000 > 100000` → `ALERT TRIGGERED!`), dan `Notifier.PostJSON` mengirim body ke sink HTTP di host (`Content-Type: application/json`; `alert_id`, `env_tag`, `metric_value`, `operator`, `threshold`, `title`, `triggered_at`). Jalur gagal juga dibuktikan: target di port tertutup → `ERROR failed to dispatch alert: webhook: delivery failed: ... connection refused` + `asynq task failed`, sehingga task di-retry alih-alih tercatat terkirim. Slack/Telegram asli tidak dipanggil (tidak ada webhook live) — bentuk payload, path `/bot<token>/sendMessage` dan penanganan `ok:false` dijamin `internal/worker/notifier_test.go`.
  - **Bug nyata yang ditemukan & ditutup (web → API selalu gagal di compose):** `frontend/Dockerfile` menulis bahwa `API_BASE_URL` "dibaca saat server start", padahal Next menulis rewrite `/api/*` ke `.next/routes-manifest.json` **saat build**. Image lama membawa `http://127.0.0.1:8080`, sehingga setiap panggilan API dari UI mati dengan `ECONNREFUSED 127.0.0.1:8080`. Sekarang `API_BASE_URL` adalah build arg (compose: `WEB_API_BASE_URL`, default `http://api:8080`) dan manifest terverifikasi berisi `http://api:8080/api/:path*`; nilai runtime tetap dipakai `src/proxy.ts` untuk lookup CSP share/embed.
  - **Bug nyata yang ditemukan & ditutup (tes koneksi MongoDB):** `ConnectionService.TestConnection` melakukan TCP dial ke `host:port`, padahal profil MongoDB menyimpan targetnya di `uri` — setiap sumber Mongo melapor `Direct connection failed: dial tcp :0: connect: connection refused`, dan sebaliknya dial TCP akan "sukses" walau kredensialnya salah. Sekarang kanal `mongodb` di-ping dengan driver (`duckdb.PingMongo`, memakai `mongoURI()` sehingga bentuk host/port maupun URI sama-sama jalan) dan teks error-nya dibersihkan dari kredensial (`redactCredentials`, `TestRedactCredentials`). Teruji di container: `MongoDB ping succeeded` (614 ms).
  - Sisa yang butuh tindakan manual: source data yang dipakai dari dalam container **harus memakai nama service compose** (`sample_postgres` / `sample_mongo`), bukan `127.0.0.1:5435`; koneksi `dwh_sample` lama hanya benar untuk proses di host. Dua koneksi `cdwh` (SQL/postgres) dan `clogs` (NOSQL/mongodb) dengan env tag `container-e2e` dibiarkan ada di DB dev sebagai contoh; test alert yang sempat dibuat sudah dihapus agar scheduler 5-menit tidak menembak port yang sudah ditutup.
- [x] E2E cross-database federated query tests (Postgres + MongoDB).
  - `backend/internal/duckdb/federated_e2e_test.go` (`MUARA_E2E_FEDERATED=1 go test ./internal/duckdb -run Federated -v`): satu sesi DuckDB memanggil `AttachPostgres("dwh")` (postgres_scanner) lalu `StreamMongoCollections(alias="logs")`, dan berhasil menjalankan 5 kasus — baca sisi Postgres (6 baris), kolom hasil flattening sisi Mongo (7 baris), JOIN federated dengan parameter terikat `{{device}}`, agregasi revenue lintas sumber (Rp537.000), dan JOIN tiga sumber `orders × sessions × page_views` (7 page view). Latensi: 89 ms query pertama (pemuatan ekstensi + foreign scan), 3–10 ms sisanya.
  - Data sample Mongo kini bisa dibuat ulang: `docker/seed/mongo_analytics.js` (`sample_analytics.sessions` 7 dokumen termasuk satu sesi yatim `order_id=99`, `page_views` 8 dokumen).
  - Bug nyata yang ditemukan: `flattenMap` hanya mencocokkan `[]any`, padahal driver Mongo mendekode array sebagai `primitive.A` dan `_id` sebagai `primitive.ObjectID`. Array jatuh ke branch `default` sehingga kolom array terbentuk sebagai DuckDB `LIST` (tipe elemen bisa berubah antar dokumen) alih-alih string JSON yang stabil, dan ObjectID ter-render `ObjectID("...")`. Sudah diperbaiki + diregresi oleh `TestFlattenMapHandlesMongoDriverTypes`.
- [x] Concurrency load tests (Redis semaphores & Asynq priority queues).
  - Budget bersama: `internal/limiter/semaphore.go` — sorted set Redis ber-Lua (`ZREMRANGEBYSCORE` prune → `ZCARD` → `ZADD` dalam satu script) sehingga dua proses tidak bisa merebut slot terakhir. Slot yang punya pemanggil aktif diperpanjang tiap `lease/3`; slot milik proses yang mati berhenti diperpanjang dan diklaim kembali setelah lease lewat. Redis tidak dapat diakses → *fail open* dengan warning (mesin tetap punya batas per-proses), dan antre yang batal karena pemanggil putusnya koneksi tetap `context.Canceled`, bukan 429.
  - Beban riil (`MUARA_E2E_REDIS=127.0.0.1:6379 go test ./internal/limiter -v`): 24 pemanggil bersamaan lewat 24 instance `Semaphore` berbeda (meniru beberapa replika API/worker yang berbagi satu Redis), budget 2 — puncak konkurensi terukur tidak pernah melewati budget, semua pemanggil selesai, dan sorted set kosong kembali setelah semua release; plus uji renew hold 2× lease, klaim lease yang diabaikan, dan deadline `ErrNoSlot`.
  - Terpasang di jalur eksekusi: `SessionPreparer.Execute` mengambil slot *setelah* validasi + rewrite SQL (query yang pasti gagal tidak memakan slot) dan `defer release()`; `customerror.NewTooManyRequestsError` → `httputil.WriteTooManyRequestsResponse` (429) yang sebelumnya akan jatuh ke 500 lewat `WriteServiceError`.
  - Knob baru: `DUCKDB_MAX_CONCURRENCY` (2), `DUCKDB_QUEUE_TIMEOUT_SECONDS` (30), `DUCKDB_SLOT_LEASE_SECONDS` (120), `WORKER_CONCURRENCY` (4) — ada di `config.DuckDBConfig`/`Config`, `docker-compose.yml` anchor `x-core-env`, dan `.env.example`. `worker/server.go` tidak lagi hardcode `Concurrency: 10`; bobot antrean pindah ke `tasks.QueueWeights` satu-satunya sumber kebenaran.
  - Prioritas Asynq diverifikasi terhadap Redis hidup (`go test ./internal/tasks`): tiap tipe task benar-benar masuk antrean yang berbobot (alert→critical, query/digest→default, export→heavy) dibaca ulang lewat `asynq.Inspector`, termasuk retry dan timeout.
  - Temuan: task `query:execute` tidak pernah memasang timeout, jadi Asynq memberi default **30 menit** — satu query macet menahan slot worker sangat lama. Sekarang diikat 90 detik (deadline antrean + statement timeout). Catatan: `max_memory` DuckDB masih hardcoded 2 GB/sesi di `session_cgo.go`, jadi `DUCKDB_MAX_CONCURRENCY × 2 GB` adalah memori puncak yang harus tersedia di host/container. *(Sudah tidak berlaku sejak M1.5 2026-10-31: angka itu kini `DUCKDB_MAX_MEMORY`/`GuardrailConfig.MaxMemory` — lihat §3.)*
- [x] Cache & RLS verification tests.
  - RLS dieksekusi nyata (bukan sekadar cek string) di `backend/internal/duckdb/rls_execution_test.go` memakai pipeline yang sama dengan `SessionPreparer.Execute`: filter memotong 6 baris → 3 baris BRG-01 (Rp315.000), indeks parameter berlanjut (`{{channel}}`=$1, aturan RLS=$2), nilai `'BRG-01' OR '1'='1'` ter-bind (0 baris, bukan bocor), dan kolom guard wajib ada di proyeksi penulis SQL — kalau tidak, query gagal (fail-closed).
  - Cache diverifikasi terhadap Redis hidup (`MUARA_E2E_REDIS=127.0.0.1:6379 go test ./internal/cache -v`, DB 15, kunci dibersihkan sendiri): miss→eksekusi→hit tanpa eksekusi ulang, payload & metadata kolom utuh setelah round-trip JSON, `ForceRefresh` menjalankan ulang, dua scope RLS menghasilkan kunci berbeda sehingga tidak saling melayani, kedaluwarsa TTL, `InvalidateByWorkspace` tidak menyentuh workspace lain, dan error query tidak ikut tercache.
  - **Lubang guardrail yang ditemukan & ditutup:** `ApplyLimit` dulu memakai `limitPattern` yang cocok ke *semua* `LIMIT <angka>` di teks SQL, sehingga penulis widget cukup menulis `LIMIT 999999999` (atau menaruh LIMIT di subquery) untuk melepaskan batas baris platform. Sekarang hanya klausul `LIMIT` di ekor pernyataan yang dihitung, dan nilai author di-clamp ke `maxRows` (OFFSET dipertahankan); LIMIT di dalam subquery tetap dibiarkan tapi batas luar selalu ditambahkan. Regresi di `TestApplyLimit`.
- [x] Alert dispatching verification (Telegram & Slack).
  - Dispatcher bersama: `internal/worker/notifier.go` (`Notifier.SendText` untuk SLACK/TELEGRAM/EMAIL dan `Notifier.PostJSON` untuk WEBHOOK). `AlertWorker.dispatchNotification` dan `DigestWorker.dispatch` tidak lagi menyalin blok HTTP masing-masing — sebelumnya kedua worker punya tiga cabang `switch` yang identik.
  - **Bug nyata yang ditutup (delivery dianggap sukses padahal gagal):** kode lama menutupi `resp.Body` tanpa pernah memeriksa `resp.StatusCode`; Slack/Telegram yang menjawab 401 (webhook dicabut) atau 500 tetap dihitung terkirim, `last_triggered_at` diperbarui, dan task Asynq ditandai sukses tanpa retry. Sekarang setiap non-2xx menjadi error (pesan provider dipotong ke 200 karakter), dan khusus Telegram — yang melaporkan kegagalan dengan HTTP 200 + `{"ok":false,"description":...}` — body ikut didekode.
  - **Bug nyata yang ditutup (kirim diam-diam):** `webhook_url` / `bot_token` / `chat_id` yang kosong hanya membuat cabang `if` dilewati lalu `return nil`. Sekarang konfigurasi penerima yang tidak lengkap adalah error, sehingga alert gagal dengan jelas dan bisa di-retry.
  - Kunci bot Telegram tidak pernah muncul di log maupun string error: `redactProviderURL` memotong URL transport pada prefix `TelegramAPIBase` (Go menyisipkan URL penuh — termasuk token — ke error `*url.Error`). Diverifikasi `TestBotTokenNeverAppearsInError`.
  - Parser `recipient_config` dipindah ke domain (`internal/domain/entity/alert_recipient.go`): `ParseAlertRecipient` + `ValidateFor(channel)` dipakai service **dan** worker, jadi aturan "siapa targetnya" hanya ada satu tempat. `chat_id` numerik dibaca presisi lewat `json.Decoder.UseNumber()` (lewat float64, id grup `-100xxxxxxxxxx` bisa berubah digit) dan dinormalisasi ke string.
  - Validasi saat menyimpan (`internal/service/alert_service.go`): `Create` dan `PUT` dengan `recipient_config` kini menolak channel tanpa target yang bisa dipakai (400 dengan nama field-nya), `webhook_url` wajib URL http(s) ber-host, `EMAIL` wajib `email_list` berisi alamat; `TestAlert` memvalidasi lebih dulu alih-alih mengandalkan worker yang gagal belakangan. Alert lama dengan `{}` tetap bisa diubah field lain (hanya recipient/channel baru yang diperiksa). Placeholder UI `EMAIL` dikoreksi dari `{"to":[…]}` ke `{"email_list":[…]}` sesuai kunci yang dibaca dispatcher.
  - Test (`go test -race ./internal/worker` hijau): `internal/worker/notifier_test.go` memakai `httptest` — bentuk payload (`{"text":…}` / `{"chat_id","text"}`), header `Content-Type`, path `/bot<token>/sendMessage`, non-2xx → error, Telegram `ok:false` → error, recipient tidak lengkap → error **tanpa** permintaan keluar, channel tak dikenal → error, body provider dipotong, dan context batal → error. `internal/domain/entity/alert_recipient_test.go` menutup presisi `chat_id`, penolakan non-object, dan 6 kasus `ValidateFor`.
  - **Belum ada:** `EMAIL` masih bisa disimpan dan divalidasi, tapi belum benar-benar terkirim — tidak ada SMTP sender di repo ini (`SMTP_HOST/PORT/USERNAME/PASSWORD/FROM` hanya ada di `backend/.env.example` dan tidak dibaca `internal/config/config.go`). Dispatcher kanal ini mengembalikan nil (bukan error) disertai warning `no SMTP sender is implemented` supaya task tidak retry tanpa ujung; konsekuensinya alert email tetap tercatat sebagai terkirim padahal tidak ada pesan keluar, dan itu disengaja sampai sender-nya dibuat.
