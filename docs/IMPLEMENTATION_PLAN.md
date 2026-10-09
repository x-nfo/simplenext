# Panduan Inisiasi & Pengerjaan SimpleNext (Replika ERPNext di Golang)

Dokumen ini berisi panduan teknis langkah demi langkah untuk membangun fondasi **SimpleNext**, replika fungsional dan terstruktur dari ERPNext menggunakan bahasa **Golang** dan **PostgreSQL**, dengan fokus awal pada modul **Accounting Core** (Chart of Accounts, Naming Series, Journal Entry, dan General Ledger).

---

## 1. Arsitektur & Pola Desain

### Filosofi Inti
1. **Double-Entry Bookkeeping:** Setiap transaksi finansial harus seimbang (`Total Debit == Total Credit`).
2. **Immutable Ledger (Append-Only):** Tabel `gl_entry` tidak pernah diubah atau dihapus nilainya. Pembatalan dilakukan dengan membuat *reversal entry* (jurnal pembalik).
3. **ERPNext Document Lifecycle:** Setiap transaksi melalui status dokumen standar:
   - `docstatus = 0` (Draft): Masih dapat diedit, belum mempengaruhi pembukuan.
   - `docstatus = 1` (Submitted): Terkunci (read-only), otomatis memposting ke `gl_entry`.
   - `docstatus = 2` (Cancelled): Dibatalkan, memposting jurnal pembalik ke `gl_entry`.
4. **Naming Series Generator:** Primary Key dokumen transaksi berupa string yang berurutan (misal: `JV-2026-00001`).

---

## 2. Struktur Direktori Proyek (Standard Go Layout)

```text
simplenext/
├── cmd/
│   └── api/
│       └── main.go                  # Entry point HTTP server
├── internal/
│   ├── accounting/                  # Domain Accounting
│   │   ├── handler.go               # HTTP Handlers (REST endpoints)
│   │   ├── model.go                 # Structs (Account, JournalEntry, GLEntry)
│   │   ├── repository.go            # SQL queries & database operations
│   │   └── service.go               # Business logic & validations
│   ├── core/                        # Domain Core / Platform
│   │   ├── naming/                  # Naming series generator
│   │   │   ├── repository.go
│   │   │   └── service.go
│   │   └── types/                   # Shared types (DocStatus, etc)
│   └── platform/
│       └── database/                # Database connection pool (pgx)
├── migrations/                      # File migrasi SQL mentah
│   ├── 000001_init_schema.up.sql
│   ├── 000001_init_schema.down.sql
│   └── 000002_seed_accounts.sql
├── docker-compose.yml               # PostgreSQL untuk local dev
├── go.mod
├── go.sum
├── Makefile                         # Helper commands (run, migrate, test)
└── IMPLEMENTATION_PLAN.md           # Dokumen panduan ini
```

---

## 3. Langkah Pengerjaan Fase demi Fase

### FASE 1: Setup Lingkungan & Skema Database
- [x] **Langkah 1.1:** Inisialisasi modul Go (`go mod init simplenext`).
- [x] **Langkah 1.2:** Buat file `docker-compose.yml` untuk menjalankan PostgreSQL secara lokal.
- [x] **Langkah 1.3:** Rancang file DDL migrasi SQL (`migrations/000001_init_schema.up.sql`):
  - Tabel `series_counters` (untuk Naming Series)
  - Tabel `accounts` (Hierarki Bagan Akun / Chart of Accounts)
  - Tabel `journal_entries` (Header transaksi)
  - Tabel `journal_entry_accounts` (Child rows debit/credit)
  - Tabel `gl_entries` (Buku besar / General Ledger immutable)
- [x] **Langkah 1.4:** Buat file seeding `migrations/000002_seed_accounts.sql` untuk membuat pohon akun standar (Asset, Liability, Equity, Income, Expense).

---

### FASE 2: Engine Core Platform (Naming Series & Lifecycle)
- [x] **Langkah 2.1:** Setup koneksi database di `internal/platform/database` menggunakan driver performa tinggi (`pgx` / `pgxpool`).
- [x] **Langkah 2.2:** Implementasi **Naming Series Service** (`internal/core/naming`):
  - Fungsi untuk mem-parse pola seri: `JV-.YYYY.-.#####` $\rightarrow$ `JV-2026-00001`.
  - Menggunakan query database atomic (`SELECT ... FOR UPDATE` atau `INSERT ... ON CONFLICT DO UPDATE`) untuk mencegah duplikasi nomor saat request konkuren tinggi.
- [x] **Langkah 2.3:** Standarisasi tipe data `DocStatus` di `internal/core/types`.

---

### FASE 3: Modul Akuntansi (Domain Logic)
- [x] **Langkah 3.1: Chart of Accounts (COA)**
  - Implementasi CRUD dan query hierarki akun (Tree query menggunakan `parent_account`).
  - Validasi: Transaksi hanya boleh dilakukan pada akun *leaf* (`is_group = false`).
- [x] **Langkah 3.2: Journal Entry - Alur Draft (`docstatus = 0`)**
  - Menerima input header dan baris debit/credit.
  - Generate nomor transaksi melalui Naming Series.
  - Simpan ke `journal_entries` dan `journal_entry_accounts`.
- [x] **Langkah 3.3: Journal Entry - Alur Submit (`docstatus = 1`)**
  - Bungkus dalam transaksi database (`tx.Begin`).
  - Validasi: `SUM(debit) == SUM(credit)`.
  - Validasi: Semua akun yang digunakan harus aktif dan bukan akun grup.
  - Update `journal_entries.docstatus = 1`.
  - Generate baris di `gl_entries` untuk setiap baris debit dan kredit.
  - `tx.Commit`.
- [x] **Langkah 3.4: Journal Entry - Alur Cancel (`docstatus = 2`)**
  - Bungkus dalam transaksi database.
  - Update `journal_entries.docstatus = 2`.
  - Tulis *reversal entry* ke `gl_entries` (membalik saldo debit jadi kredit dan sebaliknya) dengan referensi dokumen pembatalan.

---

### FASE 4: API Layer & HTTP Handlers
- [x] **Langkah 4.1:** Buat HTTP Router ringan (misal menggunakan `net/http` standar atau `go-chi/chi`).
- [x] **Langkah 4.2:** Endpoint RESTful:
  - `GET /api/v1/accounts` — Daftar Chart of Accounts dalam bentuk Tree/Flat.
  - `POST /api/v1/journal-entries` — Membuat Draft Journal Entry baru.
  - `GET /api/v1/journal-entries/{name}` — Melihat detail Journal Entry beserta baris akun.
  - `POST /api/v1/journal-entries/{name}/submit` — Memposting transaksi ke Buku Besar.
  - `POST /api/v1/journal-entries/{name}/cancel` — Membatalkan transaksi.
  - `GET /api/v1/reports/general-ledger` — Laporan rincian buku besar per akun.
  - `GET /api/v1/reports/trial-balance` — Laporan Neraca Saldo (cek keseimbangan buku besar).

---

### FASE 5: Pengujian & Validasi (Automated Testing)
- [x] **Langkah 5.1:** Unit test untuk formula Naming Series dan kalkulasi balancing debit/credit.
- [x] **Langkah 5.2:** Integration test: Skenario *end-to-end* siklus transaksi:
  1. Buat Draft Journal Entry (Kas berkurang, Beban bertambah).
  2. Coba Submit dengan saldo tidak balance $\rightarrow$ Harus gagal (400 Bad Request).
  3. Submit dengan saldo balance $\rightarrow$ Berhasil, cek saldo di `gl_entries`.
  4. Cancel Journal Entry $\rightarrow$ Berhasil, pastikan saldo kembali ke posisi 0 di laporan buku besar.

---

## 4. Referensi Skema Database Detail (DDL)

```sql
-- 1. Naming Series Counter
CREATE TABLE series_counters (
    prefix VARCHAR(50) PRIMARY KEY,
    current_value BIGINT NOT NULL DEFAULT 0
);

-- 2. Chart of Accounts (COA)
CREATE TABLE accounts (
    name VARCHAR(140) PRIMARY KEY,
    account_number VARCHAR(50),
    account_name VARCHAR(140) NOT NULL,
    account_type VARCHAR(50) NOT NULL, -- Asset, Liability, Equity, Income, Expense
    root_type VARCHAR(50) NOT NULL,    -- Asset, Liability, Equity, Income, Expense
    is_group BOOLEAN NOT NULL DEFAULT FALSE,
    parent_account VARCHAR(140) REFERENCES accounts(name),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Header Transaksi Jurnal
CREATE TABLE journal_entries (
    name VARCHAR(140) PRIMARY KEY,
    posting_date DATE NOT NULL,
    docstatus SMALLINT NOT NULL DEFAULT 0, -- 0=Draft, 1=Submitted, 2=Cancelled
    total_debit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    total_credit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    user_remark TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Baris Akun Jurnal (Child Table)
CREATE TABLE journal_entry_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent VARCHAR(140) NOT NULL REFERENCES journal_entries(name) ON DELETE CASCADE,
    idx INT NOT NULL,
    account VARCHAR(140) NOT NULL REFERENCES accounts(name),
    debit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    credit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    user_remark TEXT
);

-- 5. General Ledger Entry (Immutable Ledger)
CREATE TABLE gl_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    posting_date DATE NOT NULL,
    account VARCHAR(140) NOT NULL REFERENCES accounts(name),
    debit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    credit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    voucher_type VARCHAR(50) NOT NULL, -- Misal: 'Journal Entry'
    voucher_no VARCHAR(140) NOT NULL,  -- Misal: 'JV-2026-00001'
    is_cancelled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gl_entries_account_date ON gl_entries(account, posting_date);
CREATE INDEX idx_gl_entries_voucher ON gl_entries(voucher_type, voucher_no);
```

---

## 5. Checklist Mulai Bekerja
- [x] 1. Setup Go module & Docker Compose.
- [x] 2. Jalankan PostgreSQL lokal.
- [x] 3. Eksekusi skema migrasi dan data seed awal.
- [x] 4. Buat server Go sederhana untuk memastikan konektivitas database berjalan baik.

